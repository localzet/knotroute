package ops

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

type Store struct {
	mu   sync.RWMutex
	path string
	data State
}

func OpenStore(path string) (*Store, error) {
	s := &Store{path: path, data: State{Networks: map[string]Network{}, Agents: map[string]Agent{}, Jobs: map[string]Job{}}}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &s.data); err != nil {
		return nil, err
	}
	if s.data.Networks == nil {
		s.data.Networks = map[string]Network{}
	}
	if s.data.Agents == nil {
		s.data.Agents = map[string]Agent{}
	}
	if s.data.Jobs == nil {
		s.data.Jobs = map[string]Job{}
	}
	return s, nil
}

func (s *Store) View(fn func(State)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fn(cloneState(s.data))
}

func (s *Store) Update(fn func(*State) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := fn(&s.data); err != nil {
		return err
	}
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil && filepath.Dir(s.path) != "." {
		return err
	}
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".state-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, s.path)
}

func cloneState(in State) State {
	raw, _ := json.Marshal(in)
	var out State
	_ = json.Unmarshal(raw, &out)
	return out
}
