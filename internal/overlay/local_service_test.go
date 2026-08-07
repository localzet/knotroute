package overlay

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/serviceid"
)

func startEcho(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) { defer c.Close(); _, _ = io.Copy(c, c) }(conn)
		}
	}()
	return listener
}

func assertEcho(t *testing.T, conn net.Conn, payload string) {
	t.Helper()
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(3 * time.Second))
	if _, err := conn.Write([]byte(payload)); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != payload {
		t.Fatalf("echo mismatch: %q", got)
	}
}

func TestLocalDirectServiceFastPath(t *testing.T) {
	echo := startEcho(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := config.Default()
	cfg.Services = []config.Service{{Name: "echo", Target: echo.Addr().String(), Allow: []string{"*"}}}
	node := startTestNode(t, ctx, cfg)
	defer node.Stop()

	conn, err := node.OpenCircuitStream(ctx, node.ID(), "echo")
	if err != nil {
		t.Fatal(err)
	}
	assertEcho(t, conn, "local direct service")
}

func TestLocalPublishedServiceFastPath(t *testing.T) {
	echo := startEcho(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := config.Default()
	cfg.Services = []config.Service{{Name: "site", Target: echo.Addr().String(), Allow: []string{"*"}, Publish: true}}
	node := startTestNode(t, ctx, cfg)
	defer node.Stop()

	var sid serviceid.ID
	node.directory.mu.RLock()
	for _, service := range node.directory.local {
		sid = service.identity.ID
	}
	node.directory.mu.RUnlock()
	if sid == (serviceid.ID{}) {
		t.Fatal("published service identity missing")
	}

	conn, err := node.OpenService(ctx, sid)
	if err != nil {
		t.Fatal(err)
	}
	assertEcho(t, conn, "local published service")
}
