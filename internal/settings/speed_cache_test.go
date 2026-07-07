package settings

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSpeedCacheLoadSave(t *testing.T) {
	dir := t.TempDir()
	c := NewSpeedTestCache(dir)
	if err := c.Set("sub1", map[string]int64{"a": 100, "b": -1, "c": 200}); err != nil {
		t.Fatal(err)
	}
	_ = c.Flush()

	c2 := NewSpeedTestCache(dir)
	results := c2.Get("sub1")
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results["a"] != 100 || results["b"] != -1 || results["c"] != 200 {
		t.Fatalf("unexpected results: %v", results)
	}
}

func TestSpeedCacheBestNode(t *testing.T) {
	dir := t.TempDir()
	c := NewSpeedTestCache(dir)
	if err := c.Set("sub1", map[string]int64{"a": 100, "b": -1, "c": 50, "d": -1}); err != nil {
		t.Fatal(err)
	}
	if best := c.BestNode("sub1"); best != "c" {
		t.Fatalf("expected best c, got %s", best)
	}
	if best := c.BestNode("sub2"); best != "" {
		t.Fatalf("expected empty, got %s", best)
	}
}

func TestSpeedCacheClear(t *testing.T) {
	dir := t.TempDir()
	c := NewSpeedTestCache(dir)
	_ = c.Set("sub1", map[string]int64{"a": 100})
	_ = c.Clear("sub1")
	_ = c.Flush()
	if len(c.Get("sub1")) != 0 {
		t.Fatal("expected cleared")
	}
}

func TestSpeedCacheFileDoesNotExist(t *testing.T) {
	dir := t.TempDir()
	c := NewSpeedTestCache(dir)
	if len(c.Get("sub1")) != 0 {
		t.Fatal("expected empty cache")
	}
}

func TestSpeedCacheCorruptedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "speed_cache.json")
	_ = os.WriteFile(path, []byte("not json"), 0644)
	c := NewSpeedTestCache(dir)
	if c == nil {
		t.Fatal("expected cache even on corrupted file")
	}
}

func TestSpeedCacheTimeNow(t *testing.T) {
	old := timeNowFunc
	defer func() { timeNowFunc = old }()
	timeNowFunc = func() int64 { return 12345 }
	if timeNow() != 12345 {
		t.Fatal("timeNow stub failed")
	}
}

// ensure time is imported in non-test build too
var _ = time.Now
