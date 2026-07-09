package platform

import (
	"testing"

	"zem/internal/settings"
)

func TestTunDNSAddress(t *testing.T) {
	dir := t.TempDir()
	sm := settings.NewManager(dir)
	pm := NewManager(sm, dir)

	if err := sm.SetTunSettings(settings.TunSettings{Address: []string{"172.19.0.1/30"}}); err != nil {
		t.Fatalf("set tun settings: %v", err)
	}
	if got := pm.tunDNSAddress(); got != "172.19.0.2" {
		t.Fatalf("expected 172.19.0.2, got %s", got)
	}

	if err := sm.SetTunSettings(settings.TunSettings{Address: []string{"172.19.0.2/30"}}); err != nil {
		t.Fatalf("set tun settings: %v", err)
	}
	if got := pm.tunDNSAddress(); got != "172.19.0.1" {
		t.Fatalf("expected 172.19.0.1, got %s", got)
	}

	if err := sm.SetTunSettings(settings.TunSettings{Address: []string{"10.0.0.5/30"}}); err != nil {
		t.Fatalf("set tun settings: %v", err)
	}
	if got := pm.tunDNSAddress(); got != "10.0.0.6" {
		t.Fatalf("expected 10.0.0.6, got %s", got)
	}
}
