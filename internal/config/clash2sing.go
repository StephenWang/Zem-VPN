package config

import (
	"encoding/base64"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultGeositeCNRuleSetURL = "https://github.com/aleskxyz/sing-box-rules/releases/download/202607060934/geosite-cn.srs"
	defaultGeoIPCNRuleSetURL   = "https://github.com/aleskxyz/sing-box-rules/releases/download/202607060934/geoip-cn.srs"
)

// ========== Clash 结构定义 ==========

type ClashConfig struct {
	Port               int                    `yaml:"port"`
	SocksPort          int                    `yaml:"socks-port"`
	RedirPort          int                    `yaml:"redir-port"`
	MixedPort          int                    `yaml:"mixed-port"`
	AllowLan           bool                   `yaml:"allow-lan"`
	Mode               string                 `yaml:"mode"`
	LogLevel           string                 `yaml:"log-level"`
	ExternalController string                 `yaml:"external-controller"`
	Proxies            []ClashProxy           `yaml:"proxies"`
	ProxyGroups        []ClashProxyGroup      `yaml:"proxy-groups"`
	ProxyProviders     map[string]interface{} `yaml:"proxy-providers,omitempty"`
	RuleProviders      map[string]interface{} `yaml:"rule-providers,omitempty"`
	Rules              []string               `yaml:"rules"`
	DNS                ClashDNS               `yaml:"dns"`
}

type ClashProxy struct {
	Name              string                 `yaml:"name"`
	Type              string                 `yaml:"type"`
	Server            string                 `yaml:"server"`
	Port              int                    `yaml:"port"`
	UUID              string                 `yaml:"uuid,omitempty"`
	AlterID           int                    `yaml:"alterId,omitempty"`
	Cipher            string                 `yaml:"cipher,omitempty"`
	Password          string                 `yaml:"password,omitempty"`
	Username          string                 `yaml:"username,omitempty"`
	UDP               bool                   `yaml:"udp,omitempty"`
	SkipCertVerify    bool                   `yaml:"skip-cert-verify,omitempty"`
	TLS               bool                   `yaml:"tls,omitempty"`
	Network           string                 `yaml:"network,omitempty"`
	Flow              string                 `yaml:"flow,omitempty"`
	ServerName        string                 `yaml:"servername,omitempty"`
	SNI               string                 `yaml:"sni,omitempty"`
	WSOpts            *ClashWSOpts           `yaml:"ws-opts,omitempty"`
	GRPCOpts          *ClashGRPCOpts         `yaml:"grpc-opts,omitempty"`
	HTTPOpts          *ClashHTTPOpts         `yaml:"http-opts,omitempty"`
	RealityOpts       *ClashRealityOpts      `yaml:"reality-opts,omitempty"`
	Obfs              string                 `yaml:"obfs,omitempty"`
	ObfsParam         string                 `yaml:"obfs-param,omitempty"`
	ObfsPassword      string                 `yaml:"obfs-password,omitempty"`
	Protocol          string                 `yaml:"protocol,omitempty"`
	ProtocolParam     string                 `yaml:"protocol-param,omitempty"`
	Plugin            string                 `yaml:"plugin,omitempty"`
	PluginOpts        map[string]interface{} `yaml:"plugin-opts,omitempty"`
	Headers           map[string]string      `yaml:"headers,omitempty"`
	ALPN              []string               `yaml:"alpn,omitempty"`
	Fingerprint       string                 `yaml:"fingerprint,omitempty"`
	ClientFingerprint string                 `yaml:"client-fingerprint,omitempty"`
	Up                string                 `yaml:"up,omitempty"`
	Down              string                 `yaml:"down,omitempty"`
	Ports             string                 `yaml:"ports,omitempty"`
	Congestion        string                 `yaml:"congestion,omitempty"`
	ReduceRtt         bool                   `yaml:"reduce-rtt,omitempty"`
	UDPRelayMode      string                 `yaml:"udp-relay-mode,omitempty"`
	Reserved          string                 `yaml:"reserved,omitempty"`
	PublicKey         string                 `yaml:"public-key,omitempty"`
	PreSharedKey      string                 `yaml:"pre-shared-key,omitempty"`
	PrivateKey        string                 `yaml:"private-key,omitempty"`
	Peers             []ClashWGPeer          `yaml:"peers,omitempty"`
	MTU               int                    `yaml:"mtu,omitempty"`
}

type ClashWSOpts struct {
	Path    string            `yaml:"path"`
	Headers map[string]string `yaml:"headers"`
}

type ClashGRPCOpts struct {
	ServiceName string `yaml:"grpc-service-name"`
}

type ClashHTTPOpts struct {
	Method  string              `yaml:"method"`
	Path    []string            `yaml:"path"`
	Headers map[string][]string `yaml:"headers"`
}

type ClashRealityOpts struct {
	PublicKey string `yaml:"public-key"`
	ShortID   string `yaml:"short-id"`
}

type ClashWGPeer struct {
	Server       string `yaml:"server"`
	Port         int    `yaml:"port"`
	PublicKey    string `yaml:"public-key"`
	PreSharedKey string `yaml:"pre-shared-key"`
	Reserved     string `yaml:"reserved"`
}

type ClashProxyGroup struct {
	Name     string   `yaml:"name"`
	Type     string   `yaml:"type"`
	Proxies  []string `yaml:"proxies"`
	URL      string   `yaml:"url,omitempty"`
	Interval int      `yaml:"interval,omitempty"`
}

type ClashDNS struct {
	Enable         bool     `yaml:"enable"`
	Listen         string   `yaml:"listen"`
	Nameserver     []string `yaml:"nameserver"`
	Fallback       []string `yaml:"fallback"`
	FallbackFilter struct {
		GeoIP     bool     `yaml:"geoip"`
		GeoIPCode string   `yaml:"geoip-code"`
		IPCIDR    []string `yaml:"ipcidr"`
	} `yaml:"fallback-filter"`
}

// ========== 构建各模块 ==========

func buildOutbounds(proxies []ClashProxy) []Outbound {
	var outbounds []Outbound

	for _, p := range proxies {
		// sing-box 1.13+ 已移除 dns outbound 等特殊类型
		if strings.ToLower(p.Type) == "dns" {
			continue
		}

		out := buildOutbound(p)
		if out.Type == "" {
			continue
		}

		outbounds = append(outbounds, out)
	}

	outbounds = append(outbounds,
		Outbound{Type: "direct", Tag: "direct"},
		Outbound{Type: "block", Tag: "block"},
	)

	return outbounds
}

func buildOutbound(p ClashProxy) Outbound {
	out := Outbound{
		Tag:  p.Name,
		Type: clashTypeToSingBox(p.Type),
	}

	switch strings.ToLower(p.Type) {
	case "vmess":
		out.Server = p.Server
		out.ServerPort = p.Port
		out.UUID = p.UUID
		out.Security = firstNonEmpty(p.Cipher, "auto")
		// sing-box 1.13+ 已移除 alterId（仅支持 VMessAEAD）
		out.AlterID = 0
		out.Transport = buildTransport(p)
		if p.TLS || out.Transport != nil && (out.Transport.Type == "grpc" || p.ServerName != "" || p.SNI != "") {
			out.TLS = buildTLS(p)
		}

	case "vless":
		out.Server = p.Server
		out.ServerPort = p.Port
		out.UUID = p.UUID
		out.Transport = buildTransport(p)
		if p.TLS || p.RealityOpts != nil || p.ServerName != "" || p.SNI != "" {
			out.TLS = buildTLS(p)
		}
		if out.TLS != nil && p.RealityOpts != nil {
			out.TLS.Reality = &RealityOptions{
				Enabled:   true,
				PublicKey: p.RealityOpts.PublicKey,
				ShortID:   p.RealityOpts.ShortID,
			}
		}

	case "trojan":
		out.Server = p.Server
		out.ServerPort = p.Port
		out.Password = p.Password
		out.Transport = buildTransport(p)
		if p.TLS || p.ServerName != "" || p.SNI != "" {
			out.TLS = buildTLS(p)
		}

	case "shadowsocks", "ss":
		out.Server = p.Server
		out.ServerPort = p.Port
		out.Method = p.Cipher
		out.Password = p.Password
		if p.Plugin == "obfs" || p.Plugin == "v2ray-plugin" {
			out.Plugin = p.Plugin
			out.PluginOpts = p.PluginOpts
		}

	case "shadowsocksr", "ssr":
		out.Type = "shadowsocks"
		out.Server = p.Server
		out.ServerPort = p.Port
		out.Method = p.Cipher
		out.Password = p.Password
		if p.PluginOpts == nil {
			out.PluginOpts = make(map[string]interface{})
		} else {
			out.PluginOpts = copyMap(p.PluginOpts)
		}
		if p.Protocol != "" {
			out.PluginOpts["protocol"] = p.Protocol
		}
		if p.ProtocolParam != "" {
			out.PluginOpts["protocol-param"] = p.ProtocolParam
		}
		if p.Obfs != "" {
			out.PluginOpts["obfs"] = p.Obfs
		}
		if p.ObfsParam != "" {
			out.PluginOpts["obfs-param"] = p.ObfsParam
		}

	case "http", "https":
		out.Server = p.Server
		out.ServerPort = p.Port
		out.Username = p.Username
		out.Password = p.Password
		if p.TLS {
			out.TLS = buildTLS(p)
		}

	case "socks5", "socks":
		out.Server = p.Server
		out.ServerPort = p.Port
		out.Username = p.Username
		out.Password = p.Password

	case "anytls", "any-tls":
		out.Server = p.Server
		out.ServerPort = p.Port
		out.Password = p.Password
		if p.TLS || p.ServerName != "" || p.SNI != "" {
			out.TLS = buildTLS(p)
		}

	case "hysteria", "hysteria2":
		out.Type = "hysteria2"
		out.Server = p.Server
		out.ServerPort = p.Port
		out.Password = p.Password
		if p.Obfs != "" {
			out.Obfs = &Hysteria2ObfsOptions{
				Type:     p.Obfs,
				Password: p.ObfsPassword,
			}
		}
		if p.TLS || p.ServerName != "" || p.SNI != "" {
			out.TLS = buildTLS(p)
		}
		if p.Up != "" {
			out.UpMbps = parseMbps(p.Up)
		}
		if p.Down != "" {
			out.DownMbps = parseMbps(p.Down)
		}

	case "tuic":
		out.Server = p.Server
		out.ServerPort = p.Port
		out.UUID = p.UUID
		out.Password = p.Password
		if p.Congestion != "" {
			out.Congestion = p.Congestion
		}
		if p.TLS || p.ServerName != "" || p.SNI != "" {
			out.TLS = buildTLS(p)
		}

	case "wireguard", "wg":
		out.Type = "wireguard"
		out.Server = p.Server
		out.ServerPort = p.Port
		out.PrivateKey = p.PrivateKey
		out.PublicKey = p.PublicKey
		out.PreSharedKey = p.PreSharedKey
		out.MTU = p.MTU
		if p.Reserved != "" {
			out.Reserved = parseIntList(p.Reserved)
		}
		if len(p.Peers) > 0 {
			peer := p.Peers[0]
			out.Server = peer.Server
			out.ServerPort = peer.Port
			out.PublicKey = peer.PublicKey
			out.PreSharedKey = peer.PreSharedKey
			if peer.Reserved != "" {
				out.Reserved = parseIntList(peer.Reserved)
			}
		}

	case "ssh":
		out.Type = "ssh"
		out.Server = p.Server
		out.ServerPort = firstPositive(p.Port, 22)
		out.Username = p.Username
		out.Password = p.Password

	case "shadowtls":
		out.Type = "shadowtls"
		out.Server = p.Server
		out.ServerPort = p.Port
		out.Password = p.Password
		if p.TLS || p.ServerName != "" || p.SNI != "" {
			out.TLS = buildTLS(p)
		}

	default:
		// 不再使用 anytls 兜底，避免生成无效配置
		return Outbound{}
	}

	return out
}

func buildTransport(p ClashProxy) *Transport {
	network := strings.ToLower(p.Network)
	switch network {
	case "ws":
		t := &Transport{Type: "ws"}
		if p.WSOpts != nil {
			t.Path = p.WSOpts.Path
			t.Headers = p.WSOpts.Headers
		}
		if t.Path == "" {
			t.Path = "/"
		}
		// 如果 headers 里没有 Host，使用 SNI/ServerName 作为 Host
		if t.Headers == nil || t.Headers["Host"] == "" {
			host := firstNonEmpty(p.SNI, p.ServerName)
			if host != "" {
				if t.Headers == nil {
					t.Headers = make(map[string]string)
				}
				t.Headers["Host"] = host
			}
		}
		return t
	case "grpc":
		if p.GRPCOpts == nil {
			return nil
		}
		return &Transport{
			Type:        "grpc",
			ServiceName: p.GRPCOpts.ServiceName,
		}
	case "http":
		return &Transport{Type: "http"}
	}
	return nil
}

func buildTLS(p ClashProxy) *TLSOptions {
	sni := firstNonEmpty(p.SNI, p.ServerName)
	tls := &TLSOptions{
		Enabled:    true,
		ServerName: sni,
		Insecure:   p.SkipCertVerify,
	}
	if len(p.ALPN) > 0 {
		tls.ALPN = p.ALPN
	}
	fp := firstNonEmpty(p.Fingerprint, p.ClientFingerprint)
	if fp != "" {
		tls.UTLS = &UTLSOptions{Enabled: true, Fingerprint: fp}
	}
	return tls
}

func buildGroupOutbounds(groups []ClashProxyGroup) []Outbound {
	var outbounds []Outbound
	for _, g := range groups {
		if g.Name == "" {
			continue
		}
		switch strings.ToLower(g.Type) {
		case "select":
			outbounds = append(outbounds, Outbound{
				Type:      "selector",
				Tag:       g.Name,
				Outbounds: g.Proxies,
			})
		case "url-test", "urltest", "fallback", "load-balance":
			outbounds = append(outbounds, Outbound{
				Type:      "urltest",
				Tag:       g.Name,
				Outbounds: g.Proxies,
			})
		default:
			// 其他类型也统一生成 selector，避免被上层引用时找不到依赖
			outbounds = append(outbounds, Outbound{
				Type:      "selector",
				Tag:       g.Name,
				Outbounds: g.Proxies,
			})
		}
	}
	return outbounds
}

func buildRoute(rules []string, groups []ClashProxyGroup) RouteOptions {
	return buildRouteWithRuleProviders(rules, nil)
}

func buildRouteWithRuleProviders(rules []string, providers map[string]interface{}) RouteOptions {
	var routeRules []RouteRule
	var finalOutbound string
	ruleSets := buildRuleSetsFromProviders(providers)
	knownRuleSets := make(map[string]bool, len(ruleSets))
	for _, rs := range ruleSets {
		knownRuleSets[rs.Tag] = true
	}

	// 流量嗅探规则
	routeRules = append(routeRules, RouteRule{
		Action: "sniff",
	})

	// DNS 劫持规则
	routeRules = append(routeRules, RouteRule{
		Action: "hijack-dns",
		Rule:   Rule{Protocol: []string{"dns"}},
	})

	for _, rule := range rules {
		parts := splitClashRule(rule)
		if len(parts) < 2 {
			continue
		}

		ruleType := strings.ToUpper(parts[0])
		var target, outTag string

		if len(parts) >= 3 {
			target = parts[1]
			outTag = parts[2]
		} else {
			outTag = parts[1]
		}

		if strings.EqualFold(outTag, "DIRECT") {
			outTag = "direct"
		} else if strings.EqualFold(outTag, "REJECT") {
			outTag = "block"
		}

		if ruleType == "MATCH" || ruleType == "FINAL" {
			finalOutbound = outTag
			continue
		}

		r := RouteRule{Action: "route", Outbound: outTag}

		switch ruleType {
		case "DOMAIN":
			r.Domain = []string{target}
		case "DOMAIN-SUFFIX":
			r.DomainSuffix = []string{target}
		case "DOMAIN-KEYWORD":
			r.DomainKeyword = []string{target}
		case "IP-CIDR", "IP-CIDR6":
			r.IPCIDR = []string{target}
		case "SRC-IP-CIDR":
			r.SourceIPCIDR = []string{target}
		case "GEOIP", "GEOSITE":
			rs, ok := defaultRuleSetForGeoRule(ruleType, target)
			if !ok {
				continue
			}
			if !knownRuleSets[rs.Tag] {
				ruleSets = append(ruleSets, rs)
				knownRuleSets[rs.Tag] = true
			}
			r.RuleSet = []string{rs.Tag}
		case "RULE-SET":
			if !knownRuleSets[target] {
				continue
			}
			r.RuleSet = []string{target}
		case "DST-PORT":
			ports := parsePortList(target)
			r.Port = ports
		case "SRC-PORT":
			ports := parsePortList(target)
			r.SourcePort = ports
		case "PROCESS-NAME":
			r.ProcessName = []string{target}
		case "PROCESS-PATH":
			r.ProcessPath = []string{target}
		case "NETWORK":
			r.Network = []string{strings.ToLower(target)}
		default:
			// 不支持的规则类型，跳过
			continue
		}

		routeRules = append(routeRules, r)
	}

	if finalOutbound == "" {
		finalOutbound = "direct"
	}

	return RouteOptions{
		AutoDetectInterface: true,
		Final:               finalOutbound,
		Rules:               routeRules,
		RuleSet:             ruleSets,
	}
}

func splitClashRule(rule string) []string {
	raw := strings.Split(rule, ",")
	parts := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		parts = append(parts, p)
	}
	return parts
}

func buildRuleSetsFromProviders(providers map[string]interface{}) []RuleSet {
	if len(providers) == 0 {
		return nil
	}
	ruleSets := make([]RuleSet, 0, len(providers))
	for name, raw := range providers {
		if strings.TrimSpace(name) == "" {
			continue
		}
		provider, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		rs, ok := ruleSetFromProvider(name, provider)
		if ok {
			ruleSets = append(ruleSets, rs)
		}
	}
	return ruleSets
}

func ruleSetFromProvider(name string, provider map[string]interface{}) (RuleSet, bool) {
	rs := RuleSet{Tag: name}
	providerType := strings.ToLower(mapString(provider, "type"))
	path := mapString(provider, "path")
	url := mapString(provider, "url")

	switch {
	case strings.HasSuffix(strings.ToLower(path), ".srs"):
		rs.Type = "local"
		rs.Path = path
		rs.Format = "binary"
	case strings.HasSuffix(strings.ToLower(url), ".srs"):
		rs.Type = "remote"
		rs.URL = url
		rs.Format = "binary"
	case providerType == "file" && path != "":
		rs.Type = "local"
		rs.Path = path
		rs.Format = ruleSetFormatFromPath(path)
	case (providerType == "http" || providerType == "https") && url != "":
		rs.Type = "remote"
		rs.URL = url
		rs.Format = ruleSetFormatFromPath(url)
	default:
		return RuleSet{}, false
	}
	if rs.Format == "" {
		return RuleSet{}, false
	}
	return rs, true
}

func mapString(m map[string]interface{}, key string) string {
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func ruleSetFormatFromPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".srs":
		return "binary"
	default:
		return ""
	}
}

func defaultRuleSetForGeoRule(ruleType, target string) (RuleSet, bool) {
	target = strings.ToLower(strings.TrimSpace(target))
	switch strings.ToUpper(ruleType) {
	case "GEOIP":
		if target == "cn" || target == "china" {
			return RuleSet{Type: "remote", Tag: "geoip-cn", Format: "binary", URL: defaultGeoIPCNRuleSetURL}, true
		}
	case "GEOSITE":
		if target == "cn" || target == "china" || target == "geolocation-cn" {
			return RuleSet{Type: "remote", Tag: "geosite-cn", Format: "binary", URL: defaultGeositeCNRuleSetURL}, true
		}
	}
	return RuleSet{}, false
}

func buildDNS(dns ClashDNS) *DNSOptions {
	// Clash 默认启用 DNS；只要配置了 nameserver/fallback 就认为启用
	if !dns.Enable && len(dns.Nameserver) == 0 && len(dns.Fallback) == 0 {
		return nil
	}

	servers := []DNSServer{}
	for i, ns := range dns.Nameserver {
		if ns = strings.TrimSpace(ns); ns != "" {
			servers = append(servers, dnsServerFromAddress(ns, fmt.Sprintf("local-%d", i), ""))
		}
	}
	for i, ns := range dns.Fallback {
		if ns = strings.TrimSpace(ns); ns != "" {
			servers = append(servers, dnsServerFromAddress(ns, fmt.Sprintf("remote-%d", i), "proxy"))
		}
	}

	// 兜底：如果没有解析到 DNS server，使用默认配置
	if len(servers) == 0 {
		servers = []DNSServer{
			dnsServerFromAddress("223.5.5.5", "local", ""),
			dnsServerFromAddress("https://1.1.1.1/dns-query", "remote", "proxy"),
		}
	}

	// 确保有一个 type: local 的 DNS server，用于解析其他域名型 DNS server 的地址
	servers = ensureLocalDNSServer(servers)

	return &DNSOptions{
		Servers: servers,
		Rules:   buildDNSRules(servers),
	}
}

// ========== 工具函数 ==========

func clashTypeToSingBox(t string) string {
	switch strings.ToLower(t) {
	case "ss":
		return "shadowsocks"
	case "ssr":
		return "shadowsocks"
	case "socks5":
		return "socks"
	case "any-tls":
		return "anytls"
	case "wg":
		return "wireguard"
	default:
		return strings.ToLower(t)
	}
}

func mapLogLevel(level string) string {
	switch strings.ToLower(level) {
	case "silent":
		return "error"
	case "error":
		return "warn"
	case "warning":
		return "warn"
	case "info":
		return "info"
	case "debug":
		return "debug"
	default:
		return "info"
	}
}

// 尝试 base64 解码，如果失败返回原数据
func DecodeBase64IfNeeded(data []byte) []byte {
	trimmed := strings.TrimSpace(string(data))
	if matched, _ := regexp.MatchString(`^[A-Za-z0-9+/=]+$`, trimmed); matched && len(trimmed) > 100 {
		decoded, err := base64.StdEncoding.DecodeString(trimmed)
		if err == nil {
			return decoded
		}
	}
	return data
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func firstPositive(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

func copyMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

func parseMbps(s string) int {
	// 支持 "100 Mbps" 或 "100"
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "mbps", "")
	s = strings.ReplaceAll(s, "mb/s", "")
	s = strings.TrimSpace(s)
	if n, err := strconv.Atoi(s); err == nil {
		return n
	}
	return 0
}

func parseIntList(s string) []int {
	parts := strings.Split(s, ",")
	var result []int
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if n, err := strconv.Atoi(p); err == nil {
			result = append(result, n)
		}
	}
	return result
}

func parsePortList(s string) []int {
	var result []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			// 端口范围
			bounds := strings.SplitN(part, "-", 2)
			if len(bounds) != 2 {
				continue
			}
			start, err1 := strconv.Atoi(strings.TrimSpace(bounds[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(bounds[1]))
			if err1 != nil || err2 != nil || start > end || end-start > 1000 {
				continue
			}
			for i := start; i <= end; i++ {
				result = append(result, i)
			}
		} else {
			if n, err := strconv.Atoi(part); err == nil {
				result = append(result, n)
			}
		}
	}
	return result
}
