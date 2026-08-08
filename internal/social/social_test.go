package social

import (
	"path/filepath"
	"testing"
	"time"
)

func TestIdentityMessageAndPost(t *testing.T) {
	id, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	profile, err := id.Public("Alice", "hello", "", time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := profile.Verify(); err != nil {
		t.Fatal(err)
	}
	m, err := NewMessage(id, profile, "kr_node", "ku_recipient", "hello world", "", time.Unix(101, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := m.Verify(); err != nil {
		t.Fatal(err)
	}
	p, err := NewPost(id, profile, "kr_node", "first post", []string{"Test", "#test", "knot"}, time.Unix(102, 0))
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Verify(); err != nil {
		t.Fatal(err)
	}
	if len(p.Tags) != 2 {
		t.Fatalf("tags=%v", p.Tags)
	}
}

func TestStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	id, _ := Generate()
	profile, _ := id.Public("Alice", "", "", time.Now())
	m, _ := NewMessage(id, profile, "kr_a", "ku_b", "hello", "", time.Now())
	s, err := OpenStore(filepath.Join(dir, "social.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := s.PutMessage("ku_b", m); err != nil {
		t.Fatal(err)
	}
	loaded, err := OpenStore(filepath.Join(dir, "social.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(loaded.Snapshot().Messages["ku_b"]); got != 1 {
		t.Fatalf("messages=%d", got)
	}
}
