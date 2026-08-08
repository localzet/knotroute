package clientruntime

import (
	"context"
	"testing"
	"time"

	"github.com/localzet/knotroute/internal/config"
)

func TestMessengerAcrossTwoRuntimes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	a, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	b, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	configureTestRuntime(t, a)
	configureTestRuntime(t, b)
	if err := a.SetUserProfile("Alice", "first node"); err != nil {
		t.Fatal(err)
	}
	if err := b.SetUserProfile("Bob", "second node"); err != nil {
		t.Fatal(err)
	}
	if err := a.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer a.Stop()
	ast, ok := a.Status()
	if !ok || len(ast.Listen) == 0 {
		t.Fatal("first runtime did not expose a listener")
	}
	bcfg := b.Config()
	bcfg.Peers = []config.Peer{{Address: ast.Listen[0], ExpectedID: ast.NodeID}}
	if err := b.SaveConfig(bcfg); err != nil {
		t.Fatal(err)
	}
	if err := b.Start(ctx); err != nil {
		t.Fatal(err)
	}
	defer b.Stop()
	waitFor(t, 5*time.Second, func() bool { st, ok := b.Status(); return ok && len(st.Routes) > 0 })
	contact, err := b.AddContact(ctx, ast.NodeID, "Alice")
	if err != nil {
		t.Fatal(err)
	}
	if contact.Profile.ID != a.UserID() {
		t.Fatalf("contact user mismatch: got %s want %s", contact.Profile.ID, a.UserID())
	}
	if _, err := b.SendMessage(ctx, a.UserID(), "hello over KnotRoute"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, 3*time.Second, func() bool { return len(a.SocialState().Messages[b.UserID()]) == 1 })
	msg := a.SocialState().Messages[b.UserID()][0]
	if msg.Body != "hello over KnotRoute" || msg.Sender.ID != b.UserID() {
		t.Fatalf("unexpected message: %+v", msg)
	}
	if _, err := a.CreatePost("v4 is alive", []string{"knotroute", "v4"}); err != nil {
		t.Fatal(err)
	}
	posts, err := b.FetchContactFeed(ctx, a.UserID())
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 1 || posts[0].Text != "v4 is alive" {
		t.Fatalf("unexpected feed: %+v", posts)
	}
}

func configureTestRuntime(t *testing.T, r *Runtime) {
	t.Helper()
	cfg := r.Config()
	cfg.Listen = []string{"127.0.0.1:0"}
	cfg.Dashboard = ""
	cfg.Proxy.HTTP = ""
	cfg.Proxy.SOCKS = ""
	cfg.Discovery.Enabled = false
	cfg.Discovery.LAN = false
	cfg.Discovery.PeerExchange = false
	cfg.Privacy.CircuitHops = 1
	if err := r.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
}

func waitFor(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("condition not reached before timeout")
}
