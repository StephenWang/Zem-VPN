package subscription

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func newTestManager(t *testing.T) *Manager {
	dir := t.TempDir()
	return NewManager(dir)
}

func TestManagerLoadAllAndList(t *testing.T) {
	m := newTestManager(t)

	// 构造一个虚拟订阅文件
	sub := &Subscription{
		ID:          "test-id",
		URL:         "https://example.com/sub.yaml",
		Name:        "test",
		LastUpdate:  time.Now(),
		SingBoxJSON: `{"outbounds":[]}`,
	}
	if err := m.Save(sub); err != nil {
		t.Fatalf("save: %v", err)
	}

	m2 := NewManager(m.dataDir)
	if err := m2.LoadAll(); err != nil {
		t.Fatalf("load all: %v", err)
	}

	list := m2.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 sub, got %d", len(list))
	}
	if list[0].ID != "test-id" {
		t.Fatalf("unexpected id: %s", list[0].ID)
	}
	if list[0].SingBoxJSON == "" {
		t.Fatal("expected sing box json loaded")
	}
}

func TestManagerConcurrentAccess(t *testing.T) {
	m := newTestManager(t)
	sub := &Subscription{
		ID:          "concurrent-id",
		URL:         "https://example.com/sub.yaml",
		Name:        "test",
		LastUpdate:  time.Now(),
		SingBoxJSON: `{"outbounds":[]}`,
	}
	if err := m.Save(sub); err != nil {
		t.Fatalf("save: %v", err)
	}
	m.Replace(sub)

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = m.Get("concurrent-id")
			_ = m.List()
		}()
	}
	wg.Wait()
}

func TestManagerDelete(t *testing.T) {
	m := newTestManager(t)
	sub := &Subscription{
		ID:          "delete-id",
		URL:         "https://example.com/sub.yaml",
		Name:        "test",
		LastUpdate:  time.Now(),
		SingBoxJSON: `{"outbounds":[]}`,
	}
	if err := m.Save(sub); err != nil {
		t.Fatalf("save: %v", err)
	}
	m.Replace(sub)

	if err := m.Delete("delete-id"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if m.Get("delete-id") != nil {
		t.Fatal("expected sub deleted")
	}

	subDir := filepath.Join(m.dataDir, SubDir)
	if _, err := os.Stat(filepath.Join(subDir, "delete-id.json")); !os.IsNotExist(err) {
		t.Fatal("expected meta file removed")
	}
}

func TestManagerGetAndListReturnCopies(t *testing.T) {
	m := newTestManager(t)
	sub := &Subscription{
		ID:          "copy-id",
		URL:         "https://example.com/sub.yaml",
		Name:        "test",
		LastUpdate:  time.Now(),
		SingBoxJSON: `{"outbounds":[]}`,
		Options: SubscriptionOptions{
			Headers: map[string]string{"X-Test": "one"},
		},
	}
	m.Replace(sub)

	got := m.Get("copy-id")
	got.Name = "mutated"
	got.Options.Headers["X-Test"] = "two"

	again := m.Get("copy-id")
	if again.Name != "test" {
		t.Fatalf("Get returned internal pointer, name mutated to %q", again.Name)
	}
	if again.Options.Headers["X-Test"] != "one" {
		t.Fatalf("Get returned shared headers map: %+v", again.Options.Headers)
	}

	list := m.List()
	list[0].Name = "list-mutated"
	if m.Get("copy-id").Name != "test" {
		t.Fatal("List returned internal pointer")
	}
}

func TestValidateSubscriptionURL(t *testing.T) {
	if err := validateSubscriptionURL("https://example.com/sub.yaml"); err != nil {
		t.Fatalf("valid url rejected: %v", err)
	}
	for _, raw := range []string{"file:///tmp/sub.yaml", "ftp://example.com/sub", "https:///missing-host"} {
		if err := validateSubscriptionURL(raw); err == nil {
			t.Fatalf("expected invalid url rejected: %s", raw)
		}
	}
}

func TestGenID(t *testing.T) {
	id1 := genID("https://example.com/1")
	id2 := genID("https://example.com/1")
	id3 := genID("https://example.com/2")
	if id1 != id2 {
		t.Fatal("same url should generate same id")
	}
	if id1 == id3 {
		t.Fatal("different url should generate different id")
	}
	if len(id1) != 8 {
		t.Fatalf("expected 8 char id, got %d", len(id1))
	}
}
