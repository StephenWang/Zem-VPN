package config

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConvertSubscriptionData 自动识别订阅格式并转换为 sing-box JSON。
// 支持：
//   - Clash YAML（含 proxy/proxy-groups/rules）
//   - sing-box JSON 原生配置
//   - base64 编码的 SS URL 列表（ss://...）
//   - 混合协议 URL 列表（ss://, vmess://, vless://, trojan://, ssr://, hysteria2://, tuic://）
func ConvertSubscriptionData(data []byte) (string, error) {
	decoded := DecodeBase64IfNeeded(data)

	// 1. 尝试 sing-box JSON 原生配置
	if looksLikeSingBoxJSON(decoded) {
		return normalizeSingBoxJSON(decoded)
	}

	// 2. 尝试混合协议 URL 列表（base64 编码或明文）
	if looksLikeMixedProtocolList(decoded) || looksLikeBase64ProtocolList(data) {
		return ConvertProtocolURLListToSingBox(decoded)
	}

	// 3. 尝试 SS URL 列表（优先，因为 SS URL 列表可能被 YAML 解析为空结构）
	if looksLikeSSURLList(decoded) {
		return ConvertSSURLListToSingBox(decoded)
	}

	// 4. 尝试 Clash YAML
	var clash ClashConfig
	if err := yaml.Unmarshal(decoded, &clash); err == nil && len(clash.Proxies) > 0 {
		return convertClashConfig(&clash)
	}

	// 5. YAML 解析成功但 proxies 为空；也尝试 SS URL/混合协议
	if result, err := ConvertSSURLListToSingBox(decoded); err == nil {
		return result, nil
	}
	if result, err := ConvertProtocolURLListToSingBox(decoded); err == nil {
		return result, nil
	}

	// 6. 再尝试一次 YAML，即使 proxies 为空也可能有 rules
	if err := yaml.Unmarshal(decoded, &clash); err == nil {
		return convertClashConfig(&clash)
	}

	return "", fmt.Errorf("unable to detect subscription format")
}

// ConvertClashToSingBox 保持旧接口，内部调用自动识别。
func ConvertClashToSingBox(yamlData []byte) (string, error) {
	return ConvertSubscriptionData(yamlData)
}

// convertClashConfig 把已解析的 ClashConfig 转换为 sing-box JSON。
func convertClashConfig(clash *ClashConfig) (string, error) {
	sb := &SingBoxConfig{
		Log: &LogOptions{
			Level: mapLogLevel(clash.LogLevel),
		},
		DNS: buildDNS(clash.DNS),
		Inbounds: []Inbound{
			{
				Type:        "tun",
				Tag:         "tun-in",
				Address:     []string{"172.19.0.1/30"},
				AutoRoute:   true,
				StrictRoute: true,
			},
		},
		Outbounds: append(buildOutbounds(clash.Proxies), buildGroupOutbounds(clash.ProxyGroups)...),
		Route:     buildRoute(clash.Rules, clash.ProxyGroups),
	}

	result, err := json.MarshalIndent(sb, "", "  ")
	return string(result), err
}

// HasProxyOutbounds 检查 sing-box JSON 中是否包含实际代理节点。
func HasProxyOutbounds(jsonStr string) bool {
	for _, t := range ProxyTypes {
		if strings.Contains(jsonStr, fmt.Sprintf(`"type": %q`, t)) {
			return true
		}
	}
	return false
}

// looksLikeSSURLList 判断数据是否是 ss:// 链接列表。
func looksLikeSSURLList(data []byte) bool {
	text := strings.TrimSpace(string(data))
	if text == "" {
		return false
	}
	lines := strings.Split(text, "\n")
	ssCount := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "ss://") {
			ssCount++
		} else {
			return false
		}
	}
	return ssCount > 0
}

// ConvertSSURLListToSingBox 把 SS URL 列表转换为 sing-box JSON。
func ConvertSSURLListToSingBox(data []byte) (string, error) {
	text := string(DecodeBase64IfNeeded(data))
	lines := strings.Split(text, "\n")

	var proxies []Outbound
	var proxyTags []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "ss://") {
			continue
		}
		out, err := parseSSURL(line)
		if err != nil {
			continue
		}
		if out.Tag == "" {
			out.Tag = fmt.Sprintf("%s:%d", out.Server, out.ServerPort)
		}
		proxies = append(proxies, out)
		proxyTags = append(proxyTags, out.Tag)
	}

	if len(proxies) == 0 {
		return "", fmt.Errorf("no valid ss:// urls found")
	}

	outbounds := append([]Outbound{}, proxies...)
	outbounds = append(outbounds,
		Outbound{Type: "direct", Tag: "direct"},
		Outbound{Type: "block", Tag: "block"},
	)

	// 生成默认分组
	outbounds = append(outbounds,
		Outbound{Type: "selector", Tag: "selected", Outbounds: proxyTags, Default: proxyTags[0]},
		Outbound{Type: "urltest", Tag: "自动选择", Outbounds: proxyTags},
		Outbound{Type: "urltest", Tag: "故障转移", Outbounds: proxyTags},
	)

	servers := []DNSServer{
		dnsServerFromAddress("223.5.5.5", "local-0", ""),
		dnsServerFromAddress("https://1.1.1.1/dns-query", "remote-0", "proxy"),
	}
	servers = ensureLocalDNSServer(servers)

	sb := &SingBoxConfig{
		Log: &LogOptions{Level: "info"},
		DNS: &DNSOptions{
			Servers: servers,
			Rules: []DNSRule{
				{Action: "route", Server: "remote-0", Rule: Rule{DomainSuffix: []string{"google.com", "youtube.com", "twitter.com", "facebook.com", "github.com", "cloudflare.com"}}},
				{Action: "route", Server: "local-0", Rule: Rule{DomainSuffix: []string{"cn"}}},
			},
		},
		Inbounds: []Inbound{
			{
				Type:        "tun",
				Tag:         "tun-in",
				Address:     []string{"172.19.0.1/30"},
				AutoRoute:   true,
				StrictRoute: true,
			},
		},
		Outbounds: outbounds,
		Route: RouteOptions{
			AutoDetectInterface: true,
			Final:               "selected",
			Rules: []RouteRule{
				{Action: "sniff"},
				{Action: "hijack-dns", Rule: Rule{Protocol: []string{"dns"}}},
			},
		},
	}

	result, err := json.MarshalIndent(sb, "", "  ")
	return string(result), err
}

// parseSSURL 解析单个 ss:// 链接。
// 支持格式：
//   ss://base64(method:password@server:port)#name
//   ss://method:password@server:port#name
func parseSSURL(line string) (Outbound, error) {
	body := strings.TrimPrefix(line, "ss://")

	var name string
	if idx := strings.LastIndex(body, "#"); idx >= 0 {
		name = body[idx+1:]
		body = body[:idx]
		if decoded, err := url.QueryUnescape(name); err == nil {
			name = decoded
		}
	}

	// body 现在应该是 [base64/明文](method:password)@server:port
	// 先分离 server:port
	atIdx := strings.LastIndex(body, "@")
	if atIdx < 0 {
		return Outbound{}, fmt.Errorf("missing @ in ss url")
	}

	credsPart := body[:atIdx]
	addr := body[atIdx+1:]

	// 尝试 base64 解码 credentials，失败则视为明文
	creds, err := base64DecodeBody(credsPart)
	if err != nil {
		creds = credsPart
	}

	colonIdx := strings.Index(creds, ":")
	if colonIdx < 0 {
		return Outbound{}, fmt.Errorf("missing method:password in ss url")
	}
	method := creds[:colonIdx]
	password := creds[colonIdx+1:]

	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return Outbound{}, fmt.Errorf("invalid server:port: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Outbound{}, fmt.Errorf("invalid port: %w", err)
	}

	return Outbound{
		Type:       "shadowsocks",
		Tag:        name,
		Server:     host,
		ServerPort: port,
		Method:     method,
		Password:   password,
	}, nil
}

// looksLikeSingBoxJSON 判断是否是 sing-box JSON 原生配置
func looksLikeSingBoxJSON(data []byte) bool {
	text := strings.TrimSpace(string(data))
	if !strings.HasPrefix(text, "{") {
		return false
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return false
	}
	_, hasOutbounds := raw["outbounds"]
	_, hasInbounds := raw["inbounds"]
	return hasOutbounds || hasInbounds
}

// normalizeSingBoxJSON 标准化 sing-box JSON：移除不支持的旧字段，补全默认路由/DNS
func normalizeSingBoxJSON(data []byte) (string, error) {
	var cfg SingBoxConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return "", fmt.Errorf("parse sing-box json: %w", err)
	}
	if cfg.Log == nil {
		cfg.Log = &LogOptions{Level: "info"}
	}
	if cfg.DNS == nil || len(cfg.DNS.Servers) == 0 {
		cfg.DNS = buildDNS(ClashDNS{})
	}
	if len(cfg.Inbounds) == 0 {
		cfg.Inbounds = []Inbound{
			{Type: "tun", Tag: "tun-in", Address: []string{"172.19.0.1/30"}, AutoRoute: true, StrictRoute: true},
		}
	}
	if len(cfg.Outbounds) == 0 {
		return "", fmt.Errorf("sing-box json has no outbounds")
	}
	// 确保有 direct/block
	hasDirect, hasBlock := false, false
	for _, out := range cfg.Outbounds {
		if out.Type == "direct" {
			hasDirect = true
		}
		if out.Type == "block" {
			hasBlock = true
		}
	}
	if !hasDirect {
		cfg.Outbounds = append(cfg.Outbounds, Outbound{Type: "direct", Tag: "direct"})
	}
	if !hasBlock {
		cfg.Outbounds = append(cfg.Outbounds, Outbound{Type: "block", Tag: "block"})
	}
	if cfg.Route.Final == "" {
		cfg.Route.Final = "direct"
	}

	result, err := json.MarshalIndent(cfg, "", "  ")
	return string(result), err
}
