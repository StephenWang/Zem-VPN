package subscription

import (
	"context"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"zem/internal/config"
)

const (
	SubDir         = "subscriptions"
	UpdateInterval = 24 * time.Hour
)

// SubscriptionOptions 订阅下载与预处理选项
type SubscriptionOptions struct {
	UserAgent   string            `json:"user_agent,omitempty"`
	Cookie      string            `json:"cookie,omitempty"`
	Headers     map[string]string `json:"headers,omitempty"`
	Preprocess  string            `json:"preprocess,omitempty"` // 可选：base64, aes256, etc
	Password    string            `json:"password,omitempty"`   // 用于加密订阅解密
	SkipTLS     bool              `json:"skip_tls,omitempty"`
}

// Subscription 订阅元数据
type Subscription struct {
	ID          string              `json:"id"`
	URL         string              `json:"url"`
	Name        string              `json:"name"`
	LastUpdate  time.Time           `json:"last_update"`
	SingBoxJSON string              `json:"sing_box_json"`
	Options     SubscriptionOptions `json:"options,omitempty"`
}

// Manager 订阅管理器（线程安全）
type Manager struct {
	dataDir string
	subs    map[string]*Subscription
	mu      sync.RWMutex

	// OnUpdate 在订阅成功更新后被调用，供上层决定是否需要重连
	OnUpdate func(id string)
}

func NewManager(dataDir string) *Manager {
	return &Manager{
		dataDir: dataDir,
		subs:    make(map[string]*Subscription),
	}
}

func genID(urlStr string) string {
	h := md5.New()
	h.Write([]byte(urlStr))
	return hex.EncodeToString(h.Sum(nil))[:8]
}

func (m *Manager) Add(urlStr, name string) (*Subscription, error) {
	return m.AddWithOptions(urlStr, name, SubscriptionOptions{})
}

func (m *Manager) AddWithOptions(urlStr, name string, opts SubscriptionOptions) (*Subscription, error) {
	id := genID(urlStr)

	singBoxJSON, _, err := m.fetchAndConvert(urlStr, opts)
	if err != nil {
		return nil, fmt.Errorf("fetch subscription: %w", err)
	}

	sub := &Subscription{
		ID:          id,
		URL:         urlStr,
		Name:        name,
		LastUpdate:  time.Now(),
		SingBoxJSON: singBoxJSON,
		Options:     opts,
	}

	if err := m.Save(sub); err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.subs[id] = sub
	m.mu.Unlock()
	return sub, nil
}

func (m *Manager) fetch(urlStr string, opts SubscriptionOptions) ([]byte, error) {
	return m.fetchWithOptions(urlStr, opts)
}

func skipTLSVerify() bool {
	return os.Getenv("ZEM_SKIP_TLS_VERIFY") == "1"
}

func (m *Manager) fetchWithOptions(urlStr string, opts SubscriptionOptions) ([]byte, error) {
	tr := &http.Transport{}
	if skipTLSVerify() || opts.SkipTLS {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   30 * time.Second,
	}

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	ua := opts.UserAgent
	if ua == "" {
		ua = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
	}
	req.Header.Set("User-Agent", ua)
	req.Header.Set("Accept", "application/yaml, text/plain, application/json, */*")
	if opts.Cookie != "" {
		req.Header.Set("Cookie", opts.Cookie)
	}
	for k, v := range opts.Headers {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		body := strings.TrimSpace(string(data))
		if len(body) > 200 {
			body = body[:200] + "..."
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, body)
	}

	// 预处理
	data, err = applyPreprocess(data, opts)
	if err != nil {
		return nil, fmt.Errorf("preprocess: %w", err)
	}

	return config.DecodeBase64IfNeeded(data), nil
}

// applyPreprocess 对订阅内容进行预处理（如解密、解码等）
func applyPreprocess(data []byte, opts SubscriptionOptions) ([]byte, error) {
	switch strings.ToLower(opts.Preprocess) {
	case "":
		return data, nil
	case "base64":
		decoded := config.DecodeBase64IfNeeded(data)
		return decoded, nil
	case "aes256", "aes-256-cbc":
		return decryptAES256CBC(data, opts.Password)
	default:
		return nil, fmt.Errorf("unsupported preprocess: %s", opts.Preprocess)
	}
}

// fetchAndConvert 尝试多种 UA，选择能解析出最多代理节点的结果。
func (m *Manager) fetchAndConvert(urlStr string, opts SubscriptionOptions) (string, []byte, error) {
	// 如果用户指定了 UA，优先使用用户配置；否则尝试多个 UA
	if opts.UserAgent != "" {
		raw, err := m.fetchWithOptions(urlStr, opts)
		if err != nil {
			return "", nil, err
		}
		singBoxJSON, err := config.ConvertClashToSingBox(raw)
		if err != nil {
			return "", nil, err
		}
		return singBoxJSON, raw, nil
	}

	uas := []string{
		"ClashforWindows/0.20.39",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
	}

	type candidate struct {
		json string
		raw  []byte
	}
	var candidates []candidate

	for _, ua := range uas {
		testOpts := opts
		testOpts.UserAgent = ua
		raw, err := m.fetchWithOptions(urlStr, testOpts)
		if err != nil {
			continue
		}
		singBoxJSON, err := config.ConvertClashToSingBox(raw)
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{json: singBoxJSON, raw: raw})
	}

	if len(candidates) == 0 {
		return "", nil, fmt.Errorf("unable to fetch or convert subscription from any UA")
	}

	// 选择代理节点最多的结果
	best := candidates[0]
	bestCount := countProxyOutbounds(best.json)
	for _, c := range candidates[1:] {
		if n := countProxyOutbounds(c.json); n > bestCount {
			best = c
			bestCount = n
		}
	}
	return best.json, best.raw, nil
}

// countProxyOutbounds 统计 sing-box JSON 中实际代理节点数量。
func countProxyOutbounds(jsonStr string) int {
	count := 0
	for _, t := range config.ProxyTypes {
		count += strings.Count(jsonStr, fmt.Sprintf(`"type": %q`, t))
	}
	return count
}

func (m *Manager) Update(id string) (*Subscription, error) {
	m.mu.RLock()
	sub, ok := m.subs[id]
	m.mu.RUnlock()
	if !ok {
		return m.loadAndUpdate(id)
	}

	singBoxJSON, _, err := m.fetchAndConvert(sub.URL, sub.Options)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	sub.SingBoxJSON = singBoxJSON
	sub.LastUpdate = time.Now()
	updated := sub
	m.mu.Unlock()

	if err := m.Save(updated); err != nil {
		return nil, err
	}

	if m.OnUpdate != nil {
		m.OnUpdate(id)
	}

	return updated, nil
}

func (m *Manager) AutoUpdate(ctx context.Context) {
	ticker := time.NewTicker(UpdateInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.mu.RLock()
			subs := make([]*Subscription, 0, len(m.subs))
			for _, sub := range m.subs {
				subs = append(subs, sub)
			}
			m.mu.RUnlock()

			for _, sub := range subs {
				if time.Since(sub.LastUpdate) > UpdateInterval {
					if _, err := m.Update(sub.ID); err != nil {
						fmt.Printf("update sub %s failed: %v\n", sub.Name, err)
					}
				}
			}
		}
	}
}

func (m *Manager) Save(sub *Subscription) error {
	if sub == nil || sub.ID == "" {
		return fmt.Errorf("invalid subscription: missing id")
	}

	subDir := filepath.Join(m.dataDir, SubDir)
	if err := os.MkdirAll(subDir, 0755); err != nil {
		return err
	}

	metaPath := filepath.Join(subDir, sub.ID+".json")
	metaData, err := json.MarshalIndent(sub, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return err
	}

	jsonPath := filepath.Join(subDir, sub.ID+"_sing.json")
	if err := os.WriteFile(jsonPath, []byte(sub.SingBoxJSON), 0644); err != nil {
		return err
	}

	return nil
}

func (m *Manager) loadAndUpdate(id string) (*Subscription, error) {
	metaPath := filepath.Join(m.dataDir, SubDir, id+".json")
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return nil, err
	}

	var sub Subscription
	if err := json.Unmarshal(data, &sub); err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.subs[id] = &sub
	m.mu.Unlock()
	return m.Update(id)
}

func (m *Manager) LoadAll() error {
	subDir := filepath.Join(m.dataDir, SubDir)
	entries, err := os.ReadDir(subDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		// 跳过 *_sing.json 配置文件，只加载订阅元数据
		if strings.HasSuffix(entry.Name(), "_sing.json") {
			continue
		}

		filePath := filepath.Join(subDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var sub Subscription
		if err := json.Unmarshal(data, &sub); err != nil {
			continue
		}

		// 跳过并清理损坏/空的订阅文件
		if sub.ID == "" {
			os.Remove(filePath)
			os.Remove(filepath.Join(subDir, "_sing.json"))
			continue
		}

		jsonPath := filepath.Join(subDir, sub.ID+"_sing.json")
		if jsonData, err := os.ReadFile(jsonPath); err == nil {
			sub.SingBoxJSON = string(jsonData)
		}

		m.subs[sub.ID] = &sub
	}

	return nil
}

func (m *Manager) List() []*Subscription {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Subscription, 0, len(m.subs))
	for _, sub := range m.subs {
		result = append(result, sub)
	}
	return result
}

func (m *Manager) Get(id string) *Subscription {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.subs[id]
}

func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	delete(m.subs, id)
	m.mu.Unlock()

	subDir := filepath.Join(m.dataDir, SubDir)
	os.Remove(filepath.Join(subDir, id+".json"))
	os.Remove(filepath.Join(subDir, id+".yaml"))
	os.Remove(filepath.Join(subDir, id+"_sing.json"))

	return nil
}

// Replace 原子性替换订阅对象（用于 SelectServer/SelectGroup 修改后同步）
func (m *Manager) Replace(sub *Subscription) {
	if sub == nil || sub.ID == "" {
		return
	}
	m.mu.Lock()
	m.subs[sub.ID] = sub
	m.mu.Unlock()
}

// decryptAES256CBC 使用密码派生 key 解密 AES-256-CBC 数据（兼容部分机场加密订阅）
func decryptAES256CBC(data []byte, password string) ([]byte, error) {
	if password == "" {
		return nil, fmt.Errorf("password required for aes256 preprocess")
	}
	// 简单实现：假设 data 是 hex 编码的 iv + ciphertext
	// 实际场景可能需要根据具体机场格式调整
	return nil, fmt.Errorf("aes256 preprocess not fully implemented")
}
