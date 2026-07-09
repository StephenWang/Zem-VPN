package main

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"zem/internal/config"
	"zem/internal/connection"
	"zem/internal/engine"
	"zem/internal/platform"
	"zem/internal/profile"
	"zem/internal/service"
	"zem/internal/settings"
	"zem/internal/subscription"
	"zem/internal/sys"
)

//go:embed all:frontend/dist
var assets embed.FS

type App struct {
	ctx            context.Context
	connMgr        *connection.Manager
	subManager     *subscription.Manager
	profileManager *profile.Manager
	settings       *settings.Manager
	speedCache     *settings.SpeedTestCache
	dataDir        string
}

func NewApp() *App {
	dataDir := getDataDir()
	sm := settings.NewManager(dataDir)
	platMgr := platform.NewManager(sm, dataDir)
	subManager := subscription.NewManager(dataDir)
	profileManager := profile.NewManager(dataDir)
	speedCache := settings.NewSpeedTestCache(dataDir)
	eng := &engine.SingBoxEngine{}

	connMgr := connection.NewManager(connection.Options{
		DataDir:        dataDir,
		Settings:       sm,
		SubManager:     subManager,
		ProfileManager: profileManager,
		SpeedCache:     speedCache,
		Engine:         eng,
		Platform:       platMgr,
	})

	app := &App{
		ctx:            context.Background(),
		connMgr:        connMgr,
		subManager:     subManager,
		profileManager: profileManager,
		settings:       sm,
		speedCache:     speedCache,
		dataDir:        dataDir,
	}
	_ = connMgr.RefreshServiceClient()
	return app
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.connMgr.SetContext(ctx)
	a.connMgr.Platform().Init()

	if err := a.subManager.LoadAll(); err != nil {
		fmt.Println("load subscriptions:", err)
	}

	a.subManager.OnUpdate = a.connMgr.OnSubscriptionUpdated

	a.connMgr.StartRuleSetDownloader(ctx)
	go a.autoConnectLastSubscription()
	go a.subManager.AutoUpdate(ctx)
}

func (a *App) Shutdown(ctx context.Context) {
	_ = a.connMgr.Disconnect()
	a.connMgr.Platform().Cleanup()
	_ = a.speedCache.Close()

	if runtime.GOOS == "windows" && a.settings.GetServiceMode() {
		if sys.IsServiceRunning() {
			if err := sys.StopZemService(); err != nil {
				fmt.Println("stop service on shutdown:", err)
			}
		}
	}
	_ = a.settings.Close()
}

func (a *App) autoConnectLastSubscription() {
	lastID := a.settings.GetCurrentSubID()
	if lastID == "" {
		return
	}
	if a.subManager.Get(lastID) == nil {
		_ = a.settings.SetCurrentSubID("")
		return
	}
	if err := a.ConnectSubscription(lastID); err != nil {
		fmt.Println("auto connect failed:", err)
	}
}

func (a *App) parseSubConfig(subID string) (*config.SingBoxConfig, *subscription.Subscription, error) {
	sub := a.subManager.Get(subID)
	if sub == nil {
		return nil, nil, fmt.Errorf("subscription not found: %s", subID)
	}
	if sub.SingBoxJSON == "" {
		return nil, nil, fmt.Errorf("config not ready")
	}
	var cfg config.SingBoxConfig
	if err := json.Unmarshal([]byte(sub.SingBoxJSON), &cfg); err != nil {
		return nil, nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, sub, nil
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
	return a.connMgr.ConnectSubscription(a.ctx, subID)
}

func (a *App) UpdateSubscription(subID string) error {
	_, err := a.subManager.Update(subID)
	return err
}

func (a *App) DeleteSubscription(subID string) error {
	if a.GetCurrentSubscriptionID() == subID {
		_ = a.connMgr.Disconnect()
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

// GetServers 返回订阅中的服务器列表
func (a *App) GetServers(subID string) ([]map[string]interface{}, error) {
	cfg, _, err := a.parseSubConfig(subID)
	if err != nil {
		return nil, err
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

// SelectServer 选择指定服务器作为当前代理
func (a *App) SelectServer(subID, serverTag string) error {
	return a.connMgr.SelectServer(a.ctx, subID, serverTag)
}

// GetGroups 返回订阅中的代理分组（selector/urltest）
func (a *App) GetGroups(subID string) ([]map[string]interface{}, error) {
	cfg, _, err := a.parseSubConfig(subID)
	if err != nil {
		return nil, err
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

// SelectGroup 切换当前使用的代理分组
func (a *App) SelectGroup(subID, groupTag string) error {
	return a.connMgr.SelectGroup(a.ctx, subID, groupTag)
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

// SpeedTest 对订阅内所有代理节点进行并发延迟测试
func (a *App) SpeedTest(subID string) (map[string]int64, error) {
	ctx := a.connMgr.StartSpeedTest(a.ctx)
	defer a.connMgr.StopSpeedTest()
	return a.connMgr.SpeedTest(ctx, subID)
}

// SpeedTestNodes 对指定节点标签列表进行并发延迟测试
func (a *App) SpeedTestNodes(subID string, nodeTags []string) (map[string]int64, error) {
	ctx := a.connMgr.StartSpeedTest(a.ctx)
	defer a.connMgr.StopSpeedTest()
	return a.connMgr.SpeedTestNodes(ctx, subID, nodeTags)
}

// AbortSpeedTest 取消当前正在进行的测速
func (a *App) AbortSpeedTest() {
	a.connMgr.CancelSpeedTest()
}

// GetSpeedTestCache 返回指定订阅的测速缓存结果
func (a *App) GetSpeedTestCache(subID string) (map[string]int64, error) {
	return a.speedCache.Get(subID), nil
}

// ClearSpeedTestCache 清除指定订阅的测速缓存
func (a *App) ClearSpeedTestCache(subID string) error {
	return a.speedCache.Clear(subID)
}

// GetTrafficStats 返回当前 tun-in 的总上/下行流量（字节）
func (a *App) GetTrafficStats() (map[string]int64, error) {
	return a.connMgr.TrafficStats(a.ctx)
}

// ========== Profile / 多订阅合并 ==========

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
	return a.connMgr.ConnectProfile(a.ctx, profileID)
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

// ========== 代理端口设置 ==========

func (a *App) GetProxyPort() int {
	return a.settings.GetProxyPort()
}

func (a *App) SetProxyPort(port int) error {
	return a.connMgr.SetProxyPort(a.ctx, port)
}

// ========== 代理模式 ==========

func (a *App) GetProxyMode() string {
	return a.settings.GetProxyMode()
}

func (a *App) SetProxyMode(mode string) error {
	return a.connMgr.SetProxyMode(a.ctx, mode)
}

func (a *App) GetAutoReconnectOnUpdate() bool {
	return a.settings.GetAutoReconnectOnUpdate()
}

func (a *App) SetAutoReconnectOnUpdate(enabled bool) error {
	return a.settings.SetAutoReconnectOnUpdate(enabled)
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
	return a.connMgr.RefreshServiceClient()
}

func (a *App) SetServiceMode(enabled bool) error {
	return a.connMgr.SetServiceMode(enabled)
}

// ========== 基础控制接口 ==========

func (a *App) Disconnect() error {
	return a.connMgr.Disconnect()
}

func (a *App) GetStatus() string {
	return a.connMgr.Status()
}

func (a *App) GetCurrentSubscriptionID() string {
	return a.connMgr.CurrentSubID()
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
	dir, err := os.UserConfigDir()
	if err != nil {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "Zem")
}

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
		"ru": "俄罗斯", "russia": "俄罗斯", "俄罗斯": "俄罗斯",
		"ca": "加拿大", "canada": "加拿大", "加拿大": "加拿大",
		"au": "澳大利亚", "australia": "澳大利亚", "澳大利亚": "澳大利亚",
		"in": "印度", "india": "印度", "印度": "印度",
		"nl": "荷兰", "netherlands": "荷兰", "荷兰": "荷兰",
	}
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

func main() {
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

// runServiceMode 在 Windows 下以服务形式运行 sing-box 核心
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
