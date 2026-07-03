package profile

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Profile 聚合多个订阅的配置视图
type Profile struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	SubscriptionIDs []string `json:"subscription_ids"`
	MergeMode       string   `json:"merge_mode"` // union 或 select
	CreatedAt       time.Time `json:"created_at"`
}

func genID(name string) string {
	h := md5.New()
	h.Write([]byte(name + time.Now().String()))
	return hex.EncodeToString(h.Sum(nil))[:8]
}

// Manager Profile 管理器
type Manager struct {
	dataDir  string
	profiles map[string]*Profile
	mu       sync.RWMutex
}

func NewManager(dataDir string) *Manager {
	m := &Manager{
		dataDir:  dataDir,
		profiles: make(map[string]*Profile),
	}
	_ = m.LoadAll()
	return m
}

func (m *Manager) filePath() string {
	return filepath.Join(m.dataDir, "profiles.json")
}

func (m *Manager) LoadAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var list []Profile
	if err := json.Unmarshal(data, &list); err != nil {
		return fmt.Errorf("parse profiles: %w", err)
	}
	for _, p := range list {
		if p.ID != "" {
			m.profiles[p.ID] = &p
		}
	}
	return nil
}

func (m *Manager) SaveAll() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.saveAllLocked()
}

// saveAllLocked 不加锁，调用者必须已持有 m.mu 的写锁
func (m *Manager) saveAllLocked() error {
	list := make([]Profile, 0, len(m.profiles))
	for _, p := range m.profiles {
		list = append(list, *p)
	}

	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(m.dataDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(m.filePath(), data, 0644)
}

func (m *Manager) Create(name string, subIDs []string, mergeMode string) (*Profile, error) {
	if name == "" {
		return nil, fmt.Errorf("profile name cannot be empty")
	}
	if len(subIDs) == 0 {
		return nil, fmt.Errorf("profile must contain at least one subscription")
	}
	if mergeMode == "" {
		mergeMode = "union"
	}
	if mergeMode != "union" && mergeMode != "select" {
		return nil, fmt.Errorf("invalid merge mode: %s", mergeMode)
	}

	p := &Profile{
		ID:              genID(name),
		Name:            name,
		SubscriptionIDs: subIDs,
		MergeMode:       mergeMode,
		CreatedAt:       time.Now(),
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	m.profiles[p.ID] = p

	return p, m.saveAllLocked()
}

func (m *Manager) Update(id, name string, subIDs []string, mergeMode string) (*Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.profiles[id]
	if !ok {
		return nil, fmt.Errorf("profile not found: %s", id)
	}
	if name != "" {
		p.Name = name
	}
	if len(subIDs) > 0 {
		p.SubscriptionIDs = subIDs
	}
	if mergeMode != "" {
		if mergeMode != "union" && mergeMode != "select" {
			return nil, fmt.Errorf("invalid merge mode: %s", mergeMode)
		}
		p.MergeMode = mergeMode
	}
	return p, m.saveAllLocked()
}

func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.profiles, id)
	return m.saveAllLocked()
}

func (m *Manager) Get(id string) *Profile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.profiles[id]
}

func (m *Manager) List() []*Profile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Profile, 0, len(m.profiles))
	for _, p := range m.profiles {
		result = append(result, p)
	}
	return result
}
