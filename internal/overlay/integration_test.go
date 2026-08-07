package overlay

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/identity"
)

func TestThreeNodeEncryptedMultiHopStream(t *testing.T) {
	echo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	echoDone := make(chan struct{})
	go func() {
		defer close(echoDone)
		for {
			c, err := echo.Accept()
			if err != nil {
				return
			}
			go func(conn net.Conn) { defer conn.Close(); _, _ = io.Copy(conn, conn) }(c)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := startTestNode(t, ctx, config.Default())
	cfgC := config.Default()
	cfgC.Listen = []string{"127.0.0.1:0"}
	cfgC.Dashboard = ""
	cfgC.Peers = []config.Peer{{Address: b.Addresses()[0]}}
	cfgC.Services = []config.Service{{Name: "echo", Target: echo.Addr().String(), Allow: []string{"*"}}}
	cfgC.Routing = config.Routing{LSAInterval: "200ms", LSATTL: "3s", MaxHops: 16}
	c := startTestNode(t, ctx, cfgC)
	cfgA := config.Default()
	cfgA.Listen = []string{"127.0.0.1:0"}
	cfgA.Dashboard = ""
	cfgA.Peers = []config.Peer{{Address: b.Addresses()[0]}}
	cfgA.Routing = config.Routing{LSAInterval: "200ms", LSATTL: "3s", MaxHops: 16}
	a := startTestNode(t, ctx, cfgA)
	defer a.Stop()
	defer c.Stop()
	defer b.Stop()

	deadline := time.Now().Add(8 * time.Second)
	for {
		if route, ok := a.RouteTo(c.ID()); ok && route.Hops == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("A never learned a two-hop route to C; status=%+v", a.Status())
		}
		time.Sleep(50 * time.Millisecond)
	}

	openCtx, openCancel := context.WithTimeout(ctx, 5*time.Second)
	defer openCancel()
	conn, err := a.OpenStream(openCtx, c.ID(), "echo")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	payload := []byte("hello through node B — end-to-end encrypted")
	if _, err := conn.Write(payload); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("echo mismatch: %q", got)
	}
	if b.Status().BytesReceived == 0 || b.Status().BytesSent == 0 {
		t.Fatal("relay node did not forward traffic")
	}
}

func startTestNode(t *testing.T, ctx context.Context, cfg config.Config) *Node {
	t.Helper()
	cfg.Listen = []string{"127.0.0.1:0"}
	cfg.Dashboard = ""
	cfg.Proxy.SOCKS = ""
	cfg.Proxy.HTTP = ""
	cfg.Discovery.Enabled = false
	if cfg.Routing.LSAInterval == "20s" {
		cfg.Routing = config.Routing{LSAInterval: "200ms", LSATTL: "3s", MaxHops: 16}
	}
	id, err := identity.Generate()
	if err != nil {
		t.Fatal(err)
	}
	node, err := New(cfg, id)
	if err != nil {
		t.Fatal(err)
	}
	if err := node.Start(ctx); err != nil {
		t.Fatal(err)
	}
	return node
}
