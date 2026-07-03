package profile

import (
	"testing"
)

func TestManagerCreateAndList(t *testing.T) {
	m := NewManager(t.TempDir())
	p, err := m.Create("test-profile", []string{"sub1", "sub2"}, "union")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.ID == "" {
		t.Fatal("profile id should not be empty")
	}

	list := m.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 profile, got %d", len(list))
	}
	if list[0].Name != "test-profile" {
		t.Fatalf("unexpected name: %s", list[0].Name)
	}
}

func TestManagerPersistence(t *testing.T) {
	dir := t.TempDir()
	m1 := NewManager(dir)
	if _, err := m1.Create("persist", []string{"a", "b"}, "select"); err != nil {
		t.Fatalf("create: %v", err)
	}

	m2 := NewManager(dir)
	list := m2.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 profile after reload, got %d", len(list))
	}
	if list[0].MergeMode != "select" {
		t.Fatalf("unexpected merge mode: %s", list[0].MergeMode)
	}
}

func TestManagerUpdateAndDelete(t *testing.T) {
	m := NewManager(t.TempDir())
	p, err := m.Create("test", []string{"sub1"}, "union")
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := m.Update(p.ID, "updated", []string{"sub1", "sub2"}, "select"); err != nil {
		t.Fatalf("update: %v", err)
	}

	updated := m.Get(p.ID)
	if updated.Name != "updated" || updated.MergeMode != "select" {
		t.Fatalf("unexpected update: %+v", updated)
	}

	if err := m.Delete(p.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if m.Get(p.ID) != nil {
		t.Fatal("profile should be deleted")
	}
}
