package config

import (
	"fmt"
	"strings"
	"testing"
)

func TestConvertSSURLList(t *testing.T) {
	data := []byte(`c3M6Ly9ZV1Z6TFRJMU5pMW5ZMjA2T1RBNVkyRXpOelF0TlRBeFlTMDBZVE0wTFRoak1XVXRZVFZpWlROallXUmlaV0l5QDExMC4yNDIuNzQuMTAyOjEwMDAxIyVGMCU5RiU4NyVBRCVGMCU5RiU4NyVCMCUyMCVFOSVBNiU5OSVFNiVCOCVBRklFUEwNCnNzOi8vWVdWekxUSTFOaTFuWTIwNk9UQTVZMkV6TnpRdE5UQXhZUzAwWVRNMExUaGpNV1V0WVRWaVpUTmpZV1JpWldJeUAxMTAuMjQyLjc0LjEwMjoxMDAwMiMlRjAlOUYlODclQTglRjAlOUYlODclQjMlMjAlRTUlOEYlQjAlRTYlQjklQkVJRVBMDQpzczovL1lXVnpMVEkxTmkxblkyMDZPVEE1WTJFek56UXROVEF4WVMwMFlUTTBMVGhqTVdVdFlUVmlaVE5qWVdSaVpXSXlAMTEwLjI0Mi43NC4xMDI6MTAwMDMjJUYwJTlGJTg3JUFGJUYwJTlGJTg3JUI1JTIwJUU2JTk3JUE1JUU2JTlDJUFDSUVQTA0Kc3M6Ly9ZV1Z6TFRJMU5pMW5ZMjA2T1RBNVkyRXpOelF0TlRBeFlTMDBZVE0wTFRoak1XVXRZVFZpWlROallXUmlaV0l5QDExMC4yNDIuNzQuMTAyOjEwMDA0IyVGMCU5RiU4NyVCOCVGMCU5RiU4NyVBQyUyMCVFNiU5NiVCMCVFNSU4QSVBMCVFNSU5RCVBMUlFUEwNCnNzOi8vWVdWekxUSTFOaTFuWTIwNk9UQTVZMkV6TnpRdE5UQXhZUzAwWVRNMExUaGpNV1V0WVRWaVpUTmpZV1JpWldJeUAxMTAuMjQyLjc0LjEwMjoxMDAwNSMlRjAlOUYlODclQkElRjAlOUYlODclQjglMjAlRTclQkUlOEUlRTUlOUIlQkRJRVBM`)

	jsonData, err := ConvertSubscriptionData(data)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	if !strings.Contains(jsonData, `"type": "shadowsocks"`) {
		t.Fatalf("expected shadowsocks outbound in result")
	}
	if !strings.Contains(jsonData, `"tag": "selected"`) {
		t.Fatalf("expected selected selector in result")
	}
	if !strings.Contains(jsonData, `"tag": "自动选择"`) {
		t.Fatalf("expected 自动选择 urltest in result")
	}

	fmt.Println(jsonData)
}

func TestParseSSURL(t *testing.T) {
	cases := []string{
		"ss://YWVzLTI1Ni1nY206OTA5Y2EzNzQtNTAxYS00YTM0LThjMWUtYTViZTNjYWRiZWIy@110.242.74.102:10001#%F0%9F%87%AD%F0%9F%87%B0%20%E9%A6%99%E6%B8%AFIEPL",
		"ss://aes-256-gcm:password@1.2.3.4:8388#test",
	}
	for _, c := range cases {
		out, err := parseSSURL(c)
		if err != nil {
			t.Errorf("parse %q failed: %v", c, err)
			continue
		}
		if out.Type != "shadowsocks" || out.Server == "" || out.ServerPort == 0 || out.Method == "" || out.Password == "" {
			t.Errorf("parse %q got invalid outbound: %+v", c, out)
		}
	}
}
