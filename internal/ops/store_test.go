package ops

import (
	"os"
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

func TestStoreUpdateRollsBackWhenPersistenceFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Update(func(st *State) error {
		st.Networks["kn_before"] = Network{ID: "kn_before", Name: "Before"}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	blocker := filepath.Join(dir, "not-a-directory")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	s.path = filepath.Join(blocker, "state.json")
	if err := s.Update(func(st *State) error {
		st.Networks["kn_should_rollback"] = Network{ID: "kn_should_rollback", Name: "Nope"}
		return nil
	}); err == nil {
		t.Fatal("expected persistence failure")
	}

	s.View(func(st State) {
		if _, ok := st.Networks["kn_should_rollback"]; ok {
			t.Fatal("failed update leaked into in-memory state")
		}
		if _, ok := st.Networks["kn_before"]; !ok {
			t.Fatal("previous state was lost")
		}
	})
}
