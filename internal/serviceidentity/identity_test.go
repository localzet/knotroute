package serviceidentity

import (
	"path/filepath"
	"testing"
)

func TestPersist(t *testing.T) {
	p := filepath.Join(t.TempDir(), "s.json")
	a, err := LoadOrCreate(p)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != b.ID {
		t.Fatal("identity changed")
	}
}
