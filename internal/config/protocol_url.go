package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// ParseProtocolURL 解析单个协议链接，支持 ss://, vmess://, vless://, trojan://, ssr://, hysteria2://, tuic://
func ParseProtocolURL(line string) (Outbound, error) {
	line = strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(line, "ss://"):
		return parseSSURL(line)
	case strings.HasPrefix(line, "vmess://"):
		return parseVmessURL(line)
	case strings.HasPrefix(line, "vless://"):
		return parseVLESSURL(line)
	case strings.HasPrefix(line, "trojan://"):
		return parseTrojanURL(line)
	case strings.HasPrefix(line, "ssr://"):
		return parseSSRURL(line)
	case strings.HasPrefix(line, "hysteria2://") || strings.HasPrefix(line, "hy2://"):
		return parseHysteria2URL(line)
	case strings.HasPrefix(line, "tuic://"):
		return parseTUICURL(line)
	default:
		return Outbound{}, fmt.Errorf("unsupported protocol URL")
	}
}

// parseVmessURL 解析 vmess:// 链接（VMessAEAD / V2RayN 格式）
func parseVmessURL(line string) (Outbound, error) {
	body := strings.TrimPrefix(line, "vmess://")
	body = strings.TrimSpace(body)

	// 尝试 base64 解码 JSON
	decoded, err := base64DecodeBody(body)
	if err != nil {
		return Outbound{}, fmt.Errorf("decode vmess: %w", err)
	}

	var vm struct {
		V    string `json:"v"`
		Ps   string `json:"ps"`
		Add  string `json:"add"`
		Port string `json:"port"`
		ID   string `json:"id"`
		Aid  string `json:"aid"`
		Net  string `json:"net"`
		Type string `json:"type"`
		Host string `json:"host"`
		Path string `json:"path"`
		TLS  string `json:"tls"`
		SNI  string `json:"sni"`
		Alpn string `json:"alpn"`
		Scy  string `json:"scy"`
	}
	if err := json.Unmarshal([]byte(decoded), &vm); err != nil {
		return Outbound{}, fmt.Errorf("parse vmess json: %w", err)
	}

	port, _ := strconv.Atoi(vm.Port)
	if port == 0 {
		port = 443
	}

	out := Outbound{
		Type:       "vmess",
		Tag:        firstNonEmpty(vm.Ps, fmt.Sprintf("%s:%d", vm.Add, port)),
		Server:     vm.Add,
		ServerPort: port,
		UUID:       vm.ID,
		// sing-box 1.13+ 已移除 alterId（仅支持 VMessAEAD）
		AlterID:  0,
		Security: firstNonEmpty(vm.Scy, "auto"),
	}

	network := strings.ToLower(vm.Net)
	if network == "" {
		network = "tcp"
	}
	if network == "ws" || network == "grpc" || network == "http" || network == "h2" {
		out.Transport = &Transport{Type: network}
		if network == "ws" || network == "h2" || network == "http" {
			out.Transport.Path = vm.Path
		if vm.Host != "" {
			if out.Transport.Headers == nil {
				out.Transport.Headers = make(map[string]string)
			}
			out.Transport.Headers["Host"] = vm.Host
		}
		}
		if network == "grpc" {
			out.Transport.ServiceName = vm.Path
		}
	}

	if strings.ToLower(vm.TLS) == "tls" || strings.ToLower(vm.TLS) == "true" || strings.ToLower(vm.TLS) == "xtls" {
		out.TLS = &TLSOptions{
			Enabled:    true,
			ServerName: firstNonEmpty(vm.SNI, vm.Host),
		}
		if vm.Alpn != "" {
			out.TLS.ALPN = strings.Split(vm.Alpn, ",")
		}
	}

	return out, nil
}

// parseVLESSURL 解析 vless:// 链接
func parseVLESSURL(line string) (Outbound, error) {
	body := strings.TrimPrefix(line, "vless://")
	body = strings.TrimSpace(body)

	var name string
	if idx := strings.LastIndex(body, "#"); idx >= 0 {
		name = body[idx+1:]
		body = body[:idx]
		if decoded, err := url.QueryUnescape(name); err == nil {
			name = decoded
		}
	}

	u, err := url.Parse("vless://" + body)
	if err != nil {
		return Outbound{}, fmt.Errorf("parse vless url: %w", err)
	}

	uuid := u.User.Username()
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		return Outbound{}, fmt.Errorf("invalid vless host: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Outbound{}, fmt.Errorf("invalid vless port: %w", err)
	}

	out := Outbound{
		Type:       "vless",
		Tag:        firstNonEmpty(name, fmt.Sprintf("%s:%d", host, port)),
		Server:     host,
		ServerPort: port,
		UUID:       uuid,
	}

	q := u.Query()
	network := strings.ToLower(q.Get("type"))
	if network == "" {
		network = "tcp"
	}
	if network == "ws" || network == "grpc" || network == "http" {
		out.Transport = &Transport{Type: network}
		path := q.Get("path")
		if path == "" {
			path = "/"
		}
		out.Transport.Path = path
		hostHeader := q.Get("host")
		if hostHeader != "" {
			if out.Transport.Headers == nil {
				out.Transport.Headers = make(map[string]string)
			}
			out.Transport.Headers["Host"] = hostHeader
		}
		if network == "grpc" {
			out.Transport.ServiceName = q.Get("serviceName")
		}
	}

	security := strings.ToLower(q.Get("security"))
	if security == "tls" || security == "xtls" || security == "reality" {
		out.TLS = &TLSOptions{
			Enabled:    true,
			ServerName: firstNonEmpty(q.Get("sni"), q.Get("host")),
		}
		alpn := q.Get("alpn")
		if alpn != "" {
			out.TLS.ALPN = strings.Split(alpn, ",")
		}
	}
	if security == "reality" && out.TLS != nil {
		out.TLS.Reality = &RealityOptions{
			Enabled:   true,
			PublicKey: q.Get("pbk"),
			ShortID:   q.Get("sid"),
		}
	}

	return out, nil
}

// parseTrojanURL 解析 trojan:// 链接
func parseTrojanURL(line string) (Outbound, error) {
	body := strings.TrimPrefix(line, "trojan://")
	body = strings.TrimSpace(body)

	var name string
	if idx := strings.LastIndex(body, "#"); idx >= 0 {
		name = body[idx+1:]
		body = body[:idx]
		if decoded, err := url.QueryUnescape(name); err == nil {
			name = decoded
		}
	}

	u, err := url.Parse("trojan://" + body)
	if err != nil {
		return Outbound{}, fmt.Errorf("parse trojan url: %w", err)
	}

	password, _ := u.User.Password()
	if password == "" {
		password = u.User.Username()
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		return Outbound{}, fmt.Errorf("invalid trojan host: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Outbound{}, fmt.Errorf("invalid trojan port: %w", err)
	}

	out := Outbound{
		Type:       "trojan",
		Tag:        firstNonEmpty(name, fmt.Sprintf("%s:%d", host, port)),
		Server:     host,
		ServerPort: port,
		Password:   password,
	}

	q := u.Query()
	network := strings.ToLower(q.Get("type"))
	if network == "ws" || network == "grpc" || network == "http" {
		out.Transport = &Transport{Type: network}
		path := q.Get("path")
		if path == "" {
			path = "/"
		}
		out.Transport.Path = path
		hostHeader := q.Get("host")
		if hostHeader != "" {
			if out.Transport.Headers == nil {
				out.Transport.Headers = make(map[string]string)
			}
			out.Transport.Headers["Host"] = hostHeader
		}
		if network == "grpc" {
			out.Transport.ServiceName = q.Get("serviceName")
		}
	}

	if strings.ToLower(q.Get("security")) == "tls" || q.Get("sni") != "" || q.Get("allowInsecure") == "1" {
		out.TLS = &TLSOptions{
			Enabled:    true,
			ServerName: q.Get("sni"),
			Insecure:   q.Get("allowInsecure") == "1" || strings.ToLower(q.Get("allowInsecure")) == "true",
		}
		alpn := q.Get("alpn")
		if alpn != "" {
			out.TLS.ALPN = strings.Split(alpn, ",")
		}
	}

	return out, nil
}

// parseSSRURL 解析 ssr:// 链接
func parseSSRURL(line string) (Outbound, error) {
	body := strings.TrimPrefix(line, "ssr://")
	body = strings.TrimSpace(body)

	decoded, err := base64DecodeBody(body)
	if err != nil {
		return Outbound{}, fmt.Errorf("decode ssr: %w", err)
	}

	// 格式：server:port:protocol:method:obfs:password_base64/?params&protoparam=...&obfsparam=...&remarks=...
	parts := strings.SplitN(decoded, "/?", 2)
	serverPart := parts[0]
	var query string
	if len(parts) == 2 {
		query = parts[1]
	}

	serverInfo := strings.Split(serverPart, ":")
	if len(serverInfo) < 6 {
		return Outbound{}, fmt.Errorf("invalid ssr url")
	}

	server := serverInfo[0]
	port, _ := strconv.Atoi(serverInfo[1])
	protocol := serverInfo[2]
	method := serverInfo[3]
	obfs := serverInfo[4]
	passB64 := serverInfo[5]
	password, _ := base64DecodeBody(passB64)

	q, _ := url.ParseQuery(query)
	remarks, _ := base64DecodeBody(q.Get("remarks"))
	if remarks == "" {
		remarks = fmt.Sprintf("%s:%d", server, port)
	}

	protoParam, _ := base64DecodeBody(q.Get("protoparam"))
	obfsParam, _ := base64DecodeBody(q.Get("obfsparam"))

	out := Outbound{
		Type:       "shadowsocks",
		Tag:        remarks,
		Server:     server,
		ServerPort: port,
		Method:     method,
		Password:   password,
		PluginOpts: map[string]interface{}{
			"protocol":       protocol,
			"protocol-param": protoParam,
			"obfs":           obfs,
			"obfs-param":     obfsParam,
		},
	}
	return out, nil
}

// parseHysteria2URL 解析 hysteria2:// / hy2:// 链接
func parseHysteria2URL(line string) (Outbound, error) {
	body := strings.TrimPrefix(line, "hysteria2://")
	if strings.HasPrefix(line, "hy2://") {
		body = strings.TrimPrefix(line, "hy2://")
	}
	body = strings.TrimSpace(body)

	var name string
	if idx := strings.LastIndex(body, "#"); idx >= 0 {
		name = body[idx+1:]
		body = body[:idx]
		if decoded, err := url.QueryUnescape(name); err == nil {
			name = decoded
		}
	}

	u, err := url.Parse("hysteria2://" + body)
	if err != nil {
		return Outbound{}, fmt.Errorf("parse hysteria2 url: %w", err)
	}

	password, _ := u.User.Password()
	if password == "" {
		password = u.User.Username()
	}
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		return Outbound{}, fmt.Errorf("invalid hysteria2 host: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Outbound{}, fmt.Errorf("invalid hysteria2 port: %w", err)
	}

	out := Outbound{
		Type:       "hysteria2",
		Tag:        firstNonEmpty(name, fmt.Sprintf("%s:%d", host, port)),
		Server:     host,
		ServerPort: port,
		Password:   password,
	}

	q := u.Query()
	sni := q.Get("sni")
	if sni != "" {
		out.TLS = &TLSOptions{Enabled: true, ServerName: sni}
	}
	if obfsType := q.Get("obfs"); obfsType != "" {
		out.Obfs = &Hysteria2ObfsOptions{
			Type:     obfsType,
			Password: q.Get("obfs-password"),
		}
	}
	if up := q.Get("upmbps"); up != "" {
		out.UpMbps = parseMbps(up)
	}
	if down := q.Get("downmbps"); down != "" {
		out.DownMbps = parseMbps(down)
	}

	return out, nil
}

// parseTUICURL 解析 tuic:// 链接
func parseTUICURL(line string) (Outbound, error) {
	body := strings.TrimPrefix(line, "tuic://")
	body = strings.TrimSpace(body)

	var name string
	if idx := strings.LastIndex(body, "#"); idx >= 0 {
		name = body[idx+1:]
		body = body[:idx]
		if decoded, err := url.QueryUnescape(name); err == nil {
			name = decoded
		}
	}

	u, err := url.Parse("tuic://" + body)
	if err != nil {
		return Outbound{}, fmt.Errorf("parse tuic url: %w", err)
	}

	uuid := u.User.Username()
	password, _ := u.User.Password()
	host, portStr, err := net.SplitHostPort(u.Host)
	if err != nil {
		return Outbound{}, fmt.Errorf("invalid tuic host: %w", err)
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return Outbound{}, fmt.Errorf("invalid tuic port: %w", err)
	}

	out := Outbound{
		Type:       "tuic",
		Tag:        firstNonEmpty(name, fmt.Sprintf("%s:%d", host, port)),
		Server:     host,
		ServerPort: port,
		UUID:       uuid,
		Password:   password,
	}

	q := u.Query()
	sni := q.Get("sni")
	if sni != "" {
		out.TLS = &TLSOptions{Enabled: true, ServerName: sni}
	}
	out.Congestion = q.Get("congestion_control")
	if out.Congestion == "" {
		out.Congestion = q.Get("congestion")
	}

	return out, nil
}

// isProtocolURL 判断是否是单个协议分享链接
func isProtocolURL(line string) bool {
	prefixes := []string{"ss://", "vmess://", "vless://", "trojan://", "ssr://", "hysteria2://", "hy2://", "tuic://"}
	for _, p := range prefixes {
		if strings.HasPrefix(line, p) {
			return true
		}
	}
	return false
}

// looksLikeBase64ProtocolList 判断解码后是否是 base64 编码的混合协议链接列表
func looksLikeBase64ProtocolList(data []byte) bool {
	text := strings.TrimSpace(string(DecodeBase64IfNeeded(data)))
	if text == "" {
		return false
	}
	lines := strings.Split(text, "\n")
	valid := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if isProtocolURL(line) {
			valid++
		} else {
			return false
		}
	}
	return valid > 0
}

// looksLikeMixedProtocolList 判断是否是明文混合协议链接列表
func looksLikeMixedProtocolList(data []byte) bool {
	text := strings.TrimSpace(string(data))
	if text == "" {
		return false
	}
	lines := strings.Split(text, "\n")
	valid := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if isProtocolURL(line) {
			valid++
		} else {
			return false
		}
	}
	return valid > 0
}

// ConvertProtocolURLListToSingBox 把混合协议 URL 列表转换为 sing-box JSON
func ConvertProtocolURLListToSingBox(data []byte) (string, error) {
	text := string(DecodeBase64IfNeeded(data))
	lines := strings.Split(text, "\n")

	var proxies []Outbound
	var proxyTags []string
	seen := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !isProtocolURL(line) {
			continue
		}
		out, err := ParseProtocolURL(line)
		if err != nil {
			continue
		}
		if out.Tag == "" {
			out.Tag = fmt.Sprintf("%s:%d", out.Server, out.ServerPort)
		}
		// 去重 tag
		originalTag := out.Tag
		for seen[out.Tag] {
			out.Tag = originalTag + "_" + strconv.Itoa(len(seen)+1)
		}
		seen[out.Tag] = true

		proxies = append(proxies, out)
		proxyTags = append(proxyTags, out.Tag)
	}

	if len(proxies) == 0 {
		return "", fmt.Errorf("no valid protocol urls found")
	}

	return buildSimpleSingBoxJSON(proxies, proxyTags)
}

// buildSimpleSingBoxJSON 从代理列表构建基础 sing-box 配置
func buildSimpleSingBoxJSON(proxies []Outbound, proxyTags []string) (string, error) {
	outbounds := append([]Outbound{}, proxies...)
	outbounds = append(outbounds,
		Outbound{Type: "direct", Tag: "direct"},
		Outbound{Type: "block", Tag: "block"},
	)

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

// base64DecodeBody 尝试用多种 base64 变体解码
func base64DecodeBody(body string) (string, error) {
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, enc := range encodings {
		if decoded, err := enc.DecodeString(body); err == nil {
			return string(decoded), nil
		}
	}
	return "", fmt.Errorf("unable to decode base64")
}
