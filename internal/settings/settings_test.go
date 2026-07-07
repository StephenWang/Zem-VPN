package settings

import (
	"testing"
)

func TestDefaultTunSettings(t *testing.T) {
	s := DefaultTunSettings()
	if len(s.Address) == 0 || s.Address[0] != "172.19.0.1/30" {
		t.Fatalf("unexpected default address: %v", s.Address)
	}
	if s.Stack != "mixed" {
		t.Fatalf("unexpected default stack: %s", s.Stack)
	}
	if s.MTU != 9000 {
		t.Fatalf("unexpected default mtu: %d", s.MTU)
	}
}

func TestSetTunSettings(t *testing.T) {
	m := NewManager(t.TempDir())
	tun := TunSettings{
		Address: []string{"10.0.0.1/24"},
		Stack:   "gvisor",
		MTU:     1500,
	}
	if err := m.SetTunSettings(tun); err != nil {
		t.Fatalf("set tun: %v", err)
	}
	got := m.GetTunSettings()
	if got.Stack != "gvisor" || got.MTU != 1500 {
		t.Fatalf("unexpected tun settings: %+v", got)
	}
}

func TestSetTunSettingsInvalid(t *testing.T) {
	m := NewManager(t.TempDir())
	if err := m.SetTunSettings(TunSettings{Stack: "invalid"}); err == nil {
		t.Fatal("expected error for invalid stack")
	}
	if err := m.SetTunSettings(TunSettings{Stack: "mixed", MTU: 1000}); err == nil {
		t.Fatal("expected error for invalid mtu")
	}
}

func TestServiceMode(t *testing.T) {
	m := NewManager(t.TempDir())
	if m.GetServiceMode() {
		t.Fatal("service mode should be disabled by default")
	}
	if err := m.SetServiceMode(true); err != nil {
		t.Fatalf("set service mode: %v", err)
	}
	if !m.GetServiceMode() {
		t.Fatal("service mode should be enabled")
	}
}

func TestServiceTokenPersists(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	token := m.GetServiceToken()
	if token == "" {
		t.Fatal("expected service token")
	}

	reloaded := NewManager(dir)
	if reloaded.GetServiceToken() != token {
		t.Fatal("service token should persist across manager reloads")
	}
}
