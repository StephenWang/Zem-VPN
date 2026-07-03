package config

import (
	"encoding/json"
	"fmt"
	"strings"
)

// MergeMode 定义多订阅合并时的冲突处理方式
type MergeMode string

const (
	MergeModeUnion  MergeMode = "union"  // 合并所有节点与规则（默认）
	MergeModeSelect MergeMode = "select" // 每个订阅作为独立 selector
)

// Profile 是一个聚合多个订阅的配置视图
type Profile struct {
	ID            string   `json:"id"`
	Name          string   `json:"name"`
	SubscriptionIDs []string `json:"subscription_ids"`
	MergeMode     MergeMode `json:"merge_mode"`
}

// MergeSubscriptions 把多个订阅的 sing-box JSON 合并成一个配置
func MergeSubscriptions(jsons []string, mode MergeMode) (string, error) {
	if len(jsons) == 0 {
		return "", fmt.Errorf("no subscriptions to merge")
	}

	merged := &SingBoxConfig{
		Log: &LogOptions{Level: "info"},
		Inbounds: []Inbound{
			{Type: "tun", Tag: "tun-in", Address: []string{"172.19.0.1/30"}, AutoRoute: true, StrictRoute: true},
		},
		Route: RouteOptions{
			AutoDetectInterface: true,
			Final:               "direct",
			Rules: []RouteRule{
				{Action: "sniff"},
				{Action: "hijack-dns", Rule: Rule{Protocol: []string{"dns"}}},
			},
		},
	}

	var allDNS []DNSServer
	seenProxyTags := make(map[string]bool)
	proxyTypeSet := make(map[string]bool)
	for _, t := range ProxyTypes {
		proxyTypeSet[t] = true
	}

	var groupSelectors []Outbound

	for idx, jsonStr := range jsons {
		var cfg SingBoxConfig
		if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
			continue
		}

		// 收集该订阅的代理 tag
		var subProxyTags []string
		for _, out := range cfg.Outbounds {
			if proxyTypeSet[out.Type] {
				tag := uniqueTag(seenProxyTags, out.Tag)
				out.Tag = tag
				merged.Outbounds = append(merged.Outbounds, out)
				seenProxyTags[tag] = true
				subProxyTags = append(subProxyTags, tag)
			} else if out.Type == "selector" || out.Type == "urltest" {
				// 保留原分组但 tag 去重
				tag := uniqueTag(seenProxyTags, out.Tag)
				out.Tag = tag
				// 清理引用中不存在的 tag（后续统一处理）
				groupSelectors = append(groupSelectors, out)
				seenProxyTags[tag] = true
			}
		}

		// select 模式下，每个订阅生成一个 selector
		if mode == MergeModeSelect && len(subProxyTags) > 0 {
			groupTag := fmt.Sprintf("sub-%d", idx+1)
			if idx == 0 {
				groupTag = "sub-1"
			}
			merged.Outbounds = append(merged.Outbounds, Outbound{
				Type:      "selector",
				Tag:       groupTag,
				Outbounds: subProxyTags,
				Default:   subProxyTags[0],
			})
		}

		// 合并 DNS server（去重 tag）
		if cfg.DNS != nil {
			for _, s := range cfg.DNS.Servers {
				if s.Tag == "" {
					continue
				}
				s.Tag = uniqueDNSTag(allDNS, s.Tag)
				allDNS = append(allDNS, s)
			}
		}
	}

	// 添加默认 direct/block
	merged.Outbounds = append(merged.Outbounds,
		Outbound{Type: "direct", Tag: "direct"},
		Outbound{Type: "block", Tag: "block"},
	)

	// 重新整理 selector/urltest 引用
	existingTags := make(map[string]bool)
	for _, out := range merged.Outbounds {
		existingTags[out.Tag] = true
	}
	for i := range groupSelectors {
		filtered := make([]string, 0, len(groupSelectors[i].Outbounds))
		for _, tag := range groupSelectors[i].Outbounds {
			if existingTags[tag] {
				filtered = append(filtered, tag)
			}
		}
		if len(filtered) == 0 {
			filtered = []string{"direct"}
		}
		groupSelectors[i].Outbounds = filtered
		merged.Outbounds = append(merged.Outbounds, groupSelectors[i])
	}

	// 构建统一的 selected selector
	var proxyTags []string
	for _, out := range merged.Outbounds {
		if proxyTypeSet[out.Type] {
			proxyTags = append(proxyTags, out.Tag)
		}
	}
	if len(proxyTags) > 0 {
		merged.Outbounds = append(merged.Outbounds, Outbound{
			Type:      "selector",
			Tag:       "selected",
			Outbounds: proxyTags,
			Default:   proxyTags[0],
		})
		merged.Route.Final = "selected"
	}

	// DNS 兜底
	if len(allDNS) == 0 {
		allDNS = []DNSServer{
			dnsServerFromAddress("223.5.5.5", "local-0", ""),
			dnsServerFromAddress("https://1.1.1.1/dns-query", "remote-0", "proxy"),
		}
	}
	allDNS = ensureLocalDNSServer(allDNS)
	merged.DNS = &DNSOptions{
		Servers: allDNS,
		Rules:   buildDNSRules(allDNS),
	}

	result, err := json.MarshalIndent(merged, "", "  ")
	return string(result), err
}

func uniqueTag(seen map[string]bool, tag string) string {
	if tag == "" {
		tag = "node"
	}
	if !seen[tag] {
		return tag
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s_%d", tag, i)
		if !seen[candidate] {
			return candidate
		}
	}
}

func uniqueDNSTag(servers []DNSServer, tag string) string {
	seen := make(map[string]bool)
	for _, s := range servers {
		seen[s.Tag] = true
	}
	if !seen[tag] {
		return tag
	}
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s_%d", tag, i)
		if !seen[candidate] {
			return candidate
		}
	}
}

// BuildProfileFromURLs 通过 URL 列表直接构建一个合并后的 sing-box JSON（用于 proxy-provider）
func BuildProfileFromURLs(urls []string, fetcher func(string) ([]byte, error)) (string, error) {
	var jsons []string
	for _, u := range urls {
		data, err := fetcher(u)
		if err != nil {
			return "", fmt.Errorf("fetch %s: %w", u, err)
		}
		json, err := ConvertSubscriptionData(data)
		if err != nil {
			return "", fmt.Errorf("convert %s: %w", u, err)
		}
		jsons = append(jsons, json)
	}
	return MergeSubscriptions(jsons, MergeModeUnion)
}

// HasProxyProviderSupport 返回是否包含 proxy-provider/rule-provider 相关字段（Clash）
func HasProxyProviderSupport(data []byte) bool {
	text := strings.ToLower(string(data))
	return strings.Contains(text, "proxy-providers") || strings.Contains(text, "rule-providers")
}
