package knotmailbox

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type FileStore struct {
	Directory       string
	TTL             time.Duration
	MaxPerRecipient int
	mu              sync.Mutex
}

func (s *FileStore) defaults() (time.Duration, int) {
	ttl := s.TTL
	if ttl <= 0 {
		ttl = 14 * 24 * time.Hour
	}
	max := s.MaxPerRecipient
	if max <= 0 {
		max = 1000
	}
	return ttl, max
}
func (s *FileStore) Put(envelope Envelope) error {
	id, err := ParseID(envelope.RecipientID)
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.Directory, id.String())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	ttl, max := s.defaults()
	entries, _ := os.ReadDir(dir)
	live := 0
	cutoff := time.Now().Add(-ttl).Unix()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		var existing Envelope
		if raw, err := os.ReadFile(path); err == nil && json.Unmarshal(raw, &existing) == nil && existing.CreatedUnix < cutoff {
			_ = os.Remove(path)
			continue
		}
		live++
	}
	path := filepath.Join(dir, envelope.MessageID+".json")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	if live >= max {
		return errors.New("mailbox recipient capacity reached")
	}
	raw, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".message-*")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer os.Remove(name)
	_ = tmp.Chmod(0o600)
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}
func (s *FileStore) List(id ID, limit int) ([]Envelope, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.Directory, id.String())
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	ttl, _ := s.defaults()
	cutoff := time.Now().Add(-ttl).Unix()
	out := make([]Envelope, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var envelope Envelope
		if json.Unmarshal(raw, &envelope) != nil {
			continue
		}
		if envelope.CreatedUnix < cutoff {
			_ = os.Remove(path)
			continue
		}
		out = append(out, envelope)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedUnix < out[j].CreatedUnix })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
func (s *FileStore) Delete(id ID, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	dir := filepath.Join(s.Directory, id.String())
	for _, messageID := range ids {
		if messageID == "" {
			continue
		}
		// Message IDs are base64url SHA-256 digests and cannot contain path separators.
		for _, r := range messageID {
			if !(r >= 'A' && r <= 'Z') && !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9') && r != '-' && r != '_' {
				return errors.New("invalid message id")
			}
		}
		if err := os.Remove(filepath.Join(dir, messageID+".json")); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}
