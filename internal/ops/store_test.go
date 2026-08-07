package ops

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStorePersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Update(func(st *State) error {
		st.Networks["kn_test"] = Network{ID: "kn_test", Name: "Test", CreatedAt: time.Now()}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	reloaded, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	reloaded.View(func(st State) { _, found = st.Networks["kn_test"] })
	if !found {
		t.Fatal("network was not persisted")
	}
}
