package overlay

import (
	"path/filepath"
	"testing"

	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/identity"
)

func TestDescriptorRevisionPersistsAcrossNodeRestart(t *testing.T) {
	identityPath := filepath.Join(t.TempDir(), "services", "web.identity.json")
	cfg := config.Default()
	cfg.Dashboard = ""
	cfg.Proxy.HTTP = ""
	cfg.Proxy.SOCKS = ""
	cfg.Discovery.Enabled = false
	cfg.Services = []config.Service{{Name: "web", Target: "127.0.0.1:1", Publish: true, IdentityFile: identityPath}}

	newNode := func() *Node {
		id, err := identity.Generate()
		if err != nil {
			t.Fatal(err)
		}
		n, err := New(cfg, id)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	first := newNode()
	pub1 := first.directory.local["web"]
	if pub1 == nil {
		t.Fatal("published service missing")
	}
	base := pub1.revision.Load()
	if base < 1<<32 {
		t.Fatalf("migration revision seed is unexpectedly small: %d", base)
	}
	r1, err := nextDescriptorRevision(pub1)
	if err != nil {
		t.Fatal(err)
	}
	r2, err := nextDescriptorRevision(pub1)
	if err != nil {
		t.Fatal(err)
	}
	if r2 <= r1 {
		t.Fatalf("revision did not increase: %d <= %d", r2, r1)
	}

	second := newNode()
	pub2 := second.directory.local["web"]
	if got := pub2.revision.Load(); got != r2 {
		t.Fatalf("revision was not restored across restart: got %d want %d", got, r2)
	}
	r3, err := nextDescriptorRevision(pub2)
	if err != nil {
		t.Fatal(err)
	}
	if r3 <= r2 {
		t.Fatalf("revision regressed after restart: %d <= %d", r3, r2)
	}
}
