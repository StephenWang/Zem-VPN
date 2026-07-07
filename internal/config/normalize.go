package config

import (
	"net"
	"sort"
	"strconv"
	"strings"
)

// NormalizeDNS 将旧版 DNS server 格式替换为 sing-box 1.14 兼容的新格式。
// 包括：过滤损坏 server、为域名型 server 补全 domain_resolver、确保有 local server。
func NormalizeDNS(dns *DNSOptions) *DNSOptions {
	if dns == nil {
		return nil
	}
	fixed := make([]DNSServer, 0, len(dns.Servers))
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
			s.DomainResolver = &DomainResolverOptions{Server: "local"}
		}
		fixed = append(fixed, s)
	}
	if len(fixed) == 0 {
		// 没有任何有效 server 时，使用默认配置
		fixed = []DNSServer{
			{Type: "local", Tag: "local"},
			{Type: "https", Tag: "remote", Server: "1.1.1.1", ServerPort: 443, Detour: "direct"},
		}
		hasLocal = true
	}
	// 确保有 local DNS server 作为 domain resolver
	if !hasLocal {
		fixed = append([]DNSServer{{Type: "local", Tag: "local"}}, fixed...)
	}
	dns.Servers = fixed
	return dns
}

// FixOutbounds 过滤 sing-box 1.13+ 已移除的 outbound 类型（如 dns），
// 并清理 selector/urltest 中对被移除节点的引用。
func FixOutbounds(outbounds []Outbound) []Outbound {
	filtered := make([]Outbound, 0, len(outbounds))
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

// FixOutboundsReferences 清理 selector/urltest 中对不存在 outbound 的引用，
// 避免启动时报 dependency not found。
func FixOutboundsReferences(outbounds []Outbound) []Outbound {
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

func filterStrings(items []string, removed map[string]bool) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		if !removed[item] {
			result = append(result, item)
		}
	}
	return result
}

// ensureLocalDNSServer 如果没有本地 DNS server，则在开头插入一个，作为 address resolver
func ensureLocalDNSServer(servers []DNSServer) []DNSServer {
	for _, s := range servers {
		if s.Type == "local" {
			return servers
		}
	}
	return append([]DNSServer{dnsServerFromAddress("local", "local", "")}, servers...)
}

// buildDNSRules 根据实际存在的 server tag 生成 DNS 规则，避免引用不存在的 tag
// 同时避开 sing-box 1.14 已移除的 geosite/geoip 旧格式
func buildDNSRules(servers []DNSServer) []DNSRule {
	var localTag, remoteTag string
	for _, s := range servers {
		if strings.HasPrefix(s.Tag, "local") && localTag == "" {
			localTag = s.Tag
		}
		if strings.HasPrefix(s.Tag, "remote") && remoteTag == "" {
			remoteTag = s.Tag
		}
	}

	var rules []DNSRule
	if remoteTag != "" {
		rules = append(rules, DNSRule{
			Action: "route",
			Server: remoteTag,
			Rule:   Rule{DomainSuffix: []string{"google.com", "youtube.com", "twitter.com", "facebook.com", "github.com", "cloudflare.com"}},
		})
	}
	if localTag != "" {
		rules = append(rules, DNSRule{
			Action: "route",
			Server: localTag,
			Rule:   Rule{DomainSuffix: []string{"cn"}},
		})
	}
	return rules
}

// dnsServerFromAddress 从 address 字符串构建 DNSServer
func dnsServerFromAddress(address, tag, detour string) DNSServer {
	s := DNSServer{Tag: tag, Detour: detour}
	address = strings.TrimSpace(address)

	switch {
	case strings.HasPrefix(address, "https://"):
		s.Type = "https"
		u := strings.TrimPrefix(address, "https://")
		if idx := strings.Index(u, "/"); idx > 0 {
			s.Server = u[:idx]
		} else {
			s.Server = u
		}
		s.ServerPort = 443
		if net.ParseIP(s.Server) == nil {
			s.DomainResolver = &DomainResolverOptions{Server: "local"}
		}
	case strings.HasPrefix(address, "tls://"):
		s.Type = "tls"
		s.Server = strings.TrimPrefix(address, "tls://")
		s.ServerPort = 853
		if net.ParseIP(s.Server) == nil {
			s.DomainResolver = &DomainResolverOptions{Server: "local"}
		}
	case strings.HasPrefix(address, "dhcp://"):
		s.Type = "dhcp"
	case address == "system" || address == "local":
		s.Type = "local"
	default:
		// 默认作为 UDP DNS
		s.Type = "udp"
		if idx := strings.LastIndex(address, ":"); idx > 0 {
			s.Server = address[:idx]
			if port, err := strconv.Atoi(address[idx+1:]); err == nil {
				s.ServerPort = port
			}
		} else {
			s.Server = address
			s.ServerPort = 53
		}
		if net.ParseIP(s.Server) == nil {
			s.DomainResolver = &DomainResolverOptions{Server: "local"}
		}
	}
	return s
}

// GuessCountry 从节点名称中猜测国家/地区（纯函数，便于测试）
func GuessCountry(tag string) string {
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
