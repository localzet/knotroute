package social

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

type Contact struct {
	Profile PublicIdentity `json:"profile"`
	Node    string         `json:"node"`
	Alias   string         `json:"alias,omitempty"`
}

type State struct {
	DisplayName string               `json:"display_name"`
	Bio         string               `json:"bio,omitempty"`
	AvatarHash  string               `json:"avatar_hash,omitempty"`
	Contacts    map[string]Contact   `json:"contacts"`
	Messages    map[string][]Message `json:"messages"`
	Posts       []Post               `json:"posts"`
}

type Store struct {
	path  string
	mu    sync.RWMutex
	state State
}

func OpenStore(path string) (*Store, error) {
	s := &Store{path: path, state: State{Contacts: map[string]Contact{}, Messages: map[string][]Message{}, Posts: []Post{}}}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &s.state); err != nil {
		return nil, err
	}
	if s.state.Contacts == nil {
		s.state.Contacts = map[string]Contact{}
	}
	if s.state.Messages == nil {
		s.state.Messages = map[string][]Message{}
	}
	if s.state.Posts == nil {
		s.state.Posts = []Post{}
	}
	return s, nil
}

func (s *Store) Snapshot() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	raw, _ := json.Marshal(s.state)
	var out State
	_ = json.Unmarshal(raw, &out)
	return out
}

func (s *Store) SetProfile(name, bio, avatar string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.DisplayName, s.state.Bio, s.state.AvatarHash = name, bio, avatar
	return s.saveLocked()
}

func (s *Store) PutContact(c Contact) error {
	if _, err := c.Profile.Verify(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state.Contacts[c.Profile.ID] = c
	return s.saveLocked()
}

func (s *Store) PutMessage(peerID string, m Message) error {
	if err := m.Verify(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.state.Messages[peerID] {
		if existing.ID == m.ID {
			return nil
		}
	}
	s.state.Messages[peerID] = append(s.state.Messages[peerID], m)
	sort.Slice(s.state.Messages[peerID], func(i, j int) bool {
		return s.state.Messages[peerID][i].CreatedUnix < s.state.Messages[peerID][j].CreatedUnix
	})
	if len(s.state.Messages[peerID]) > 5000 {
		s.state.Messages[peerID] = append([]Message(nil), s.state.Messages[peerID][len(s.state.Messages[peerID])-5000:]...)
	}
	return s.saveLocked()
}

func (s *Store) PutPost(p Post) error {
	if err := p.Verify(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.state.Posts {
		if existing.ID == p.ID {
			return nil
		}
	}
	s.state.Posts = append(s.state.Posts, p)
	sort.Slice(s.state.Posts, func(i, j int) bool { return s.state.Posts[i].CreatedUnix > s.state.Posts[j].CreatedUnix })
	if len(s.state.Posts) > 1000 {
		s.state.Posts = append([]Post(nil), s.state.Posts[:1000]...)
	}
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil && filepath.Dir(s.path) != "." {
		return err
	}
	raw, err := json.MarshalIndent(s.state, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".social-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	_ = tmp.Chmod(0o600)
	if _, err := tmp.Write(append(raw, '\n')); err != nil {
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
