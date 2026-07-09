package connection

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"zem/internal/config"
	"zem/internal/engine"
	"zem/internal/platform"
	"zem/internal/profile"
	"zem/internal/service"
	"zem/internal/settings"
	"zem/internal/subscription"
	"zem/internal/sys"
)

// Options 构造 ConnectionManager 所需的依赖。
type Options struct {
	DataDir        string
	Settings       *settings.Manager
	SubManager     *subscription.Manager
	ProfileManager *profile.Manager
	SpeedCache     *settings.SpeedTestCache
	Engine         *engine.SingBoxEngine
	Platform       *platform.Manager
}

const (
	geositeCNURL  = "https://github.com/aleskxyz/sing-box-rules/releases/download/202607060934/geosite-cn.srs"
	geoipCNURL    = "https://github.com/aleskxyz/sing-box-rules/releases/download/202607060934/geoip-cn.srs"
	maxRuleSetAge = 7 * 24 * time.Hour
	maxRuleSetSize = 64 << 20 // 64 MB
)

// Manager 负责连接生命周期、平台配置、服务客户端与测速。
type Manager struct {
	ctx             context.Context
	engine          *engine.SingBoxEngine
	serviceClient   *service.Client
	platform        *platform.Manager
	settings        *settings.Manager
	subManager      *subscription.Manager
	profileManager  *profile.Manager
	speedCache      *settings.SpeedTestCache
	dataDir         string
	serviceMu       sync.Mutex
	speedTestMu     sync.Mutex
	speedTestCancel context.CancelFunc
}

func NewManager(opts Options) *Manager {
	return &Manager{
		engine:         opts.Engine,
		platform:       opts.Platform,
		settings:       opts.Settings,
		subManager:     opts.SubManager,
		profileManager: opts.ProfileManager,
		speedCache:     opts.SpeedCache,
		dataDir:        opts.DataDir,
	}
}

// SetContext 设置 Wails 生命周期上下文，供后台任务使用。
func (m *Manager) SetContext(ctx context.Context) {
	m.ctx = ctx
}

// Platform 返回底层平台管理器。
func (m *Manager) Platform() *platform.Manager {
	return m.platform
}

func (m *Manager) prepareConfig(configJSON, subID string) (string, error) {
	var cfg config.SingBoxConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return "", fmt.Errorf("parse config: %w", err)
	}
	tun := m.settings.GetTunSettings()
	opts := config.PrepareOptions{
		DataDir:      m.dataDir,
		ProxyPort:    m.settings.GetProxyPort(),
		ProxyMode:    m.settings.GetProxyMode(),
		TunSettings:  tun,
		SelectedNode: m.settings.GetSelectedNode(subID),
		SubID:        subID,
	}
	if err := config.Prepare(&cfg, opts); err != nil {
		return "", fmt.Errorf("prepare config: %w", err)
	}
	out, err := json.Marshal(cfg)
	return string(out), err
}

// Status 返回当前连接状态。
func (m *Manager) Status() string {
	m.serviceMu.Lock()
	sc := m.serviceClient
	m.serviceMu.Unlock()
	if sc != nil {
		status, err := sc.Status()
		if err != nil {
			return "disconnected"
		}
		return status
	}
	return m.engine.Status()
}

// CurrentSubID 返回当前使用的订阅/Profile ID。
func (m *Manager) CurrentSubID() string {
	m.serviceMu.Lock()
	sc := m.serviceClient
	m.serviceMu.Unlock()
	if sc != nil {
		id, err := sc.GetCurrentSubID()
		if err != nil {
			return ""
		}
		return id
	}
	return m.engine.GetCurrentSubID()
}

func (m *Manager) serviceClientLocked() *service.Client {
	m.serviceMu.Lock()
	defer m.serviceMu.Unlock()
	return m.serviceClient
}

func (m *Manager) isServiceMode() bool {
	return m.serviceClientLocked() != nil
}

// Disconnect 断开当前连接。
func (m *Manager) Disconnect() error {
	m.serviceMu.Lock()
	sc := m.serviceClient
	m.serviceMu.Unlock()

	if sc != nil {
		err := sc.Disconnect()
		_ = m.settings.SetCurrentSubID("")
		return err
	}

	_ = m.platform.Apply(false)
	_ = m.settings.SetCurrentSubID("")
	return m.engine.Stop()
}

// ConnectSubscription 连接指定订阅。
func (m *Manager) ConnectSubscription(ctx context.Context, subID string) error {
	m.serviceMu.Lock()
	sc := m.serviceClient
	m.serviceMu.Unlock()

	sub := m.subManager.Get(subID)
	if sub == nil {
		return fmt.Errorf("subscription not found: %s", subID)
	}
	if sub.SingBoxJSON == "" {
		return fmt.Errorf("config not ready")
	}

	if sc != nil {
		configJSON, err := m.prepareConfig(sub.SingBoxJSON, subID)
		if err != nil {
			return fmt.Errorf("prepare config: %w", err)
		}
		return m.connectService(ctx, configJSON, subID)
	}

	if !sys.CheckAdmin() {
		return fmt.Errorf("需要管理员权限")
	}

	if m.settings.GetSelectedNode(subID) == "" {
		if best := m.speedCache.BestNode(subID); best != "" {
			if err := m.selectServerInternal(subID, best); err == nil {
				_ = m.settings.SetSelectedNode(subID, best)
			}
		}
	}

	configJSON, err := m.prepareConfig(sub.SingBoxJSON, subID)
	if err != nil {
		return fmt.Errorf("prepare config: %w", err)
	}
	return m.connectLocal(ctx, configJSON, subID)
}

// ConnectProfile 连接指定 Profile。
func (m *Manager) ConnectProfile(ctx context.Context, profileID string) error {
	p := m.profileManager.Get(profileID)
	if p == nil {
		return fmt.Errorf("profile not found: %s", profileID)
	}

	var jsons []string
	for _, subID := range p.SubscriptionIDs {
		sub := m.subManager.Get(subID)
		if sub == nil || sub.SingBoxJSON == "" {
			return fmt.Errorf("subscription %s not ready", subID)
		}
		jsons = append(jsons, sub.SingBoxJSON)
	}

	mergedJSON, err := config.MergeSubscriptions(jsons, config.MergeMode(p.MergeMode))
	if err != nil {
		return fmt.Errorf("merge profile: %w", err)
	}

	configJSON, err := m.prepareConfig(mergedJSON, "profile:"+profileID)
	if err != nil {
		return fmt.Errorf("prepare profile config: %w", err)
	}

	m.serviceMu.Lock()
	sc := m.serviceClient
	m.serviceMu.Unlock()
	if sc != nil {
		return m.connectService(ctx, configJSON, "profile:"+profileID)
	}
	if !sys.CheckAdmin() {
		return fmt.Errorf("需要管理员权限")
	}
	return m.connectLocal(ctx, configJSON, "profile:"+profileID)
}

func (m *Manager) connectLocal(ctx context.Context, configJSON, subID string) error {
	_ = m.engine.Stop()
	_ = m.platform.Apply(false)
	if err := m.engine.Start(configJSON); err != nil {
		_ = m.platform.Apply(false)
		return err
	}
	if err := m.platform.Apply(true); err != nil {
		_ = m.engine.Stop()
		_ = m.platform.Apply(false)
		return fmt.Errorf("setup platform connection: %w", err)
	}
	m.engine.SetCurrentSubID(subID)
	_ = m.settings.SetCurrentSubID(subID)
	m.emitStatus(ctx)
	return nil
}

func (m *Manager) connectService(ctx context.Context, configJSON, subID string) error {
	sc := m.serviceClientLocked()
	if sc == nil {
		return fmt.Errorf("service client not available")
	}
	if err := sc.ReloadConfig(configJSON, subID); err != nil {
		return err
	}
	if err := m.platform.Apply(true); err != nil {
		_ = sc.Disconnect()
		_ = m.platform.Apply(false)
		return fmt.Errorf("apply platform in service mode: %w", err)
	}
	m.engine.SetCurrentSubID(subID)
	_ = m.settings.SetCurrentSubID(subID)
	m.emitStatus(ctx)
	return nil
}

func (m *Manager) emitStatus(ctx context.Context) {
	if ctx == nil {
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
	}
	wailsRuntime.EventsEmit(ctx, "status", m.Status())
}

func (m *Manager) emitTraffic(ctx context.Context, stats map[string]int64) {
	if ctx == nil {
		return
	}
	select {
	case <-ctx.Done():
		return
	default:
	}
	wailsRuntime.EventsEmit(ctx, "traffic", stats)
}

// SelectServer 选择指定服务器作为当前代理，并在已连接时重连。
func (m *Manager) SelectServer(ctx context.Context, subID, serverTag string) error {
	sub := m.subManager.Get(subID)
	if sub == nil {
		return fmt.Errorf("subscription not found: %s", subID)
	}
	if sub.SingBoxJSON == "" {
		return fmt.Errorf("config not ready")
	}

	var cfg config.SingBoxConfig
	if err := json.Unmarshal([]byte(sub.SingBoxJSON), &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	found := false
	for _, out := range cfg.Outbounds {
		if out.Tag == serverTag {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("server not found: %s", serverTag)
	}

	for i := range cfg.Outbounds {
		if cfg.Outbounds[i].Type == "selector" && cfg.Outbounds[i].Tag == "selected" {
			cfg.Outbounds[i].Default = serverTag
		}
	}
	cfg.Route.Final = "selected"

	updated, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	sub.SingBoxJSON = string(updated)
	if err := m.subManager.Save(sub); err != nil {
		return err
	}
	m.subManager.Replace(sub)

	if err := m.settings.SetSelectedNode(subID, serverTag); err != nil {
		return fmt.Errorf("save selected node: %w", err)
	}

	if m.Status() == "connected" && m.CurrentSubID() == subID {
		configJSON, err := m.prepareConfig(sub.SingBoxJSON, subID)
		if err != nil {
			return err
		}
		return m.connectCurrent(ctx, configJSON, subID)
	}
	return nil
}

// SelectGroup 切换当前使用的代理分组，并在已连接时重连。
func (m *Manager) SelectGroup(ctx context.Context, subID, groupTag string) error {
	sub := m.subManager.Get(subID)
	if sub == nil {
		return fmt.Errorf("subscription not found: %s", subID)
	}
	if sub.SingBoxJSON == "" {
		return fmt.Errorf("config not ready")
	}

	var cfg config.SingBoxConfig
	if err := json.Unmarshal([]byte(sub.SingBoxJSON), &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}

	found := false
	for _, out := range cfg.Outbounds {
		if out.Tag == groupTag {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("group not found: %s", groupTag)
	}

	cfg.Route.Final = groupTag

	updated, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	sub.SingBoxJSON = string(updated)
	if err := m.subManager.Save(sub); err != nil {
		return err
	}
	m.subManager.Replace(sub)

	if m.Status() == "connected" && m.CurrentSubID() == subID {
		configJSON, err := m.prepareConfig(sub.SingBoxJSON, subID)
		if err != nil {
			return err
		}
		return m.connectCurrent(ctx, configJSON, subID)
	}
	return nil
}

func (m *Manager) selectServerInternal(subID, serverTag string) error {
	sub := m.subManager.Get(subID)
	if sub == nil || sub.SingBoxJSON == "" {
		return fmt.Errorf("subscription not ready")
	}
	var cfg config.SingBoxConfig
	if err := json.Unmarshal([]byte(sub.SingBoxJSON), &cfg); err != nil {
		return fmt.Errorf("parse config: %w", err)
	}
	found := false
	for _, out := range cfg.Outbounds {
		if out.Tag == serverTag {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("server not found: %s", serverTag)
	}
	for i := range cfg.Outbounds {
		if cfg.Outbounds[i].Type == "selector" && cfg.Outbounds[i].Tag == "selected" {
			cfg.Outbounds[i].Default = serverTag
		}
	}
	cfg.Route.Final = "selected"
	updated, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	sub.SingBoxJSON = string(updated)
	if err := m.subManager.Save(sub); err != nil {
		return err
	}
	m.subManager.Replace(sub)
	return nil
}

// SetProxyMode 设置代理模式，并在已连接时重连。
func (m *Manager) SetProxyMode(ctx context.Context, mode string) error {
	if err := m.settings.SetProxyMode(mode); err != nil {
		return err
	}
	if m.Status() != "connected" {
		return nil
	}
	subID, configJSON, err := m.currentConfigJSON()
	if err != nil {
		return err
	}
	if subID == "" || configJSON == "" {
		return nil
	}
	newConfig, err := m.prepareConfig(configJSON, subID)
	if err != nil {
		return fmt.Errorf("prepare config: %w", err)
	}
	return m.connectCurrent(ctx, newConfig, subID)
}

// currentConfigJSON 返回当前连接对应的 subID 和原始配置 JSON（订阅或 Profile）。
func (m *Manager) currentConfigJSON() (string, string, error) {
	subID := m.CurrentSubID()
	if subID == "" {
		return "", "", nil
	}
	if strings.HasPrefix(subID, "profile:") {
		profileID := strings.TrimPrefix(subID, "profile:")
		p := m.profileManager.Get(profileID)
		if p == nil {
			return "", "", fmt.Errorf("profile not found: %s", profileID)
		}
		var jsons []string
		for _, id := range p.SubscriptionIDs {
			sub := m.subManager.Get(id)
			if sub == nil || sub.SingBoxJSON == "" {
				return "", "", fmt.Errorf("subscription %s not ready", id)
			}
			jsons = append(jsons, sub.SingBoxJSON)
		}
		merged, err := config.MergeSubscriptions(jsons, config.MergeMode(p.MergeMode))
		if err != nil {
			return "", "", fmt.Errorf("merge profile: %w", err)
		}
		return subID, merged, nil
	}
	sub := m.subManager.Get(subID)
	if sub == nil {
		return "", "", fmt.Errorf("subscription not found: %s", subID)
	}
	if sub.SingBoxJSON == "" {
		return "", "", fmt.Errorf("config not ready")
	}
	return subID, sub.SingBoxJSON, nil
}

// connectCurrent 根据当前模式选择 service 或本地引擎重连。
func (m *Manager) connectCurrent(ctx context.Context, configJSON, subID string) error {
	if sc := m.serviceClientLocked(); sc != nil {
		return m.connectService(ctx, configJSON, subID)
	}
	return m.connectLocal(ctx, configJSON, subID)
}

// SetProxyPort 设置代理端口，并在 Windows 系统代理模式下立即生效。
func (m *Manager) SetProxyPort(ctx context.Context, port int) error {
	if err := m.settings.SetProxyPort(port); err != nil {
		return err
	}
	if m.Status() != "connected" {
		return nil
	}
	if runtime.GOOS == "windows" && m.settings.GetProxyMode() == "system" {
		proxyAddr := fmt.Sprintf("127.0.0.1:%d", port)
		_ = sys.EnableWindowsProxy(proxyAddr)
	}
	return nil
}

// RefreshServiceClient 初始化服务客户端，返回是否可用。
func (m *Manager) RefreshServiceClient() bool {
	m.serviceMu.Lock()
	defer m.serviceMu.Unlock()

	if runtime.GOOS != "windows" {
		m.serviceClient = nil
		return false
	}
	if !m.settings.GetServiceMode() {
		m.serviceClient = nil
		return false
	}
	if !sys.IsServiceInstalled() || !sys.IsServiceRunning() {
		m.serviceClient = nil
		return false
	}
	client := service.NewClient(m.settings.GetServicePort(), m.settings.GetServiceToken())
	if _, err := client.Status(); err != nil {
		m.serviceClient = nil
		return false
	}
	m.serviceClient = client
	return true
}

// SetServiceMode 切换服务模式并刷新服务客户端。启用时若服务不可用则返回错误。
func (m *Manager) SetServiceMode(enabled bool) error {
	if err := m.settings.SetServiceMode(enabled); err != nil {
		return err
	}
	if !enabled {
		m.serviceMu.Lock()
		m.serviceClient = nil
		m.serviceMu.Unlock()
		return nil
	}
	if !m.RefreshServiceClient() {
		return fmt.Errorf("service mode not available: service not installed or not running")
	}
	return nil
}

// OnSubscriptionUpdated 在订阅自动更新后，如果当前正在使用该订阅则自动重连。
func (m *Manager) OnSubscriptionUpdated(id string) {
	if m.CurrentSubID() != id {
		return
	}
	if !m.settings.GetAutoReconnectOnUpdate() {
		fmt.Printf("subscription %s updated, auto-reconnect disabled, skipping\n", id)
		return
	}
	sub := m.subManager.Get(id)
	if sub == nil || sub.SingBoxJSON == "" {
		return
	}
	fmt.Printf("subscription %s updated, reconnecting...\n", id)
	configJSON, err := m.prepareConfig(sub.SingBoxJSON, id)
	if err != nil {
		fmt.Println("prepare config after update:", err)
		return
	}

	m.serviceMu.Lock()
	sc := m.serviceClient
	m.serviceMu.Unlock()
	if sc != nil {
		if err := sc.Connect(configJSON, id); err != nil {
			fmt.Println("reconnect via service after update:", err)
		}
		return
	}

	_ = m.engine.Stop()
	_ = m.platform.Apply(true)
	if err := m.engine.Start(configJSON); err != nil {
		fmt.Println("reconnect after update:", err)
		_ = m.platform.Apply(false)
	}
}

// TrafficStats 返回当前 tun-in 的总上行/下行流量（字节）。
func (m *Manager) TrafficStats(ctx context.Context) (map[string]int64, error) {
	if sc := m.serviceClientLocked(); sc != nil {
		stats, err := sc.TrafficStats()
		if err != nil {
			return nil, err
		}
		m.emitTraffic(ctx, stats)
		return stats, nil
	}
	up, down, err := m.engine.GetTrafficStats()
	if err != nil {
		return nil, err
	}
	stats := map[string]int64{"up": up, "down": down}
	m.emitTraffic(ctx, stats)
	return stats, nil
}

// SpeedTest 对订阅内所有代理节点进行并发延迟测试。
func (m *Manager) SpeedTest(ctx context.Context, subID string) (map[string]int64, error) {
	sub := m.subManager.Get(subID)
	if sub == nil {
		return nil, fmt.Errorf("subscription not found: %s", subID)
	}
	if sub.SingBoxJSON == "" {
		return nil, fmt.Errorf("config not ready")
	}
	var cfg config.SingBoxConfig
	if err := json.Unmarshal([]byte(sub.SingBoxJSON), &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	proxyTypeSet := make(map[string]bool)
	for _, t := range config.ProxyTypes {
		proxyTypeSet[t] = true
	}
	var tags []string
	for _, out := range cfg.Outbounds {
		if proxyTypeSet[out.Type] && out.Tag != "" {
			tags = append(tags, out.Tag)
		}
	}
	return m.SpeedTestNodes(ctx, subID, tags)
}

// SpeedTestNodes 对指定节点标签列表进行并发延迟测试，使用 worker pool 并支持 context 取消。
func (m *Manager) SpeedTestNodes(ctx context.Context, subID string, nodeTags []string) (map[string]int64, error) {
	sub := m.subManager.Get(subID)
	if sub == nil {
		return nil, fmt.Errorf("subscription not found: %s", subID)
	}
	if sub.SingBoxJSON == "" {
		return nil, fmt.Errorf("config not ready")
	}
	var cfg config.SingBoxConfig
	if err := json.Unmarshal([]byte(sub.SingBoxJSON), &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	proxyTypeSet := make(map[string]bool)
	for _, t := range config.ProxyTypes {
		proxyTypeSet[t] = true
	}
	outboundMap := make(map[string]config.Outbound)
	for _, out := range cfg.Outbounds {
		outboundMap[out.Tag] = out
	}

	const (
		testTarget = "1.1.1.1"
		maxWorkers = 20
	)

	type job struct {
		tag string
		out config.Outbound
	}
	var jobs []job
	for _, tag := range nodeTags {
		out, ok := outboundMap[tag]
		if !ok || !proxyTypeSet[out.Type] {
			continue
		}
		jobs = append(jobs, job{tag: tag, out: out})
	}

	results := make(map[string]int64)
	if len(jobs) == 0 {
		return results, nil
	}

	jobCh := make(chan job, len(jobs))
	resultCh := make(chan struct {
		tag string
		ms  int64
	}, len(jobs))

	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobCh {
				if ctx.Err() != nil {
					return
				}
				var ms int64 = -1
				if m.engine.Status() == "connected" {
					if r, err := m.engine.SpeedTest(ctx, j.tag, testTarget); err == nil {
						ms = r
					}
				} else {
					addr := net.JoinHostPort(j.out.Server, fmt.Sprintf("%d", j.out.ServerPort))
					dialer := net.Dialer{Timeout: 2 * time.Second}
					start := time.Now()
					conn, err := dialer.DialContext(ctx, "tcp", addr)
					if err == nil {
						conn.Close()
						ms = time.Since(start).Milliseconds()
					}
				}
				select {
				case resultCh <- struct{ tag string; ms int64 }{tag: j.tag, ms: ms}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	for _, j := range jobs {
		jobCh <- j
	}
	close(jobCh)
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for {
		select {
		case r, ok := <-resultCh:
			if !ok {
				_ = m.speedCache.Set(subID, results)
				return results, nil
			}
			results[r.tag] = r.ms
		case <-ctx.Done():
			_ = m.speedCache.Set(subID, results)
			return results, ctx.Err()
		}
	}
}

// StartSpeedTest 返回一个可被 CancelSpeedTest 取消的上下文。
func (m *Manager) StartSpeedTest(parent context.Context) context.Context {
	m.speedTestMu.Lock()
	defer m.speedTestMu.Unlock()
	if m.speedTestCancel != nil {
		m.speedTestCancel()
	}
	ctx, cancel := context.WithCancel(parent)
	m.speedTestCancel = cancel
	return ctx
}

// StopSpeedTest 清理当前测速上下文。
func (m *Manager) StopSpeedTest() {
	m.speedTestMu.Lock()
	defer m.speedTestMu.Unlock()
	if m.speedTestCancel != nil {
		m.speedTestCancel()
		m.speedTestCancel = nil
	}
}

// CancelSpeedTest 取消当前正在进行的测速。
func (m *Manager) CancelSpeedTest() {
	m.speedTestMu.Lock()
	defer m.speedTestMu.Unlock()
	if m.speedTestCancel != nil {
		m.speedTestCancel()
	}
}

// StartRuleSetDownloader 在后台定期下载/更新中国大陆 rule-set，不阻塞主流程。
func (m *Manager) StartRuleSetDownloader(ctx context.Context) {
	go func() {
		m.downloadChinaRuleSets(ctx)
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				m.downloadChinaRuleSets(ctx)
			}
		}
	}()
}

func (m *Manager) downloadChinaRuleSets(ctx context.Context) {
	rsDir := filepath.Join(m.dataDir, "rule-set")
	if err := os.MkdirAll(rsDir, 0755); err != nil {
		fmt.Println("rule-set mkdir:", err)
		return
	}
	for _, u := range []string{geositeCNURL, geoipCNURL} {
		name := filepath.Base(u)
		path := filepath.Join(rsDir, name)
		fi, err := os.Stat(path)
		if err == nil && time.Since(fi.ModTime()) < maxRuleSetAge {
			continue
		}
		if err := downloadFile(ctx, u, path); err != nil {
			fmt.Printf("download rule-set %s warning: %v\n", name, err)
		}
	}
}

func downloadFile(ctx context.Context, url, path string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %d", resp.StatusCode)
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	limited := io.LimitReader(resp.Body, maxRuleSetSize+1)
	written, err := io.Copy(f, limited)
	_ = f.Close()
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if written > maxRuleSetSize {
		_ = os.Remove(tmp)
		return fmt.Errorf("rule-set exceeds max size %d", maxRuleSetSize)
	}
	if written == 0 {
		_ = os.Remove(tmp)
		return fmt.Errorf("empty rule-set downloaded")
	}
	if resp.ContentLength > 0 && written != resp.ContentLength {
		_ = os.Remove(tmp)
		return fmt.Errorf("size mismatch: expected %d, got %d", resp.ContentLength, written)
	}
	return os.Rename(tmp, path)
}
