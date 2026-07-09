package settings

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const defaultProxyPort = 7890

// TunSettings TUN 设备可配置参数
type TunSettings struct {
	Address                []string `json:"address"`
	Stack                  string   `json:"stack"`
	MTU                    int      `json:"mtu"`
	AutoRoute              bool     `json:"auto_route"`
	StrictRoute            bool     `json:"strict_route"`
	EndpointIndependentNAT bool     `json:"endpoint_independent_nat"`
	GSO                    bool     `json:"gso"`
}

// DefaultTunSettings 返回默认 TUN 配置（与原先硬编码值一致）
func DefaultTunSettings() TunSettings {
	return TunSettings{
		Address:                []string{"172.19.0.1/30"},
		Stack:                  "mixed",
		MTU:                    9000,
		AutoRoute:              true,
		StrictRoute:            true,
		EndpointIndependentNAT: true,
		GSO:                    false,
	}
}

func (t *TunSettings) Normalize() {
	if len(t.Address) == 0 {
		t.Address = DefaultTunSettings().Address
	}
	if t.Stack == "" {
		t.Stack = "mixed"
	}
	if t.MTU <= 0 || t.MTU > 65535 {
		t.MTU = 9000
	}
}

type Settings struct {
	ProxyPort             int               `json:"proxy_port"`
	CurrentSubID          string            `json:"current_sub_id,omitempty"`
	Tun                   TunSettings       `json:"tun"`
	ServiceMode           bool              `json:"service_mode"`
	ServicePort           int               `json:"service_port"`
	ServiceToken          string            `json:"service_token,omitempty"`
	ProxyMode             string            `json:"proxy_mode"`
	SelectedNodes         map[string]string `json:"selected_nodes,omitempty"`
	AutoReconnectOnUpdate *bool             `json:"auto_reconnect_on_update,omitempty"`
}

type Manager struct {
	dataDir   string
	settings  Settings
	mu        sync.RWMutex
	saveMu    sync.Mutex
	saveTimer *time.Timer
	saveDelay time.Duration
	closed    bool
}

func NewManager(dataDir string) *Manager {
	m := &Manager{
		dataDir: dataDir,
		settings: Settings{
			ProxyPort:   defaultProxyPort,
			Tun:         DefaultTunSettings(),
			ServicePort: 17519,
			ProxyMode:   "rule",
		},
		saveDelay: 100 * time.Millisecond,
	}
	_ = m.Load()
	m.settings.Tun.Normalize()
	if m.settings.ProxyMode == "" {
		m.settings.ProxyMode = "rule"
	}
	if m.settings.ServiceToken == "" {
		m.settings.ServiceToken = generateServiceToken()
		_ = m.Save()
	}
	return m
}

func (m *Manager) filePath() string {
	return filepath.Join(m.dataDir, "settings.json")
}

func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("parse settings: %w", err)
	}
	if s.ProxyPort <= 0 || s.ProxyPort > 65535 {
		s.ProxyPort = defaultProxyPort
	}
	if s.ServicePort <= 0 || s.ServicePort > 65535 {
		s.ServicePort = 17519
	}
	if s.ProxyMode == "" {
		s.ProxyMode = "rule"
	}
	if s.ServiceToken == "" {
		s.ServiceToken = generateServiceToken()
	}
	s.Tun.Normalize()
	m.settings = s
	if m.settings.SelectedNodes == nil {
		m.settings.SelectedNodes = make(map[string]string)
	}
	return nil
}

func (m *Manager) Save() error {
	m.saveMu.Lock()
	if m.saveTimer != nil {
		m.saveTimer.Stop()
		m.saveTimer = nil
	}
	m.saveMu.Unlock()

	m.mu.RLock()
	defer m.mu.RUnlock()

	data, err := json.MarshalIndent(m.settings, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(m.dataDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(m.filePath(), data, 0644)
}

func (m *Manager) scheduleSave() error {
	m.saveMu.Lock()
	if m.closed {
		m.saveMu.Unlock()
		return fmt.Errorf("settings manager closed")
	}
	if m.saveTimer != nil {
		m.saveTimer.Stop()
	}
	m.saveTimer = time.AfterFunc(m.saveDelay, func() {
		_ = m.Flush()
	})
	m.saveMu.Unlock()
	return nil
}

func (m *Manager) Flush() error {
	m.saveMu.Lock()
	if m.saveTimer != nil {
		m.saveTimer.Stop()
		m.saveTimer = nil
	}
	m.saveMu.Unlock()
	return m.Save()
}

func (m *Manager) Close() error {
	m.saveMu.Lock()
	m.closed = true
	if m.saveTimer != nil {
		m.saveTimer.Stop()
		m.saveTimer = nil
	}
	m.saveMu.Unlock()
	return m.Flush()
}

func (m *Manager) GetProxyPort() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.ProxyPort
}

func (m *Manager) SetProxyPort(port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}
	m.mu.Lock()
	m.settings.ProxyPort = port
	m.mu.Unlock()
	return m.scheduleSave()
}

func (m *Manager) GetCurrentSubID() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.CurrentSubID
}

func (m *Manager) SetCurrentSubID(id string) error {
	m.mu.Lock()
	m.settings.CurrentSubID = id
	m.mu.Unlock()
	return m.scheduleSave()
}

func (m *Manager) GetTunSettings() TunSettings {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s := m.settings.Tun
	s.Normalize()
	return s
}

func (m *Manager) SetTunSettings(tun TunSettings) error {
	tun.Normalize()
	validStacks := map[string]bool{"gvisor": true, "system": true, "mixed": true}
	if !validStacks[tun.Stack] {
		return fmt.Errorf("invalid tun stack: %s", tun.Stack)
	}
	if len(tun.Address) == 0 {
		return fmt.Errorf("tun address cannot be empty")
	}
	for _, addr := range tun.Address {
		if addr == "" {
			return fmt.Errorf("tun address cannot be empty")
		}
	}
	if tun.MTU < 1280 || tun.MTU > 65535 {
		return fmt.Errorf("invalid mtu: %d", tun.MTU)
	}

	m.mu.Lock()
	m.settings.Tun = tun
	m.mu.Unlock()
	return m.scheduleSave()
}

func (m *Manager) GetServiceMode() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.ServiceMode
}

func (m *Manager) SetServiceMode(enabled bool) error {
	m.mu.Lock()
	m.settings.ServiceMode = enabled
	m.mu.Unlock()
	return m.scheduleSave()
}

func (m *Manager) GetServicePort() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.settings.ServicePort <= 0 {
		return 17519
	}
	return m.settings.ServicePort
}

func (m *Manager) GetServiceToken() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.ServiceToken
}

func (m *Manager) SetServicePort(port int) error {
	if port <= 0 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}
	m.mu.Lock()
	m.settings.ServicePort = port
	m.mu.Unlock()
	return m.scheduleSave()
}

func (m *Manager) GetProxyMode() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.settings.ProxyMode == "" {
		return "rule"
	}
	return m.settings.ProxyMode
}

func (m *Manager) SetProxyMode(mode string) error {
	valid := map[string]bool{"direct": true, "rule": true, "system": true}
	if !valid[mode] {
		return fmt.Errorf("invalid proxy mode: %s", mode)
	}
	m.mu.Lock()
	m.settings.ProxyMode = mode
	m.mu.Unlock()
	return m.scheduleSave()
}

func (m *Manager) GetSelectedNode(subID string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.settings.SelectedNodes[subID]
}

func (m *Manager) SetSelectedNode(subID, nodeTag string) error {
	m.mu.Lock()
	if m.settings.SelectedNodes == nil {
		m.settings.SelectedNodes = make(map[string]string)
	}
	m.settings.SelectedNodes[subID] = nodeTag
	m.mu.Unlock()
	return m.scheduleSave()
}

func (m *Manager) GetAutoReconnectOnUpdate() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.settings.AutoReconnectOnUpdate == nil {
		return true
	}
	return *m.settings.AutoReconnectOnUpdate
}

func (m *Manager) SetAutoReconnectOnUpdate(enabled bool) error {
	m.mu.Lock()
	m.settings.AutoReconnectOnUpdate = &enabled
	m.mu.Unlock()
	return m.scheduleSave()
}

func generateServiceToken() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "zem-local-service-token"
	}
	return hex.EncodeToString(b[:])
}
