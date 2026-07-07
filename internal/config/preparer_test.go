package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"zem/internal/settings"
)

func TestNormalizeDNSBasic(t *testing.T) {
	oldConfig := `{
  "dns": {
    "servers": [
      {"tag": "local", "address": "local"},
      {"tag": "remote", "address": "tls://8.8.8.8"}
    ]
  },
  "inbounds": [],
  "outbounds": [
    {"type":"direct","tag":"direct"},
    {"type":"block","tag":"block"}
  ],
  "route": {"auto_detect_interface":true,"final":"direct","rules":[]}
}`
	var cfg SingBoxConfig
	if err := json.Unmarshal([]byte(oldConfig), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	cfg.DNS = NormalizeDNS(cfg.DNS)
	if cfg.DNS == nil {
		t.Fatal("dns should not be nil")
	}
	if len(cfg.DNS.Servers) != 2 {
		t.Fatalf("expected 2 dns servers, got %d", len(cfg.DNS.Servers))
	}
	for _, s := range cfg.DNS.Servers {
		if s.Type == "" {
			t.Fatalf("dns server type should not be empty: %+v", s)
		}
	}
}

func TestFixOutbounds(t *testing.T) {
	outbounds := []Outbound{
		{Type: "vmess", Tag: "node1"},
		{Type: "dns", Tag: "dns-out"},
		{Type: "selector", Tag: "sel", Outbounds: []string{"node1", "dns-out", "direct"}},
	}
	fixed := FixOutbounds(outbounds)
	if len(fixed) != 2 {
		t.Fatalf("expected 2 outbounds, got %d: %+v", len(fixed), fixed)
	}
	for _, out := range fixed {
		if out.Type == "dns" {
			t.Fatal("dns outbound should be removed")
		}
		if out.Tag == "sel" {
			if len(out.Outbounds) != 2 || out.Outbounds[0] != "node1" || out.Outbounds[1] != "direct" {
				t.Fatalf("selector references should be cleaned: %+v", out.Outbounds)
			}
		}
	}
}

func TestFixOutboundsReferences(t *testing.T) {
	outbounds := []Outbound{
		{Type: "direct", Tag: "direct"},
		{Type: "selector", Tag: "sel", Outbounds: []string{"missing", "direct"}},
	}
	fixed := FixOutboundsReferences(outbounds)
	for _, out := range fixed {
		if out.Tag == "sel" {
			if len(out.Outbounds) != 1 || out.Outbounds[0] != "direct" {
				t.Fatalf("expected only direct, got %+v", out.Outbounds)
			}
		}
	}
}

func TestPrepareRuleModeForcesProxyFinal(t *testing.T) {
	dataDir := t.TempDir()
	rsDir := filepath.Join(dataDir, "rule-set")
	if err := os.MkdirAll(rsDir, 0755); err != nil {
		t.Fatalf("create rule-set dir: %v", err)
	}
	for _, name := range []string{"geosite-cn.srs", "geoip-cn.srs"} {
		if err := os.WriteFile(filepath.Join(rsDir, name), []byte("test"), 0644); err != nil {
			t.Fatalf("write rule-set: %v", err)
		}
	}

	configJSON := `{
  "inbounds": [],
  "outbounds": [
    {"type":"trojan","tag":"node1","server":"1.1.1.1","server_port":443,"password":"pw"},
    {"type":"direct","tag":"direct"},
    {"type":"block","tag":"block"}
  ],
  "route": {"final":"direct","rules":[]}
}`
	var cfg SingBoxConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := Prepare(&cfg, PrepareOptions{
		DataDir:   dataDir,
		ProxyPort: 7890,
		ProxyMode: "rule",
		TunSettings: settings.TunSettings{
			Address:   []string{"172.19.0.1/30"},
			Stack:     "mixed",
			MTU:       9000,
			AutoRoute: true,
		},
		SubID: "sub1",
	}); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if cfg.Route.Final != "selected" {
		t.Fatalf("rule mode should force final to selected, got %s", cfg.Route.Final)
	}
	if len(cfg.Route.RuleSet) < 2 {
		t.Fatalf("expected rule sets, got %+v", cfg.Route.RuleSet)
	}
}

func TestPreparePreservesSelectedServer(t *testing.T) {
	configJSON := `{
  "inbounds": [],
  "outbounds": [
    {"type":"vmess","tag":"node1","server":"1.1.1.1","server_port":443},
    {"type":"vmess","tag":"node2","server":"2.2.2.2","server_port":443},
    {"type":"selector","tag":"selected","outbounds":["node1","node2"],"default":"node2"}
  ],
  "route": {"final":"selected"}
}`
	var cfg SingBoxConfig
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if err := Prepare(&cfg, PrepareOptions{
		DataDir:     t.TempDir(),
		ProxyPort:   7890,
		ProxyMode:   "rule",
		TunSettings: settings.DefaultTunSettings(),
		SelectedNode: "node2",
		SubID:       "sub1",
	}); err != nil {
		t.Fatalf("prepare: %v", err)
	}
	var selected *Outbound
	for i := range cfg.Outbounds {
		if cfg.Outbounds[i].Tag == "selected" {
			selected = &cfg.Outbounds[i]
			break
		}
	}
	if selected == nil {
		t.Fatal("selected selector not found")
	}
	if selected.Default != "node2" {
		t.Fatalf("expected selected default node2, got %s", selected.Default)
	}
}

func TestChinaRuleSetsPreferLocal(t *testing.T) {
	dataDir := t.TempDir()
	rsDir := filepath.Join(dataDir, "rule-set")
	if err := os.MkdirAll(rsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(rsDir, "geosite-cn.srs"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	rs := ChinaRuleSets(dataDir)
	if len(rs) != 2 {
		t.Fatalf("expected 2 rule sets, got %d", len(rs))
	}
	var seenLocal bool
	for _, r := range rs {
		if r.Tag == "geosite-cn" && r.Type == "local" {
			seenLocal = true
		}
		if r.Tag == "geoip-cn" && r.Type != "remote" {
			t.Fatalf("geoip-cn should be remote when not cached, got %+v", r)
		}
	}
	if !seenLocal {
		t.Fatal("expected local geosite-cn")
	}
}

func TestMergeRuleSets(t *testing.T) {
	existing := []RuleSet{{Type: "remote", Tag: "a", URL: "u1"}}
	extra := []RuleSet{{Type: "remote", Tag: "a", URL: "u2"}, {Type: "remote", Tag: "b", URL: "u3"}}
	merged := MergeRuleSets(existing, extra)
	if len(merged) != 2 {
		t.Fatalf("expected 2 rule sets, got %d", len(merged))
	}
	for _, rs := range merged {
		if rs.Tag == "a" && rs.URL != "u1" {
			t.Fatalf("existing should win: %+v", rs)
		}
	}
}

func TestGuessCountry(t *testing.T) {
	if got := GuessCountry("US-Silicon Valley"); got != "美国" {
		t.Fatalf("expected 美国, got %s", got)
	}
	if got := GuessCountry("HK-IEPL"); got != "香港" {
		t.Fatalf("expected 香港, got %s", got)
	}
}
