package discovery

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Cache struct {
	Peers []Candidate `json:"peers"`
}

func LoadCache(path string, maxAge time.Duration) []Candidate {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var c Cache
	if json.Unmarshal(raw, &c) != nil {
		return nil
	}
	cutoff := time.Now().Add(-maxAge).Unix()
	out := c.Peers[:0]
	for _, p := range c.Peers {
		if p.SeenUnix >= cutoff {
			out = append(out, p)
		}
	}
	return out
}
func SaveCache(path string, peers []Candidate) error {
	if path == "" {
		return nil
	}
	sort.Slice(peers, func(i, j int) bool { return peers[i].SeenUnix > peers[j].SeenUnix })
	if len(peers) > 512 {
		peers = peers[:512]
	}
	raw, err := json.MarshalIndent(Cache{Peers: peers}, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
