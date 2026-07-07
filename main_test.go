package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	singjson "github.com/sagernet/sing/common/json"
	"zem/internal/config"
	"zem/internal/settings"
)

func TestFixLegacyDNS(t *testing.T) {
	oldConfig := `{
  "dns": {
    "servers": [
      {"tag": "local", "address": "local"},
      {"tag": "remote", "address": "tls://8.8.8.8"}
    ]
  },
  "inbounds": [{"type":"tun","tag":"tun-in","address":["172.19.0.1/30"],"auto_route":true,"strict_route":true}],
  "outbounds": [
    {"type":"direct","tag":"direct"},
    {"type":"block","tag":"block"}
  ],
  "route": {"auto_detect_interface":true,"final":"direct","rules":[]}
}`

	var cfg config.SingBoxConfig
	if err := json.Unmarshal([]byte(oldConfig), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	cfg.DNS = fixLegacyDNS(cfg.DNS)
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

	out, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	ctx := include.Context(context.Background())
	_, err = singjson.UnmarshalExtendedContext[option.Options](ctx, out)
	if err != nil {
		t.Fatalf("parse fixed config: %v", err)
	}
}

func TestPrepareConfigRuleModeForcesProxyFinal(t *testing.T) {
	dataDir := t.TempDir()
	app := &App{
		settings: settings.NewManager(dataDir),
		dataDir:  dataDir,
	}

	ruleSetDir := filepath.Join(dataDir, "rule-set")
	if err := os.MkdirAll(ruleSetDir, 0755); err != nil {
		t.Fatalf("create rule-set dir: %v", err)
	}
	for _, name := range []string{"geosite-cn.srs", "geoip-cn.srs"} {
		if err := os.WriteFile(filepath.Join(ruleSetDir, name), []byte("test"), 0644); err != nil {
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

	prepared, err := app.prepareConfig(configJSON, "sub1")
	if err != nil {
		t.Fatalf("prepare config: %v", err)
	}

	var cfg config.SingBoxConfig
	if err := json.Unmarshal([]byte(prepared), &cfg); err != nil {
		t.Fatalf("unmarshal prepared: %v", err)
	}
	if cfg.Route.Final != "selected" {
		t.Fatalf("rule mode should force final to selected, got %s", cfg.Route.Final)
	}
}

func TestFixMissingDomainResolver(t *testing.T) {
	oldConfig := `{
  "dns": {
    "servers": [
      {"type": "https", "tag": "local", "server": "doh.pub", "server_port": 443},
      {"type": "https", "tag": "remote", "server": "doh-pure.onedns.net", "server_port": 443, "detour": "proxy"},
      {"type": "https", "tag": "remote", "server": "223.5.5.5", "server_port": 443, "detour": "proxy"}
    ]
  },
  "inbounds": [{"type":"tun","tag":"tun-in","address":["172.19.0.1/30"],"auto_route":true,"strict_route":true}],
  "outbounds": [
    {"type":"direct","tag":"direct"},
    {"type":"block","tag":"block"}
  ],
  "route": {"auto_detect_interface":true,"final":"direct","rules":[]}
}`

	var cfg config.SingBoxConfig
	if err := json.Unmarshal([]byte(oldConfig), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	cfg.DNS = fixLegacyDNS(cfg.DNS)
	if cfg.DNS == nil {
		t.Fatal("dns should not be nil")
	}

	var hasLocal bool
	for _, s := range cfg.DNS.Servers {
		if s.Type == "local" {
			hasLocal = true
		}
		if s.Server == "doh.pub" || s.Server == "doh-pure.onedns.net" {
			if s.DomainResolver == nil || s.DomainResolver.Server != "local" {
				t.Fatalf("server %s missing domain_resolver: %+v", s.Server, s)
			}
		}
		if s.Server == "223.5.5.5" && s.DomainResolver != nil {
			t.Fatalf("IP server should not have domain_resolver: %+v", s)
		}
	}
	if !hasLocal {
		t.Fatal("should have local DNS server")
	}

	out, _ := json.Marshal(cfg)
	ctx := include.Context(context.Background())
	_, err := singjson.UnmarshalExtendedContext[option.Options](ctx, out)
	if err != nil {
		t.Fatalf("parse fixed config: %v", err)
	}
}

func TestConvertDNSFormats(t *testing.T) {
	yaml := `
dns:
  enable: true
  nameserver:
    - 223.5.5.5
  fallback:
    - https://1.1.1.1/dns-query
proxies: []
proxy-groups: []
rules:
  - MATCH,DIRECT
`
	jsonData, err := config.ConvertClashToSingBox([]byte(yaml))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	ctx := include.Context(context.Background())
	_, err = singjson.UnmarshalExtendedContext[option.Options](ctx, []byte(jsonData))
	if err != nil {
		t.Fatalf("parse converted config: %v", err)
	}
}

func TestPrepareConfigPreservesSelectedServer(t *testing.T) {
	dataDir := t.TempDir()
	app := &App{
		settings: settings.NewManager(dataDir),
		dataDir:  dataDir,
	}

	configJSON := `{
  "inbounds": [],
  "outbounds": [
    {"type":"vmess","tag":"node1","server":"1.1.1.1","server_port":443},
    {"type":"vmess","tag":"node2","server":"2.2.2.2","server_port":443},
    {"type":"selector","tag":"selected","outbounds":["node1","node2"],"default":"node2"}
  ],
  "route": {"final":"selected"}
}`

	prepared, err := app.prepareConfig(configJSON, "sub1")
	if err != nil {
		t.Fatalf("prepare config: %v", err)
	}

	var cfg config.SingBoxConfig
	if err := json.Unmarshal([]byte(prepared), &cfg); err != nil {
		t.Fatalf("unmarshal prepared: %v", err)
	}

	var selected *config.Outbound
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

	ctx := include.Context(context.Background())
	if _, err := singjson.UnmarshalExtendedContext[option.Options](ctx, []byte(prepared)); err != nil {
		t.Fatalf("parse prepared config: %v", err)
	}
}
