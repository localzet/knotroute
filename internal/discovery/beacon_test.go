package discovery

import (
	"context"
	"github.com/localzet/knotroute/internal/identity"
	"github.com/localzet/knotroute/internal/networkid"
	"net/http/httptest"
	"testing"
	"time"
)

func TestBeaconExchange(t *testing.T) {
	srv := httptest.NewServer(NewServer(time.Minute, 100).Handler())
	defer srv.Close()
	a, _ := identity.Generate()
	b, _ := identity.Generate()
	c := Client{}
	netid := networkid.FromSeed("test")
	if peers, err := c.Exchange(context.Background(), srv.URL, a, netid, []string{"127.0.0.1:7001"}); err != nil || len(peers) != 0 {
		t.Fatalf("first: %v %#v", err, peers)
	}
	peers, err := c.Exchange(context.Background(), srv.URL, b, netid, []string{"127.0.0.1:7002"})
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].NodeID != a.ID.String() {
		t.Fatalf("unexpected peers %#v", peers)
	}
}

func TestBeaconClientOnlyCanDiscoverWithoutBeingAdvertised(t *testing.T) {
	srv := httptest.NewServer(NewServer(time.Minute, 100).Handler())
	defer srv.Close()
	serverNode, _ := identity.Generate()
	clientNode, _ := identity.Generate()
	c := Client{}
	netid := networkid.FromSeed("client-only")
	if _, err := c.Exchange(context.Background(), srv.URL, serverNode, netid, []string{"198.51.100.10:7447"}); err != nil {
		t.Fatal(err)
	}
	peers, err := c.Exchange(context.Background(), srv.URL, clientNode, netid, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].NodeID != serverNode.ID.String() {
		t.Fatalf("unexpected peers %#v", peers)
	}
	peers, err = c.Exchange(context.Background(), srv.URL, serverNode, netid, []string{"198.51.100.10:7447"})
	if err != nil {
		t.Fatal(err)
	}
	for _, peer := range peers {
		if peer.NodeID == clientNode.ID.String() {
			t.Fatalf("client-only node leaked into candidates: %#v", peers)
		}
	}
}

func TestBeaconBootstrapCandidateUsesRequestHost(t *testing.T) {
	server := NewServer(time.Minute, 100)
	netid := networkid.FromSeed("bootstrap")
	hub, _ := identity.Generate()
	if err := server.SetBootstrap(netid, hub.ID, nil, "7447"); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(server.Handler())
	defer srv.Close()
	clientNode, _ := identity.Generate()
	client := Client{}
	peers, err := client.Exchange(context.Background(), srv.URL, clientNode, netid, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(peers) != 1 || peers[0].NodeID != hub.ID.String() || len(peers[0].Endpoints) != 1 {
		t.Fatalf("unexpected bootstrap candidate %#v", peers)
	}
}
