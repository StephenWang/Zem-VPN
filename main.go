package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"zem/internal/config"
	"zem/internal/engine"
	"zem/internal/profile"
	"zem/internal/service"
	"zem/internal/settings"
	"zem/internal/subscription"
	"zem/internal/sys"
)

const (
	geositeCNURL  = "https://github.com/aleskxyz/sing-box-rules/releases/download/202607060934/geosite-cn.srs"
	geoipCNURL    = "https://github.com/aleskxyz/sing-box-rules/releases/download/202607060934/geoip-cn.srs"
	maxRuleSetAge = 7 * 24 * time.Hour
)

// ensureChinaRuleSets 检查并预下载中国大陆 rule-set 文件到 dataDir/rule-set
func (a *App) ensureChinaRuleSets() error {
	rsDir := filepath.Join(a.dataDir, "rule-set")
	if err := os.MkdirAll(rsDir, 0755); err != nil {
		return err
	}
	for _, u := range []string{geositeCNURL, geoipCNURL} {
		name := filepath.Base(u)
		path := filepath.Join(rsDir, name)
		fi, err := os.Stat(path)
		if err == nil && time.Since(fi.ModTime()) < maxRuleSetAge {
			continue
		}
		if err := downloadFile(u, path); err != nil {
			return fmt.Errorf("download %s: %w", name, err)
		}
	}
	return nil
}

func downloadFile(url, path string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(url)
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
	_, err = io.Copy(f, resp.Body)
	_ = f.Close()
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

func chinaRemoteRuleSets() []config.RuleSet {
	return []config.RuleSet{
		{Type: "remote", Tag: "geosite-cn", Format: "binary", URL: geositeCNURL},
		{Type: "remote", Tag: "geoip-cn", Format: "binary", URL: geoipCNURL},
	}
}

func mergeRuleSets(existing []config.RuleSet, extra []config.RuleSet) []config.RuleSet {
	merged := make([]config.RuleSet, 0, len(existing)+len(extra))
	seen := make(map[string]bool, len(existing)+len(extra))
	for _, rs := range existing {
		if rs.Tag == "" || seen[rs.Tag] {
			continue
		}
		merged = append(merged, rs)
		seen[rs.Tag] = true
	}
	for _, rs := range extra {
		if rs.Tag == "" || seen[rs.Tag] {
			continue
		}
		merged = append(merged, rs)
		seen[rs.Tag] = true
	}
	return merged
}

//go:embed all:frontend/dist
var assets embed.FS

type App struct {
	ctx            context.Context
	engine         *engine.SingBoxEngine
	serviceClient  *service.Client
	subManager     *subscription.Manager
	profileManager *profile.Manager
	settings       *settings.Manager
	speedCache     *settings.SpeedTestCache
	dataDir        string
	mu             sync.Mutex
}

func NewApp() *App {
	dataDir := getDataDir()
	sm := settings.NewManager(dataDir)
	app := &App{
		engine:         &engine.SingBoxEngine{},
		subManager:     subscription.NewManager(dataDir),
		profileManager: profile.NewManager(dataDir),
		settings:       sm,
		speedCache:     settings.NewSpeedTestCache(dataDir),
		dataDir:        dataDir,
	}
	app.initServiceClient()
	return app
}

// initServiceClient 如果启用了服务模式且服务正在运行，则创建服务客户
func (a *App) initServiceClient() {
	if runtime.GOOS != "windows" {
		return
	}
	if !a.settings.GetServiceMode() {
		return
	}
	if !sys.IsServiceInstalled() || !sys.IsServiceRunning() {
		a.serviceClient = nil
		return
	}
	client := service.NewClient(a.settings.GetServicePort(), a.settings.GetServiceToken())
	if _, err := client.Status(); err != nil {
		a.serviceClient = nil
		return
	}
	a.serviceClient = client
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx

	// 平台特定初始?	initPlatform()

	// 加载已有订阅
	if err := a.subManager.LoadAll(); err != nil {
		fmt.Println("load subscriptions:", err)
	}

	// 注册订阅更新回调：如果当前正在使用该订阅，则自动重连
	a.subManager.OnUpdate = a.onSubscriptionUpdated

	// 尝试恢复上次连接的订
	go a.autoConnectLastSubscription()

	// 启动自动更新
	go a.subManager.AutoUpdate(ctx)
}

// onSubscriptionUpdated 在订阅自动更新成功后触发
func (a *App) onSubscriptionUpdated(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.GetCurrentSubscriptionID() != id {
		return
	}

	sub := a.subManager.Get(id)
	if sub == nil || sub.SingBoxJSON == "" {
		return
	}

	fmt.Printf("subscription %s updated, reconnecting...\n", id)
	configJSON, err := a.prepareConfig(sub.SingBoxJSON, id)
	if err != nil {
		fmt.Println("prepare config after update:", err)
		return
	}

	if a.serviceClient != nil {
		if err := a.serviceClient.Connect(configJSON, id); err != nil {
			fmt.Println("reconnect via service after update:", err)
		}
		return
	}

	a.engine.Stop()
	_ = a.applyPlatformConnection(true)
	if err := a.engine.Start(configJSON); err != nil {
		fmt.Println("reconnect after update:", err)
		_ = a.applyPlatformConnection(false)
	}
}

// autoConnectLastSubscription 在启动后尝试恢复上次连接的订
func (a *App) autoConnectLastSubscription() {
	lastID := a.settings.GetCurrentSubID()
	if lastID == "" {
		return
	}
	if a.subManager.Get(lastID) == nil {
		// 上次订阅已不存在，清理记
		_ = a.settings.SetCurrentSubID("")
		return
	}
	// 本地模式需要管理员权限；服务模式由后台服务处理
	if a.serviceClient == nil && !sys.CheckAdmin() {
		fmt.Println("auto connect skipped: need admin")
		return
	}
	if err := a.ConnectSubscription(lastID); err != nil {
		fmt.Println("auto connect failed:", err)
	}
}

func (a *App) Shutdown(ctx context.Context) {
	// 清理
	_ = a.applyPlatformConnection(false)
	cleanupPlatform()
	a.engine.Stop()

	// 如果以服务方式运行 sing-box 核心，关闭程序时停止服务
	if runtime.GOOS == "windows" {
		if sys.IsServiceRunning() {
			if err := sys.StopZemService(); err != nil {
				fmt.Println("stop service on shutdown:", err)
			}
		}
	}
}

// initPlatform 平台特定初始化
func initPlatform() {
	switch runtime.GOOS {
	case "windows":
		// Windows: 检查管理员权限，释?wintun.dll
		if err := sys.EnsureAdmin(); err != nil {
			fmt.Println("Warning:", err)
		}
		if _, err := sys.ExtractWintun(); err != nil {
			fmt.Println("Wintun:", err)
		}
		// 添加防火墙规
		exePath, _ := os.Executable()
		sys.AddWindowsFirewallRule("Zem", exePath)

	case "linux":
		// Linux: 检?TUN 支持
		if err := sys.CheckTUNSupport(); err != nil {
			fmt.Println("Warning:", err)
			fmt.Println("尝试安装 TUN 模块...")
			if err := sys.InstallTUNModule(); err != nil {
				fmt.Println("安装失败:", err)
			}
		}
		if err := sys.EnsureAdmin(); err != nil {
			fmt.Println("Warning:", err)
		}

	case "darwin":
		// macOS: 检查权
		if err := sys.CheckMacOSPermissions(); err != nil {
			fmt.Println("Warning:", err)
		}
		if err := sys.EnsureAdmin(); err != nil {
			fmt.Println("Warning:", err)
		}
	}
}

// cleanupPlatform 平台特定清理
func cleanupPlatform() {
	switch runtime.GOOS {
	case "windows":
		// 清理防火墙规
		sys.RemoveWindowsFirewallRule("Zem")

	case "darwin":
		// 重置 DNS
		sys.ResetMacOSDNS()
	}

	// 通用：清理路
	sys.CleanupRoutes("tun0")
}

// applyPlatformConnection 连接/断开时的平台级设
func (a *App) applyPlatformConnection(connected bool) error {
	if connected {
		proxyAddr := fmt.Sprintf("127.0.0.1:%d", a.settings.GetProxyPort())
		mode := a.settings.GetProxyMode()
		tunMode := mode != "system" && a.settings.GetTunSettings().AutoRoute
		if err := sys.SetupPlatformConnection(proxyAddr, tunMode, mode); err != nil {
			fmt.Println("platform connection setup:", err)
			return err
		}
		if runtime.GOOS == "darwin" && mode != "system" {
			_ = sys.SetupMacOSDNS("172.19.0.2")
		}
	} else {
		if err := sys.CleanupPlatformConnection(); err != nil {
			fmt.Println("platform connection cleanup:", err)
			return err
		}
		if runtime.GOOS == "darwin" {
			_ = sys.ResetMacOSDNS()
		}
	}
	return nil
}

// ========== 订阅管理接口 ==========

func (a *App) AddSubscription(url, name string) (string, error) {
	sub, err := a.subManager.Add(url, name)
	if err != nil {
		return "", err
	}
	return sub.ID, nil
}

func (a *App) AddSubscriptionWithOptions(url, name string, opts subscription.SubscriptionOptions) (string, error) {
	sub, err := a.subManager.AddWithOptions(url, name, opts)
	if err != nil {
		return "", err
	}
	return sub.ID, nil
}

func (a *App) UpdateSubscriptionOptions(subID string, opts subscription.SubscriptionOptions) error {
	sub := a.subManager.Get(subID)
	if sub == nil {
		return fmt.Errorf("subscription not found: %s", subID)
	}
	sub.Options = opts
	if err := a.subManager.Save(sub); err != nil {
		return err
	}
	a.subManager.Replace(sub)
	return nil
}

func (a *App) ConnectSubscription(subID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 服务模式：直接通过服务连接，不需?GUI 以管理员运行
	if a.serviceClient != nil {
		sub := a.subManager.Get(subID)
		if sub == nil {
			return fmt.Errorf("subscription not found: %s", subID)
		}
		if sub.SingBoxJSON == "" {
			return fmt.Errorf("config not ready")
		}
		configJSON, err := a.prepareConfig(sub.SingBoxJSON, sub.ID)
		if err != nil {
			return fmt.Errorf("prepare config: %w", err)
		}
		if err := a.serviceClient.Connect(configJSON, subID); err != nil {
			return err
		}
		a.engine.SetCurrentSubID(subID)
		_ = a.settings.SetCurrentSubID(subID)
		return nil
	}

	if !sys.CheckAdmin() {
		return fmt.Errorf("需要管理员权限")
	}

	sub := a.subManager.Get(subID)
	if sub == nil {
		return fmt.Errorf("subscription not found: %s", subID)
	}
	if sub.SingBoxJSON == "" {
		return fmt.Errorf("config not ready")
	}

	// 注入代理端口和默认选择
	configJSON, err := a.prepareConfig(sub.SingBoxJSON, sub.ID)
	if err != nil {
		return fmt.Errorf("prepare config: %w", err)
	}

	// 如果用户没有手动选择过节点，尝试使用测速缓存中的最优节点
	if a.settings.GetSelectedNode(sub.ID) == "" {
		if best := a.speedCache.BestNode(sub.ID); best != "" {
			if err := a.selectServerInternal(sub.ID, best); err == nil {
				_ = a.settings.SetSelectedNode(sub.ID, best)
				// 重新 prepareConfig 以应用选择
				configJSON, err = a.prepareConfig(sub.SingBoxJSON, sub.ID)
				if err != nil {
					return fmt.Errorf("prepare config after auto select: %w", err)
				}
			}
		}
	}

	// 先停止已有引擎，避免 mixed/tun 入站端口冲突
	_ = a.engine.Stop()
	_ = a.applyPlatformConnection(false)

	// 平台特定连接前配置
	if err := a.applyPlatformConnection(true); err != nil {
		return fmt.Errorf("setup platform connection: %w", err)
	}

	if err := a.engine.Start(configJSON); err != nil {
		_ = a.applyPlatformConnection(false)
		return err
	}
	a.engine.SetCurrentSubID(subID)
	_ = a.settings.SetCurrentSubID(subID)
	return nil
}

func (a *App) prepareConfig(configJSON, subID string) (string, error) {
	var cfg config.SingBoxConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		return "", fmt.Errorf("parse config: %w", err)
	}

	// 强制开启 debug 日志并写入文件，便于诊断
	logDir := filepath.Join(a.dataDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", fmt.Errorf("create log dir: %w", err)
	}
	logName := time.Now().Format("2006010215") + ".log"
	logPath := filepath.Join(logDir, logName)
	cfg.Log = &config.LogOptions{
		Level:  "debug",
		Output: logPath,
	}

	// 修复旧版 DNS server 格式（sing-box 1.14 已移除 legacy format）
	cfg.DNS = fixLegacyDNS(cfg.DNS)

	// 过滤 sing-box 1.13+ 已移除的 outbound 类型（如 dns）
	cfg.Outbounds = fixOutbounds(cfg.Outbounds)

	// 清理 selector/urltest 中对不存在 outbound 的引用
	cfg.Outbounds = fixOutboundsReferences(cfg.Outbounds)

	// 注入 mixed 入站代理端口
	proxyPort := a.settings.GetProxyPort()
	hasMixed := false
	for i := range cfg.Inbounds {
		if cfg.Inbounds[i].Type == "mixed" {
			cfg.Inbounds[i].Listen = "127.0.0.1"
			cfg.Inbounds[i].ListenPort = proxyPort
			hasMixed = true
			break
		}
	}
	if !hasMixed {
		cfg.Inbounds = append(cfg.Inbounds, config.Inbound{
			Type:       "mixed",
			Tag:        "mixed-in",
			Listen:     "127.0.0.1",
			ListenPort: proxyPort,
		})
	}

	// 注入 TUN 设置
	mode := a.settings.GetProxyMode()
	tun := a.settings.GetTunSettings()
	hasTUN := false
	if mode != "system" {
		for i := range cfg.Inbounds {
			if cfg.Inbounds[i].Type == "tun" {
				cfg.Inbounds[i].Address = tun.Address
				cfg.Inbounds[i].Stack = tun.Stack
				cfg.Inbounds[i].MTU = tun.MTU
				cfg.Inbounds[i].AutoRoute = tun.AutoRoute
				cfg.Inbounds[i].StrictRoute = tun.StrictRoute
				cfg.Inbounds[i].EndpointIndependentNAT = tun.EndpointIndependentNAT
				// GSO 在 sing-box 1.12+ 已移除，强制禁用
				cfg.Inbounds[i].GSO = false
				hasTUN = true
				break
			}
		}
		if !hasTUN {
			cfg.Inbounds = append(cfg.Inbounds, config.Inbound{
				Type:                   "tun",
				Tag:                    "tun-in",
				Address:                tun.Address,
				Stack:                  tun.Stack,
				MTU:                    tun.MTU,
				AutoRoute:              tun.AutoRoute,
				StrictRoute:            tun.StrictRoute,
				EndpointIndependentNAT: tun.EndpointIndependentNAT,
			})
		}
	} else {
		// 系统代理模式：移除 TUN 入站
		filteredInbounds := make([]config.Inbound, 0, len(cfg.Inbounds))
		for _, in := range cfg.Inbounds {
			if in.Type != "tun" {
				filteredInbounds = append(filteredInbounds, in)
			}
		}
		cfg.Inbounds = filteredInbounds
	}

	// 直连模式：所有流量走 direct
	if mode == "direct" {
		cfg.Route.Final = "direct"
	} else if mode == "rule" {
		// rule 模式：加载中国大陆 rule-set，大陆流量直连，其余走代理
		if err := a.ensureChinaRuleSets(); err != nil {
			fmt.Println("ensure china rule-sets:", err)
		}
		cfg.Route.RuleSet = mergeRuleSets(cfg.Route.RuleSet, chinaRemoteRuleSets())
		cfg.Route.Rules = append([]config.RouteRule{
			{Action: "route", Outbound: "direct", Rule: config.Rule{RuleSet: []string{"geosite-cn"}}},
			{Action: "route", Outbound: "direct", Rule: config.Rule{RuleSet: []string{"geoip-cn"}}},
		}, cfg.Route.Rules...)
	}

	// 收集所有代理 outbound tag 并修复 selector/urltest 的 default
	proxyTypeSet := make(map[string]bool)
	for _, t := range config.ProxyTypes {
		proxyTypeSet[t] = true
	}
	var proxyTags []string
	existingTags := make(map[string]bool)
	for _, out := range cfg.Outbounds {
		existingTags[out.Tag] = true
		if proxyTypeSet[out.Type] {
			proxyTags = append(proxyTags, out.Tag)
		}
	}

	for i := range cfg.Outbounds {
		if cfg.Outbounds[i].Type != "selector" {
			// 只有 selector 支持 default 字段；urltest 等不支持
			cfg.Outbounds[i].Default = ""
			continue
		}
		// 清理无效引用
		filtered := make([]string, 0, len(cfg.Outbounds[i].Outbounds))
		for _, tag := range cfg.Outbounds[i].Outbounds {
			if existingTags[tag] {
				filtered = append(filtered, tag)
			}
		}
		if len(filtered) == 0 {
			filtered = []string{"direct"}
		}
		cfg.Outbounds[i].Outbounds = filtered
		// default 必须指向存在?tag
		if cfg.Outbounds[i].Default != "" && !existingTags[cfg.Outbounds[i].Default] {
			cfg.Outbounds[i].Default = filtered[0]
		}
		if cfg.Outbounds[i].Default == "" {
			cfg.Outbounds[i].Default = filtered[0]
		}
	}

	// 确保存在一个可用的 selector 作为路由默认出口
	if len(proxyTags) > 0 {
		// 查找已有的 selector，优先使用 tag 为 selected / select / auto 的
		existingSelectorIdx := -1
		routeSelectorTag := "selected"
		for i, out := range cfg.Outbounds {
			if out.Type == "selector" {
				existingSelectorIdx = i
				routeSelectorTag = out.Tag
				if out.Tag == "selected" {
					break
				}
			}
		}

		// 计算 selector 的 default：优先用用户当前选中的节点，其次保留原 selector 的 default，最后使用第一个代理
		defaultSelected := proxyTags[0]
		currentSelected := ""
		if subID != "" && !strings.HasPrefix(subID, "profile:") {
			currentSelected = a.settings.GetSelectedNode(subID)
		}
		if currentSelected == "" && existingSelectorIdx >= 0 {
			currentSelected = cfg.Outbounds[existingSelectorIdx].Default
		}
		if currentSelected != "" {
			for _, tag := range proxyTags {
				if tag == currentSelected {
					defaultSelected = currentSelected
					break
				}
			}
		}

		if existingSelectorIdx >= 0 {
			// 复用已有 selector，确保它包含所有代理并指向有效的 default
			cfg.Outbounds[existingSelectorIdx].Outbounds = proxyTags
			cfg.Outbounds[existingSelectorIdx].Default = defaultSelected
			routeSelectorTag = cfg.Outbounds[existingSelectorIdx].Tag
			existingTags[routeSelectorTag] = true
		} else {
			cfg.Outbounds = append(cfg.Outbounds, config.Outbound{
				Type:      "selector",
				Tag:       "selected",
				Outbounds: proxyTags,
				Default:   defaultSelected,
			})
			existingTags["selected"] = true
		}

		if mode != "direct" {
			cfg.Route.Final = routeSelectorTag
		}

		// 强制 route.final 指向一个存在的 selector
		if cfg.Route.Final == "" || !existingTags[cfg.Route.Final] {
			for _, out := range cfg.Outbounds {
				if out.Type == "selector" {
					cfg.Route.Final = out.Tag
					break
				}
			}
		}
	}

	// 修复 DNS server 的 detour 指向不存在的 outbound
	if cfg.DNS != nil {
		defaultDetour := "direct"
		if len(proxyTags) > 0 {
			defaultDetour = proxyTags[0]
		}
		for i := range cfg.DNS.Servers {
			if cfg.DNS.Servers[i].Detour != "" && !existingTags[cfg.DNS.Servers[i].Detour] {
				cfg.DNS.Servers[i].Detour = defaultDetour
			}
		}
	}

	result, err := json.Marshal(cfg)
	return string(result), err
}

// fixLegacyDNS 将旧版 DNS server 格式替换为 sing-box 1.14 兼容的新格式。
// 包括：过滤损坏 server、为域名型 server 补全 domain_resolver、确保有 local server。
func fixLegacyDNS(dns *config.DNSOptions) *config.DNSOptions {
	if dns == nil {
		return nil
	}
	fixed := make([]config.DNSServer, 0, len(dns.Servers))
	hasLocal := false
	for _, s := range dns.Servers {
		if s.Type == "" {
			continue // 旧格式或损坏的 server，跳过
		}
		if s.Type == "local" {
			hasLocal = true
		}
		// 如果 server 是域名且没有 domain_resolver，补全为 local
		if s.Server != "" && net.ParseIP(s.Server) == nil && s.DomainResolver == nil {
			s.DomainResolver = &config.DomainResolverOptions{Server: "local"}
		}
		fixed = append(fixed, s)
	}
	if len(fixed) == 0 {
		// 没有任何有效 server 时，使用默认配置
		fixed = []config.DNSServer{
			{Type: "local", Tag: "local"},
			{Type: "https", Tag: "remote", Server: "1.1.1.1", ServerPort: 443, Detour: "direct"},
		}
		hasLocal = true
	}
	// 确保?local DNS server 作为 domain resolver
	if !hasLocal {
		fixed = append([]config.DNSServer{{Type: "local", Tag: "local"}}, fixed...)
	}
	dns.Servers = fixed
	return dns
}

// fixOutbounds 过滤 sing-box 1.13+ 已移除的 outbound 类型（如 dns），
// 并清理 selector/urltest 中对被移除节点的引用。
func fixOutbounds(outbounds []config.Outbound) []config.Outbound {
	filtered := make([]config.Outbound, 0, len(outbounds))
	removed := make(map[string]bool)
	for _, out := range outbounds {
		if out.Type == "dns" {
			removed[out.Tag] = true
			continue
		}
		filtered = append(filtered, out)
	}
	if len(removed) == 0 {
		return outbounds
	}
	for i := range filtered {
		if filtered[i].Type == "selector" || filtered[i].Type == "urltest" {
			filtered[i].Outbounds = filterStrings(filtered[i].Outbounds, removed)
		}
	}
	return filtered
}

func filterStrings(items []string, removed map[string]bool) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if !removed[item] {
			result = append(result, item)
		}
	}
	return result
}

// fixOutboundsReferences 清理 selector/urltest 中对不存在 outbound 的引用，
// 避免启动时报 dependency not found。
func fixOutboundsReferences(outbounds []config.Outbound) []config.Outbound {
	existing := make(map[string]bool)
	for _, out := range outbounds {
		existing[out.Tag] = true
	}
	for i := range outbounds {
		if outbounds[i].Type != "selector" && outbounds[i].Type != "urltest" {
			continue
		}
		filtered := make([]string, 0, len(outbounds[i].Outbounds))
		for _, tag := range outbounds[i].Outbounds {
			if existing[tag] {
				filtered = append(filtered, tag)
			}
		}
		if len(filtered) == 0 {
			filtered = []string{"direct"}
		}
		outbounds[i].Outbounds = filtered
	}
	return outbounds
}

func (a *App) UpdateSubscription(subID string) error {
	_, err := a.subManager.Update(subID)
	return err
}

func (a *App) DeleteSubscription(subID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.engine.GetCurrentSubID() == subID {
		_ = a.applyPlatformConnection(false)
		a.engine.Stop()
		_ = a.settings.SetCurrentSubID("")
	}
	return a.subManager.Delete(subID)
}

func (a *App) ListSubscriptions() ([]map[string]interface{}, error) {
	subs := a.subManager.List()
	result := make([]map[string]interface{}, len(subs))
	for i, sub := range subs {
		result[i] = map[string]interface{}{
			"id":         sub.ID,
			"name":       sub.Name,
			"url":        sub.URL,
			"lastUpdate": sub.LastUpdate.Format("2006-01-02 15:04"),
			"options": map[string]interface{}{
				"user_agent": sub.Options.UserAgent,
				"cookie":     sub.Options.Cookie,
				"preprocess": sub.Options.Preprocess,
				"skip_tls":   sub.Options.SkipTLS,
			},
		}
	}
	return result, nil
}

func (a *App) GetSubscriptionConfig(subID string) string {
	sub := a.subManager.Get(subID)
	if sub == nil {
		return ""
	}
	return sub.SingBoxJSON
}

// GetServers 返回订阅中的服务器列
func (a *App) GetServers(subID string) ([]map[string]interface{}, error) {
	sub := a.subManager.Get(subID)
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

	servers := make([]map[string]interface{}, 0)
	for _, out := range cfg.Outbounds {
		if proxyTypeSet[out.Type] {
			servers = append(servers, map[string]interface{}{
				"tag":         out.Tag,
				"type":        out.Type,
				"server":      out.Server,
				"server_port": out.ServerPort,
				"country":     guessCountry(out.Tag),
			})
		}
	}
	return servers, nil
}

// selectServerInternal 更新 selector default 但不持久化用户选择、不重连引擎
func (a *App) selectServerInternal(subID, serverTag string) error {
	sub := a.subManager.Get(subID)
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
	if err := a.subManager.Save(sub); err != nil {
		return err
	}
	a.subManager.Replace(sub)
	return nil
}

// SelectServer 选择指定服务器作为当前代理
func (a *App) SelectServer(subID, serverTag string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	sub := a.subManager.Get(subID)
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

	// 更新 selected 选择器默认值，并将路由最终出口指向 selected
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
	if err := a.subManager.Save(sub); err != nil {
		return err
	}
	a.subManager.Replace(sub)

	// 持久化用户选择，供 prepareConfig 后续使用
	if err := a.settings.SetSelectedNode(subID, serverTag); err != nil {
		return fmt.Errorf("save selected node: %w", err)
	}

	// 如果当前正在使用该订阅，则重新连接
	if a.engine.GetCurrentSubID() == subID {
		_ = a.engine.Stop()
		configJSON, err := a.prepareConfig(sub.SingBoxJSON, subID)
		if err != nil {
			return err
		}
		_ = a.applyPlatformConnection(true)
		return a.engine.Start(configJSON)
	}
	return nil
}

// GetGroups 返回订阅中的代理分组（selector/urltest
func (a *App) GetGroups(subID string) ([]map[string]interface{}, error) {
	sub := a.subManager.Get(subID)
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

	groups := make([]map[string]interface{}, 0)
	for _, out := range cfg.Outbounds {
		if out.Type == "selector" || out.Type == "urltest" {
			groups = append(groups, map[string]interface{}{
				"tag":       out.Tag,
				"type":      out.Type,
				"default":   out.Default,
				"outbounds": out.Outbounds,
			})
		}
	}
	return groups, nil
}

// SelectGroup 切换当前使用的代理分组（修改 route.final
func (a *App) SelectGroup(subID, groupTag string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	sub := a.subManager.Get(subID)
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
	if err := a.subManager.Save(sub); err != nil {
		return err
	}
	a.subManager.Replace(sub)

	// 如果当前正在使用该订阅，则重新连
	if a.engine.GetCurrentSubID() == subID {
		_ = a.engine.Stop()
		configJSON, err := a.prepareConfig(sub.SingBoxJSON, subID)
		if err != nil {
			return err
		}
		_ = a.applyPlatformConnection(true)
		return a.engine.Start(configJSON)
	}
	return nil
}

func (a *App) SpeedTestNode(subID, nodeTag string) (int64, error) {
	results, err := a.SpeedTestNodes(subID, []string{nodeTag})
	if err != nil {
		return -1, err
	}
	ms, ok := results[nodeTag]
	if !ok {
		return -1, fmt.Errorf("node not found: %s", nodeTag)
	}
	return ms, nil
}

// SpeedTest 对订阅内所有代理节点进行并发延迟测试。
func (a *App) SpeedTest(subID string) (map[string]int64, error) {
	sub := a.subManager.Get(subID)
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
	return a.SpeedTestNodes(subID, tags)
}

// SpeedTestNodes 对指定节点标签列表进行并发延迟测试，未连接时直接 TCP 拨测节点地址。
func (a *App) SpeedTestNodes(subID string, nodeTags []string) (map[string]int64, error) {
	sub := a.subManager.Get(subID)
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
	var mu sync.Mutex
	var wg sync.WaitGroup

	connected := a.engine.Status() == "connected"

	semaphore := make(chan struct{}, maxWorkers)
	for _, j := range jobs {
		wg.Add(1)
		go func(j job) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			var ms int64 = -1
			if connected {
				if r, err := a.engine.SpeedTest(j.tag, testTarget); err == nil {
					ms = r
				}
			} else {
				addr := net.JoinHostPort(j.out.Server, fmt.Sprintf("%d", j.out.ServerPort))
				start := time.Now()
				conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
				if err == nil {
					conn.Close()
					ms = time.Since(start).Milliseconds()
				}
			}

			mu.Lock()
			results[j.tag] = ms
			mu.Unlock()
		}(j)
	}
	wg.Wait()

	// 持久化测速结果，供下次自动选择最优节点使用
	_ = a.speedCache.Set(subID, results)

	return results, nil
}

// GetSpeedTestCache 返回指定订阅的测速缓存结果
func (a *App) GetSpeedTestCache(subID string) (map[string]int64, error) {
	return a.speedCache.Get(subID), nil
}

// ClearSpeedTestCache 清除指定订阅的测速缓存
func (a *App) ClearSpeedTestCache(subID string) error {
	return a.speedCache.Clear(subID)
}

// GetTrafficStats 返回当前 tun-in 的总上?下行流量（字节）
func (a *App) GetTrafficStats() (map[string]int64, error) {
	up, down, err := a.engine.GetTrafficStats()
	if err != nil {
		return nil, err
	}
	return map[string]int64{"up": up, "down": down}, nil
}

// ========== Profile / 多订阅合?==========

func (a *App) CreateProfile(name string, subIDs []string, mergeMode string) (string, error) {
	p, err := a.profileManager.Create(name, subIDs, mergeMode)
	if err != nil {
		return "", err
	}
	return p.ID, nil
}

func (a *App) UpdateProfile(profileID, name string, subIDs []string, mergeMode string) error {
	_, err := a.profileManager.Update(profileID, name, subIDs, mergeMode)
	return err
}

func (a *App) DeleteProfile(profileID string) error {
	return a.profileManager.Delete(profileID)
}

func (a *App) ListProfiles() ([]map[string]interface{}, error) {
	profiles := a.profileManager.List()
	result := make([]map[string]interface{}, len(profiles))
	for i, p := range profiles {
		result[i] = map[string]interface{}{
			"id":               p.ID,
			"name":             p.Name,
			"subscription_ids": p.SubscriptionIDs,
			"merge_mode":       p.MergeMode,
		}
	}
	return result, nil
}

func (a *App) ConnectProfile(profileID string) error {
	p := a.profileManager.Get(profileID)
	if p == nil {
		return fmt.Errorf("profile not found: %s", profileID)
	}

	var jsons []string
	for _, subID := range p.SubscriptionIDs {
		sub := a.subManager.Get(subID)
		if sub == nil || sub.SingBoxJSON == "" {
			return fmt.Errorf("subscription %s not ready", subID)
		}
		jsons = append(jsons, sub.SingBoxJSON)
	}

	mergedJSON, err := config.MergeSubscriptions(jsons, config.MergeMode(p.MergeMode))
	if err != nil {
		return fmt.Errorf("merge profile: %w", err)
	}

	configJSON, err := a.prepareConfig(mergedJSON, "profile:"+profileID)
	if err != nil {
		return fmt.Errorf("prepare profile config: %w", err)
	}

	// 服务模式
	if a.serviceClient != nil {
		if err := a.serviceClient.Connect(configJSON, "profile:"+profileID); err != nil {
			return err
		}
		a.engine.SetCurrentSubID("profile:" + profileID)
		_ = a.settings.SetCurrentSubID("profile:" + profileID)
		return nil
	}

	if !sys.CheckAdmin() {
		return fmt.Errorf("需要管理员权限")
	}

	if err := a.applyPlatformConnection(true); err != nil {
		return fmt.Errorf("setup platform connection: %w", err)
	}

	// 先停止已有引擎，避免 mixed/tun 入站端口冲突
	_ = a.engine.Stop()
	_ = a.applyPlatformConnection(false)

	if err := a.applyPlatformConnection(true); err != nil {
		return fmt.Errorf("setup platform connection: %w", err)
	}
	if err := a.engine.Start(configJSON); err != nil {
		_ = a.applyPlatformConnection(false)
		return err
	}
	a.engine.SetCurrentSubID("profile:" + profileID)
	_ = a.settings.SetCurrentSubID("profile:" + profileID)
	return nil
}

func (a *App) GetProfileConfig(profileID string) (string, error) {
	p := a.profileManager.Get(profileID)
	if p == nil {
		return "", fmt.Errorf("profile not found: %s", profileID)
	}
	var jsons []string
	for _, subID := range p.SubscriptionIDs {
		sub := a.subManager.Get(subID)
		if sub == nil || sub.SingBoxJSON == "" {
			return "", fmt.Errorf("subscription %s not ready", subID)
		}
		jsons = append(jsons, sub.SingBoxJSON)
	}
	return config.MergeSubscriptions(jsons, config.MergeMode(p.MergeMode))
}

// guessCountry 从节点名称中猜测国家/地区
func guessCountry(tag string) string {
	lower := strings.ToLower(tag)
	countryMap := map[string]string{
		"cn": "中国", "china": "中国", "中国": "中国",
		"hk": "香港", "hongkong": "香港", "hong kong": "香港", "香港": "香港",
		"tw": "台湾", "taiwan": "台湾", "台湾": "台湾",
		"jp": "日本", "japan": "日本", "日本": "日本",
		"us": "美国", "usa": "美国", "america": "美国", "美国": "美国",
		"sg": "新加坡", "singapore": "新加坡", "新加坡": "新加坡",
		"kr": "韩国", "korea": "韩国", "韩国": "韩国",
		"uk": "英国", "britain": "英国", "英国": "英国",
		"de": "德国", "germany": "德国", "德国": "德国",
		"fr": "法国", "france": "法国", "法国": "法国",
		"au": "澳大利亚", "australia": "澳大利亚", "澳大利亚": "澳大利亚",
		"ca": "加拿大", "canada": "加拿大", "加拿大": "加拿大",
		"ru": "俄罗斯", "russia": "俄罗斯", "俄罗斯": "俄罗斯",
		"in": "印度", "india": "印度", "印度": "印度",
		"br": "巴西", "brazil": "巴西", "巴西": "巴西",
	}
	// ?key 长度降序，优先匹配更长的
	var keys []string
	for k := range countryMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return len(keys[i]) > len(keys[j]) })
	for _, k := range keys {
		if strings.Contains(lower, k) {
			return countryMap[k]
		}
	}
	return "未知"
}

// ========== 代理端口设置 ==========

func (a *App) GetProxyPort() int {
	return a.settings.GetProxyPort()
}

func (a *App) SetProxyPort(port int) error {
	err := a.settings.SetProxyPort(port)
	if err != nil {
		return err
	}

	// 如果当前已连接，更新 Windows 系统代理地址
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.engine.Status() == "connected" && runtime.GOOS == "windows" && a.settings.GetProxyMode() == "system" {
		proxyAddr := fmt.Sprintf("127.0.0.1:%d", port)
		_ = sys.EnableWindowsProxy(proxyAddr)
	}
	return nil
}

// ========== 代理模式 ==========

func (a *App) GetProxyMode() string {
	return a.settings.GetProxyMode()
}

func (a *App) SetProxyMode(mode string) error {
	if err := a.settings.SetProxyMode(mode); err != nil {
		return err
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	// 如果当前已连接，用新模式重新连接
	if a.engine.Status() == "connected" {
		subID := a.engine.GetCurrentSubID()
		if subID == "" {
			return nil
		}
		sub := a.subManager.Get(subID)
		if sub == nil || sub.SingBoxJSON == "" {
			return nil
		}
		_ = a.engine.Stop()
		_ = a.applyPlatformConnection(false)

		configJSON, err := a.prepareConfig(sub.SingBoxJSON, subID)
		if err != nil {
			return fmt.Errorf("prepare config: %w", err)
		}
		_ = a.applyPlatformConnection(true)
		if err := a.engine.Start(configJSON); err != nil {
			_ = a.applyPlatformConnection(false)
			return fmt.Errorf("restart engine: %w", err)
		}
	}
	return nil
}

// ========== TUN 设置 ==========

func (a *App) GetTunSettings() settings.TunSettings {
	return a.settings.GetTunSettings()
}

func (a *App) SetTunSettings(tun settings.TunSettings) error {
	return a.settings.SetTunSettings(tun)
}

// ========== Windows Service 模式 ==========

func (a *App) IsServiceModeAvailable() bool {
	return runtime.GOOS == "windows" && sys.IsServiceAvailable()
}

func (a *App) IsServiceInstalled() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	return sys.IsServiceInstalled()
}

func (a *App) IsServiceRunning() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	return sys.IsServiceRunning()
}

func (a *App) InstallService() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("service mode only available on Windows")
	}
	return sys.InstallZemService()
}

func (a *App) UninstallService() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("service mode only available on Windows")
	}
	return sys.UninstallZemService()
}

func (a *App) StartService() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("service mode only available on Windows")
	}
	return sys.StartZemService()
}

func (a *App) StopService() error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("service mode only available on Windows")
	}
	return sys.StopZemService()
}

func (a *App) GetServiceMode() bool {
	return a.settings.GetServiceMode()
}

func (a *App) RefreshServiceClient() bool {
	a.initServiceClient()
	return a.serviceClient != nil
}

func (a *App) SetServiceMode(enabled bool) error {
	if err := a.settings.SetServiceMode(enabled); err != nil {
		return err
	}
	if enabled {
		a.initServiceClient()
	} else {
		a.serviceClient = nil
	}
	return nil
}

// ========== 基础控制接口 ==========

func (a *App) Disconnect() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.serviceClient != nil {
		err := a.serviceClient.Disconnect()
		_ = a.settings.SetCurrentSubID("")
		return err
	}

	_ = a.applyPlatformConnection(false)
	_ = a.settings.SetCurrentSubID("")
	return a.engine.Stop()
}

func (a *App) GetStatus() string {
	if a.serviceClient != nil {
		status, err := a.serviceClient.Status()
		if err != nil {
			return "disconnected"
		}
		return status
	}
	return a.engine.Status()
}

func (a *App) GetCurrentSubscriptionID() string {
	if a.serviceClient != nil {
		id, err := a.serviceClient.GetCurrentSubID()
		if err != nil {
			return ""
		}
		return id
	}
	return a.engine.GetCurrentSubID()
}

func (a *App) GetSelectedNode(subID string) string {
	return a.settings.GetSelectedNode(subID)
}

func (a *App) IsAdmin() bool {
	return sys.CheckAdmin()
}

// ========== 平台信息接口 ==========

func (a *App) GetPlatformInfo() map[string]string {
	info := map[string]string{
		"os":   runtime.GOOS,
		"arch": runtime.GOARCH,
	}

	switch runtime.GOOS {
	case "windows":
		info["admin"] = fmt.Sprintf("%v", sys.CheckAdmin())
	case "linux":
		info["distro"] = sys.GetLinuxDistro()
		info["nftables"] = fmt.Sprintf("%v", sys.HasNftables())
		info["iptables"] = fmt.Sprintf("%v", sys.HasIptables())
		info["tun"] = fmt.Sprintf("%v", sys.CheckTUNSupport() == nil)
	case "darwin":
		info["version"] = sys.GetMacOSVersion()
	}

	return info
}

// ========== 工具 ==========

func getDataDir() string {
	if os.Getenv("APPDATA") != "" {
		return filepath.Join(os.Getenv("APPDATA"), "Zem")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "Zem")
}

func main() {
	// 处理服务模式命令行参
	if len(os.Args) > 1 && os.Args[1] == "--service" {
		if runtime.GOOS != "windows" {
			fmt.Println("service mode only available on Windows")
			return
		}
		runServiceMode()
		return
	}

	app := NewApp()

	err := wails.Run(&options.App{
		Title:  "Zem",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:        app.Startup,
		OnShutdown:       app.Shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// runServiceMode ?Windows 下以服务形式运行 sing-box 核心
func runServiceMode() {
	dataDir := getDataDir()
	sm := settings.NewManager(dataDir)
	port := sm.GetServicePort()

	svc := service.New(sm.GetServiceToken())
	runner := &sys.ServiceRunner{
		OnStart: func() error {
			return svc.Start(port)
		},
		OnStop: func() {
			svc.Stop()
		},
	}

	if err := sys.RunAsWindowsService(runner); err != nil {
		fmt.Println("run service:", err)
	}
}
