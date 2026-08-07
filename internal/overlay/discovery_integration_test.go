package overlay

import (
	"context"
	"net"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/discovery"
	"github.com/localzet/knotroute/internal/identity"
	"github.com/localzet/knotroute/internal/networkid"
)

func TestOverlayAutoPeersThroughBeacon(t *testing.T) {
	beacon := httptest.NewServer(discovery.NewServer(time.Minute, 100).Handler())
	defer beacon.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	network := networkid.FromSeed("overlay-beacon-integration").String()

	start := func(name string) *Node {
		t.Helper()
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		address := listener.Addr().String()
		_ = listener.Close()

		cfg := config.Default()
		cfg.NetworkID = network
		cfg.Listen = []string{address}
		cfg.Advertise = []string{address}
		cfg.Dashboard = ""
		cfg.Proxy.HTTP = ""
		cfg.Proxy.SOCKS = ""
		cfg.Discovery.Enabled = true
		cfg.Discovery.LAN = false
		cfg.Discovery.PeerExchange = true
		cfg.Discovery.Beacons = []string{beacon.URL}
		cfg.Discovery.Interval = "5s"
		cfg.Discovery.CacheFile = filepath.Join(t.TempDir(), name+"-peers.json")
		cfg.Routing = config.Routing{LSAInterval: "200ms", LSATTL: "3s", MaxHops: 16}
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

	a := start("a")
	defer a.Stop()
	b := start("b")
	defer b.Stop()

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		aStatus, bStatus := a.Status(), b.Status()
		_, aRoute := a.RouteTo(b.ID())
		_, bRoute := b.RouteTo(a.ID())
		if len(aStatus.Peers) == 1 && len(bStatus.Peers) == 1 && aRoute && bRoute {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("nodes did not auto-peer through Beacon: A=%+v B=%+v", a.Status(), b.Status())
}
