package config

import (
	"strings"
	"testing"
)

func TestBuildOutboundVMess(t *testing.T) {
	p := ClashProxy{
		Name:    "vmess-node",
		Type:    "vmess",
		Server:  "1.2.3.4",
		Port:    443,
		UUID:    "uuid-1234",
		AlterID: 0,
		Network: "ws",
		WSOpts:  &ClashWSOpts{Path: "/path"},
		TLS:     true,
		SNI:     "example.com",
	}
	out := buildOutbound(p)
	if out.Type != "vmess" {
		t.Fatalf("expected vmess, got %s", out.Type)
	}
	if out.Server != "1.2.3.4" || out.ServerPort != 443 {
		t.Fatalf("unexpected server info: %+v", out)
	}
	if out.Transport == nil || out.Transport.Type != "ws" {
		t.Fatalf("expected ws transport: %+v", out.Transport)
	}
	if out.TLS == nil || out.TLS.ServerName != "example.com" {
		t.Fatalf("expected tls: %+v", out.TLS)
	}
}

func TestBuildOutboundVLESSReality(t *testing.T) {
	p := ClashProxy{
		Name:        "vless-node",
		Type:        "vless",
		Server:      "1.2.3.4",
		Port:        443,
		UUID:        "uuid-1234",
		TLS:         true,
		SNI:         "example.com",
		RealityOpts: &ClashRealityOpts{PublicKey: "pk", ShortID: "sid"},
	}
	out := buildOutbound(p)
	if out.Type != "vless" {
		t.Fatalf("expected vless, got %s", out.Type)
	}
	if out.TLS == nil || out.TLS.Reality == nil || out.TLS.Reality.PublicKey != "pk" {
		t.Fatalf("expected tls reality opts: %+v", out.TLS)
	}
}

func TestBuildOutboundTrojan(t *testing.T) {
	p := ClashProxy{
		Name:     "trojan-node",
		Type:     "trojan",
		Server:   "1.2.3.4",
		Port:     443,
		Password: "pw",
		TLS:      true,
	}
	out := buildOutbound(p)
	if out.Type != "trojan" || out.Password != "pw" {
		t.Fatalf("unexpected trojan outbound: %+v", out)
	}
}

func TestBuildOutboundHysteria2(t *testing.T) {
	p := ClashProxy{
		Name:         "hy2-node",
		Type:         "hysteria2",
		Server:       "1.2.3.4",
		Port:         443,
		Password:     "pw",
		Obfs:         "salamander",
		ObfsPassword: "op",
		SNI:          "example.com",
	}
	out := buildOutbound(p)
	if out.Type != "hysteria2" {
		t.Fatalf("expected hysteria2, got %s", out.Type)
	}
	if out.Obfs == nil || out.Obfs.Type != "salamander" || out.Obfs.Password != "op" {
		t.Fatalf("unexpected obfs: %+v", out.Obfs)
	}
	if out.TLS == nil {
		t.Fatalf("expected tls")
	}
}

func TestBuildOutboundWireGuard(t *testing.T) {
	p := ClashProxy{
		Name:       "wg-node",
		Type:       "wireguard",
		Server:     "1.2.3.4",
		Port:       51820,
		PrivateKey: "priv",
		PublicKey:  "pub",
		Reserved:   "1,2,3",
		MTU:        1420,
	}
	out := buildOutbound(p)
	if out.Type != "wireguard" {
		t.Fatalf("expected wireguard, got %s", out.Type)
	}
	if out.PrivateKey != "priv" || out.PublicKey != "pub" {
		t.Fatalf("unexpected keys: %+v", out)
	}
	if len(out.Reserved) != 3 || out.Reserved[0] != 1 {
		t.Fatalf("unexpected reserved: %+v", out.Reserved)
	}
}

func TestBuildOutboundUnknownTypeDropped(t *testing.T) {
	p := ClashProxy{
		Name:     "unknown-node",
		Type:     "unknown-protocol",
		Server:   "1.2.3.4",
		Port:     443,
		Password: "pw",
	}
	out := buildOutbound(p)
	if out.Type != "" {
		t.Fatalf("expected unknown type dropped, got %+v", out)
	}
}

func TestBuildRouteSourceRules(t *testing.T) {
	rules := []string{
		"DOMAIN,example.com,PROXY",
		"IP-CIDR,10.0.0.0/8,DIRECT",
		"SRC-IP-CIDR,192.168.1.0/24,DIRECT",
		"SRC-PORT,8080,DIRECT",
		"DST-PORT,443,PROXY",
		"MATCH,DIRECT",
	}
	route := buildRoute(rules, nil)
	if route.Final != "direct" {
		t.Fatalf("expected final direct, got %s", route.Final)
	}
	if len(route.Rules) != 7 { // sniff + hijack-dns + 4 rules + no MATCH
		t.Fatalf("expected 7 rules, got %d", len(route.Rules))
	}

	foundSrcIP := false
	foundSrcPort := false
	for _, r := range route.Rules {
		if len(r.SourceIPCIDR) > 0 {
			foundSrcIP = true
		}
		if len(r.SourcePort) > 0 {
			foundSrcPort = true
		}
	}
	if !foundSrcIP {
		t.Fatal("expected source_ip_cidr rule")
	}
	if !foundSrcPort {
		t.Fatal("expected source_port rule")
	}
}

func TestParsePortList(t *testing.T) {
	ports := parsePortList("80,443,1000-1002")
	if len(ports) != 5 {
		t.Fatalf("expected 5 ports, got %d: %v", len(ports), ports)
	}
	if ports[0] != 80 || ports[1] != 443 || ports[2] != 1000 {
		t.Fatalf("unexpected ports: %v", ports)
	}
}

func TestClashTypeToSingBox(t *testing.T) {
	cases := map[string]string{
		"ss":      "shadowsocks",
		"ssr":     "shadowsocks",
		"socks5":  "socks",
		"any-tls": "anytls",
		"wg":      "wireguard",
		"vmess":   "vmess",
	}
	for in, want := range cases {
		got := clashTypeToSingBox(in)
		if got != want {
			t.Errorf("clashTypeToSingBox(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestConvertClashWithMultipleProtocols(t *testing.T) {
	yaml := `
proxies:
  - name: ss-node
    type: ss
    server: 1.2.3.4
    port: 8388
    cipher: aes-256-gcm
    password: pw
  - name: vless-node
    type: vless
    server: 1.2.3.5
    port: 443
    uuid: uuid-1234
    tls: true
    servername: example.com
    reality-opts:
      public-key: pk
      short-id: sid
  - name: hy2-node
    type: hysteria2
    server: 1.2.3.6
    port: 443
    password: pw
    sni: example.com
    obfs: salamander
    obfs-password: op
proxy-groups:
  - name: 节点选择
    type: select
    proxies:
      - ss-node
      - vless-node
      - hy2-node
rules:
  - MATCH,DIRECT
`
	jsonData, err := ConvertClashToSingBox([]byte(yaml))
	if err != nil {
		t.Fatalf("convert: %v", err)
	}

	for _, want := range []string{`"type": "shadowsocks"`, `"type": "vless"`, `"type": "hysteria2"`, `"obfs": {`, `"type": "salamander"`, `"password": "op"`} {
		if !strings.Contains(jsonData, want) {
			t.Fatalf("expected %s in output", want)
		}
	}
	if !strings.Contains(jsonData, `"tag": "节点选择"`) {
		t.Fatal("expected selector group")
	}
	if !strings.Contains(jsonData, `"type": "selector"`) {
		t.Fatal("expected selector outbound")
	}
}
