package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// SpeedTestCache 保存每个订阅最近一次测速结果
type SpeedTestCache struct {
	mu            sync.RWMutex
	dataDir       string
	cache         map[string]map[string]int64 // subID -> nodeTag -> latency(ms), -1 表示超时
	lastUpdated   map[string]int64            // subID -> timestamp
	dirty         bool
	flushInterval time.Duration
	timer         *time.Timer
	closed        bool
}

// NewSpeedTestCache 创建测速缓存管理器
func NewSpeedTestCache(dataDir string) *SpeedTestCache {
	c := &SpeedTestCache{
		dataDir:       dataDir,
		cache:         make(map[string]map[string]int64),
		lastUpdated:   make(map[string]int64),
		flushInterval: 5 * time.Second,
	}
	_ = c.Load()
	return c
}

func (c *SpeedTestCache) filePath() string {
	return filepath.Join(c.dataDir, "speed_cache.json")
}

// Load 从磁盘加载测速缓存
func (c *SpeedTestCache) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.filePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var payload struct {
		Subscriptions map[string]struct {
			Results     map[string]int64 `json:"results"`
			LastUpdated int64            `json:"last_updated"`
		} `json:"subscriptions"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return fmt.Errorf("parse speed cache: %w", err)
	}

	for subID, entry := range payload.Subscriptions {
		c.cache[subID] = entry.Results
		c.lastUpdated[subID] = entry.LastUpdated
	}
	return nil
}

// Save 持久化测速缓存到磁盘
func (c *SpeedTestCache) Save() error {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.saveLocked()
}

// saveLocked 执行真正的磁盘写入，调用方必须已持有 c.mu（读锁或写锁）
func (c *SpeedTestCache) saveLocked() error {
	payload := struct {
		Subscriptions map[string]struct {
			Results     map[string]int64 `json:"results"`
			LastUpdated int64            `json:"last_updated"`
		} `json:"subscriptions"`
	}{
		Subscriptions: make(map[string]struct {
			Results     map[string]int64 `json:"results"`
			LastUpdated int64            `json:"last_updated"`
		}),
	}
	for subID, results := range c.cache {
		payload.Subscriptions[subID] = struct {
			Results     map[string]int64 `json:"results"`
			LastUpdated int64            `json:"last_updated"`
		}{
			Results:     results,
			LastUpdated: c.lastUpdated[subID],
		}
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(c.dataDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(c.filePath(), data, 0644)
}

// Set 保存单个订阅的测速结果，标记脏并延迟落盘
func (c *SpeedTestCache) Set(subID string, results map[string]int64) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("cache closed")
	}
	if c.cache[subID] == nil {
		c.cache[subID] = make(map[string]int64)
	}
	for tag, ms := range results {
		c.cache[subID][tag] = ms
	}
	c.lastUpdated[subID] = timeNow()
	c.dirty = true
	c.scheduleFlush()
	c.mu.Unlock()
	return nil
}

// Clear 清除单个订阅的测速缓存
func (c *SpeedTestCache) Clear(subID string) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return fmt.Errorf("cache closed")
	}
	delete(c.cache, subID)
	delete(c.lastUpdated, subID)
	c.dirty = true
	c.scheduleFlush()
	c.mu.Unlock()
	return nil
}

func (c *SpeedTestCache) scheduleFlush() {
	if c.timer != nil {
		return
	}
	c.timer = time.AfterFunc(c.flushInterval, func() {
		_ = c.Flush()
	})
}

// Flush 立即将脏缓存写入磁盘
func (c *SpeedTestCache) Flush() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	if !c.dirty {
		return nil
	}
	c.dirty = false
	return c.saveLocked()
}

// Close 关闭缓存，退出时 flush
func (c *SpeedTestCache) Close() error {
	c.mu.Lock()
	c.closed = true
	if c.timer != nil {
		c.timer.Stop()
		c.timer = nil
	}
	c.mu.Unlock()
	return c.Flush()
}

// Get 读取单个订阅的测速结果
func (c *SpeedTestCache) Get(subID string) map[string]int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]int64)
	for k, v := range c.cache[subID] {
		out[k] = v
	}
	return out
}

// BestNode 返回指定订阅中延迟最低且未超时的节点 tag；如果没有可用结果返回空字符串
func (c *SpeedTestCache) BestNode(subID string) string {
	c.mu.RLock()
	results := c.cache[subID]
	c.mu.RUnlock()
	if len(results) == 0 {
		return ""
	}

	type pair struct {
		tag string
		ms  int64
	}
	var pairs []pair
	for tag, ms := range results {
		if ms >= 0 {
			pairs = append(pairs, pair{tag: tag, ms: ms})
		}
	}
	if len(pairs) == 0 {
		return ""
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].ms < pairs[j].ms })
	return pairs[0].tag
}

func timeNow() int64 {
	return timeNowFunc()
}

var timeNowFunc = func() int64 {
	return time.Now().Unix()
}

// ensureImport prevents unused import errors when the file is built without tests.
var _ = timeNow
