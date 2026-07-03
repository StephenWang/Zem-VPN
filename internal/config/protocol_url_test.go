package config

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestParseVmessURL(t *testing.T) {
	json := `{"v":"2","ps":"test","add":"1.2.3.4","port":"443","id":"uuid-1234","aid":"0","net":"ws","path":"/path","host":"example.com","tls":"tls"}`
	encoded := base64Encode(json)
	out, err := ParseProtocolURL("vmess://" + encoded)
	if err != nil {
		t.Fatalf("parse vmess: %v", err)
	}
	if out.Type != "vmess" || out.Server != "1.2.3.4" || out.ServerPort != 443 {
		t.Fatalf("unexpected vmess: %+v", out)
	}
	if out.Transport == nil || out.Transport.Type != "ws" {
		t.Fatalf("expected ws transport: %+v", out.Transport)
	}
	if out.TLS == nil {
		t.Fatal("expected tls")
	}
}

func TestParseTrojanURL(t *testing.T) {
	out, err := ParseProtocolURL("trojan://password@1.2.3.4:443?sni=example.com#test")
	if err != nil {
		t.Fatalf("parse trojan: %v", err)
	}
	if out.Type != "trojan" || out.Password != "password" || out.Server != "1.2.3.4" {
		t.Fatalf("unexpected trojan: %+v", out)
	}
}

func TestParseSSRURL(t *testing.T) {
	// server:port:protocol:method:obfs:password_base64/?remarks=...&protoparam=...&obfsparam=...
	pass := base64Encode("pass")
	remarks := base64Encode("ssr-node")
	line := "ssr://" + base64Encode("1.2.3.4:8388:auth_aes128_md5:aes-256-cfb:http_simple:"+pass+"/?remarks="+remarks)
	out, err := ParseProtocolURL(line)
	if err != nil {
		t.Fatalf("parse ssr: %v", err)
	}
	if out.Type != "shadowsocks" || out.Method != "aes-256-cfb" {
		t.Fatalf("unexpected ssr: %+v", out)
	}
}

func TestConvertProtocolURLList(t *testing.T) {
	data := []byte(`ss://YWVzLTI1Ni1nY206cGFzcw==@1.2.3.4:8388#ss-node
trojan://password@5.6.7.8:443?sni=example.com#trojan-node`)
	jsonData, err := ConvertProtocolURLListToSingBox(data)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if !strings.Contains(jsonData, `"type": "shadowsocks"`) {
		t.Fatal("expected shadowsocks")
	}
	if !strings.Contains(jsonData, `"type": "trojan"`) {
		t.Fatal("expected trojan")
	}
	if !strings.Contains(jsonData, `"tag": "selected"`) {
		t.Fatal("expected selected selector")
	}
}

func TestNormalizeSingBoxJSON(t *testing.T) {
	input := `{"outbounds":[{"type":"shadowsocks","tag":"ss","server":"1.2.3.4","server_port":8388,"method":"aes-256-gcm","password":"pw"}]}`
	jsonData, err := normalizeSingBoxJSON([]byte(input))
	if err != nil {
		t.Fatalf("normalize: %v", err)
	}
	if !strings.Contains(jsonData, `"type": "direct"`) {
		t.Fatal("expected direct outbound")
	}
	if !strings.Contains(jsonData, `"type": "tun"`) {
		t.Fatal("expected tun inbound")
	}
}

func TestMergeSubscriptions(t *testing.T) {
	sub1 := `{"outbounds":[{"type":"shadowsocks","tag":"node1","server":"1.2.3.4","server_port":8388,"method":"aes-256-gcm","password":"pw"}]}`
	sub2 := `{"outbounds":[{"type":"trojan","tag":"node1","server":"5.6.7.8","server_port":443,"password":"pw"}]}`

	merged, err := MergeSubscriptions([]string{sub1, sub2}, MergeModeUnion)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if !strings.Contains(merged, `"tag": "node1"`) {
		t.Fatal("expected node1")
	}
	if !strings.Contains(merged, `"tag": "node1_2"`) {
		t.Fatal("expected deduplicated node1_2")
	}
	if !strings.Contains(merged, `"tag": "selected"`) {
		t.Fatal("expected selected selector")
	}
}

func TestMergeSubscriptionsSelectMode(t *testing.T) {
	sub1 := `{"outbounds":[{"type":"shadowsocks","tag":"n1","server":"1.2.3.4","server_port":8388,"method":"aes-256-gcm","password":"pw"}]}`
	sub2 := `{"outbounds":[{"type":"trojan","tag":"n2","server":"5.6.7.8","server_port":443,"password":"pw"}]}`

	merged, err := MergeSubscriptions([]string{sub1, sub2}, MergeModeSelect)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if !strings.Contains(merged, `"tag": "sub-1"`) {
		t.Fatal("expected sub-1 selector")
	}
}

func base64Encode(s string) string {
	return base64.StdEncoding.EncodeToString([]byte(s))
}
