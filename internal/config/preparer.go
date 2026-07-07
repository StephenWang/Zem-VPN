package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"zem/internal/settings"
)

const (
	geositeCNURL  = "https://github.com/aleskxyz/sing-box-rules/releases/download/202607060934/geosite-cn.srs"
	geoipCNURL    = "https://github.com/aleskxyz/sing-box-rules/releases/download/202607060934/geoip-cn.srs"
	maxRuleSetAge = 7 * 24 * time.Hour
)

// PrepareOptions 是 Prepare 的输入参数，全部由调用方提供，便于测试。
type PrepareOptions struct {
	DataDir      string
	ProxyPort    int
	ProxyMode    string
	TunSettings  settings.TunSettings
	SelectedNode string
	SubID        string
}

// Prepare 对 sing-box 配置进行一系列可测试的纯函数变换。
func Prepare(cfg *SingBoxConfig, opts PrepareOptions) error {
	if err := injectLog(cfg, opts.DataDir); err != nil {
		return err
	}
	cfg.DNS = NormalizeDNS(cfg.DNS)
	cfg.Outbounds = FixOutbounds(cfg.Outbounds)
	cfg.Outbounds = FixOutboundsReferences(cfg.Outbounds)
	injectInbounds(cfg, opts)
	injectTUN(cfg, opts)
	injectRoute(cfg, opts)
	ensureSelector(cfg, opts)
	fixDNSDetours(cfg)
	return nil
}

func injectLog(cfg *SingBoxConfig, dataDir string) error {
	if dataDir == "" {
		return nil
	}
	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}
	logName := time.Now().Format("2006010215") + ".log"
	cfg.Log = &LogOptions{
		Level:  "debug",
		Output: filepath.Join(logDir, logName),
	}
	return nil
}

func injectInbounds(cfg *SingBoxConfig, opts PrepareOptions) {
	proxyPort := opts.ProxyPort
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
		cfg.Inbounds = append(cfg.Inbounds, Inbound{
			Type:       "mixed",
			Tag:        "mixed-in",
			Listen:     "127.0.0.1",
			ListenPort: proxyPort,
		})
	}
}

func injectTUN(cfg *SingBoxConfig, opts PrepareOptions) {
	if opts.ProxyMode == "system" {
		filtered := make([]Inbound, 0, len(cfg.Inbounds))
		for _, in := range cfg.Inbounds {
			if in.Type != "tun" {
				filtered = append(filtered, in)
			}
		}
		cfg.Inbounds = filtered
		return
	}

	tun := opts.TunSettings
	hasTUN := false
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
		cfg.Inbounds = append(cfg.Inbounds, Inbound{
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
}

func injectRoute(cfg *SingBoxConfig, opts PrepareOptions) {
	switch opts.ProxyMode {
	case "direct":
		cfg.Route.Final = "direct"
	case "rule":
		cfg.Route.RuleSet = MergeRuleSets(cfg.Route.RuleSet, ChinaRuleSets(opts.DataDir))
		cfg.Route.Rules = append([]RouteRule{
			{Action: "route", Outbound: "direct", Rule: Rule{RuleSet: []string{"geosite-cn"}}},
			{Action: "route", Outbound: "direct", Rule: Rule{RuleSet: []string{"geoip-cn"}}},
		}, cfg.Route.Rules...)
	}
}

func ensureSelector(cfg *SingBoxConfig, opts PrepareOptions) {
	proxyTypeSet := make(map[string]bool)
	for _, t := range ProxyTypes {
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
		if cfg.Outbounds[i].Default != "" && !existingTags[cfg.Outbounds[i].Default] {
			cfg.Outbounds[i].Default = filtered[0]
		}
		if cfg.Outbounds[i].Default == "" {
			cfg.Outbounds[i].Default = filtered[0]
		}
	}

	if len(proxyTags) == 0 {
		return
	}

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

	defaultSelected := proxyTags[0]
	currentSelected := ""
	if opts.SubID != "" && !strings.HasPrefix(opts.SubID, "profile:") {
		currentSelected = opts.SelectedNode
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
		cfg.Outbounds[existingSelectorIdx].Outbounds = proxyTags
		cfg.Outbounds[existingSelectorIdx].Default = defaultSelected
		routeSelectorTag = cfg.Outbounds[existingSelectorIdx].Tag
		existingTags[routeSelectorTag] = true
	} else {
		cfg.Outbounds = append(cfg.Outbounds, Outbound{
			Type:      "selector",
			Tag:       "selected",
			Outbounds: proxyTags,
			Default:   defaultSelected,
		})
		existingTags["selected"] = true
	}

	if opts.ProxyMode != "direct" {
		cfg.Route.Final = routeSelectorTag
	}

	if cfg.Route.Final == "" || !existingTags[cfg.Route.Final] {
		for _, out := range cfg.Outbounds {
			if out.Type == "selector" {
				cfg.Route.Final = out.Tag
				break
			}
		}
	}
}

func fixDNSDetours(cfg *SingBoxConfig) {
	if cfg.DNS == nil {
		return
	}
	existingTags := make(map[string]bool)
	defaultDetour := "direct"
	for _, out := range cfg.Outbounds {
		existingTags[out.Tag] = true
		if defaultDetour == "direct" && isProxyType(out.Type) {
			defaultDetour = out.Tag
		}
	}
	for i := range cfg.DNS.Servers {
		if cfg.DNS.Servers[i].Detour != "" && !existingTags[cfg.DNS.Servers[i].Detour] {
			cfg.DNS.Servers[i].Detour = defaultDetour
		}
	}
}

func isProxyType(t string) bool {
	for _, pt := range ProxyTypes {
		if pt == t {
			return true
		}
	}
	return false
}

// MergeRuleSets 合并两个 rule-set 列表，按 tag 去重。
func MergeRuleSets(existing []RuleSet, extra []RuleSet) []RuleSet {
	merged := make([]RuleSet, 0, len(existing)+len(extra))
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

// ChinaRuleSets 返回中国大陆 rule-set 列表。
// 如果本地缓存文件存在则使用 local 类型，否则使用 remote URL，连接时不阻塞下载。
func ChinaRuleSets(dataDir string) []RuleSet {
	if dataDir == "" {
		return chinaRemoteRuleSets()
	}
	rsDir := filepath.Join(dataDir, "rule-set")
	var out []RuleSet
	for _, u := range []string{geositeCNURL, geoipCNURL} {
		name := filepath.Base(u)
		tag := strings.TrimSuffix(name, filepath.Ext(name))
		path := filepath.Join(rsDir, name)
		if fi, err := os.Stat(path); err == nil && fi.Size() > 0 {
			out = append(out, RuleSet{Type: "local", Tag: tag, Format: "binary", Path: path})
		} else {
			out = append(out, RuleSet{Type: "remote", Tag: tag, Format: "binary", URL: u})
		}
	}
	return out
}

func chinaRemoteRuleSets() []RuleSet {
	return []RuleSet{
		{Type: "remote", Tag: "geosite-cn", Format: "binary", URL: geositeCNURL},
		{Type: "remote", Tag: "geoip-cn", Format: "binary", URL: geoipCNURL},
	}
}

// RuleSetNeedsDownload 返回 rule-set 是否需要重新下载（不存在或过期）。
func RuleSetNeedsDownload(dataDir string) bool {
	rsDir := filepath.Join(dataDir, "rule-set")
	for _, u := range []string{geositeCNURL, geoipCNURL} {
		path := filepath.Join(rsDir, filepath.Base(u))
		fi, err := os.Stat(path)
		if err != nil || time.Since(fi.ModTime()) >= maxRuleSetAge {
			return true
		}
	}
	return false
}
