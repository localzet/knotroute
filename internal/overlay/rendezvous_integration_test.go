package overlay

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/directory"
	"github.com/localzet/knotroute/internal/nodeid"
	"github.com/localzet/knotroute/internal/serviceid"
)

func TestPublishedServiceThroughDirectoryIntroductionAndRendezvous(t *testing.T) {
	echo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	go func() {
		for {
			c, err := echo.Accept()
			if err != nil {
				return
			}
			go func(x net.Conn) { defer x.Close(); _, _ = io.Copy(x, x) }(c)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	base := func() config.Config {
		c := config.Default()
		c.Discovery.Enabled = false
		c.Directory = config.Directory{Replicas: 5, DescriptorTTL: "15s", PublishInterval: "300ms", LookupTimeout: "4s"}
		return c
	}
	hub := startTestNode(t, ctx, base())
	defer hub.Stop()
	// Two additional relays ensure introduction and rendezvous can be distinct.
	relayCfg := base()
	relayCfg.Peers = []config.Peer{{Address: hub.Addresses()[0]}}
	r1 := startTestNode(t, ctx, relayCfg)
	defer r1.Stop()
	r2 := startTestNode(t, ctx, relayCfg)
	defer r2.Stop()
	svcCfg := base()
	svcCfg.Peers = []config.Peer{{Address: hub.Addresses()[0]}}
	svcCfg.Services = []config.Service{{Name: "site", Target: echo.Addr().String(), Allow: []string{"*"}, Publish: true, IntroCount: 2}}
	serviceNode := startTestNode(t, ctx, svcCfg)
	defer serviceNode.Stop()
	clientCfg := base()
	clientCfg.Peers = []config.Peer{{Address: hub.Addresses()[0]}}
	client := startTestNode(t, ctx, clientCfg)
	defer client.Stop()
	logStatusesOnFailure(t, client, serviceNode, hub, r1, r2)

	var sid serviceid.ID
	deadline := time.Now().Add(10 * time.Second)
	for {
		serviceNode.directory.mu.RLock()
		for _, p := range serviceNode.directory.local {
			sid = p.identity.ID
		}
		serviceNode.directory.mu.RUnlock()
		if sid != (serviceid.ID{}) {
			if d, err := client.LookupService(context.Background(), sid); err == nil && len(d.IntroductionPoints) > 0 {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("service descriptor never became available; service=%+v client=%+v r1=%+v r2=%+v", serviceNode.Status(), client.Status(), r1.Status(), r2.Status())
		}
		time.Sleep(100 * time.Millisecond)
	}
	openCtx, openCancel := context.WithTimeout(ctx, 8*time.Second)
	defer openCancel()
	conn, err := client.OpenService(openCtx, sid)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	payload := []byte("hello through service identity, introduction and rendezvous")
	if _, err := conn.Write(payload); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("mismatch %q", got)
	}
}

func TestOpenServiceRefreshesStaleDescriptor(t *testing.T) {
	echo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	go func() {
		for {
			c, err := echo.Accept()
			if err != nil {
				return
			}
			go func(x net.Conn) { defer x.Close(); _, _ = io.Copy(x, x) }(c)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	base := func() config.Config {
		c := config.Default()
		c.Discovery.Enabled = false
		c.Directory = config.Directory{Replicas: 5, DescriptorTTL: "30s", PublishInterval: "300ms", LookupTimeout: "2s"}
		return c
	}
	hub := startTestNode(t, ctx, base())
	defer hub.Stop()
	relayCfg := base()
	relayCfg.Peers = []config.Peer{{Address: hub.Addresses()[0]}}
	r1 := startTestNode(t, ctx, relayCfg)
	defer r1.Stop()
	r2 := startTestNode(t, ctx, relayCfg)
	defer r2.Stop()
	svcCfg := base()
	svcCfg.Peers = []config.Peer{{Address: hub.Addresses()[0]}}
	svcCfg.Services = []config.Service{{Name: "site", Target: echo.Addr().String(), Allow: []string{"*"}, Publish: true, IntroCount: 2}}
	serviceNode := startTestNode(t, ctx, svcCfg)
	defer serviceNode.Stop()
	clientCfg := base()
	clientCfg.Peers = []config.Peer{{Address: hub.Addresses()[0]}}
	client := startTestNode(t, ctx, clientCfg)
	defer client.Stop()

	var sid serviceid.ID
	var pub *publishedService
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		serviceNode.directory.mu.RLock()
		for _, p := range serviceNode.directory.local {
			sid = p.identity.ID
			pub = p
		}
		serviceNode.directory.mu.RUnlock()
		if pub != nil {
			if d, err := client.LookupService(ctx, sid); err == nil && len(d.IntroductionPoints) > 0 {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if pub == nil || sid == (serviceid.ID{}) {
		t.Fatal("published service identity unavailable")
	}

	active := map[nodeid.ID]bool{}
	for _, id := range serviceNode.activeIntros(pub) {
		active[id] = true
	}
	var badIntro nodeid.ID
	for _, candidate := range []nodeid.ID{hub.ID(), r1.ID(), r2.ID()} {
		if !active[candidate] {
			badIntro = candidate
			break
		}
	}
	if badIntro == (nodeid.ID{}) {
		t.Skip("all candidate relays are active introduction points")
	}
	stale, err := directory.New(pub.identity, serviceNode.network, []nodeid.ID{badIntro}, pub.revision.Load()+1000, 30*time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	client.directory.mu.Lock()
	client.directory.descriptors[sid] = stale
	client.directory.mu.Unlock()

	openCtx, openCancel := context.WithTimeout(ctx, 8*time.Second)
	defer openCancel()
	conn, err := client.OpenService(openCtx, sid)
	if err != nil {
		t.Fatalf("OpenService did not recover from stale descriptor: %v", err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	payload := []byte("fresh descriptor after stale introduction")
	if _, err := conn.Write(payload); err != nil {
		t.Fatal(err)
	}
	got := make([]byte, len(payload))
	if _, err := io.ReadFull(conn, got); err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("mismatch %q", got)
	}
}

func TestPublishedServiceRepublishesImmediatelyAfterTopologyAppears(t *testing.T) {
	echo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	go func() {
		for {
			c, err := echo.Accept()
			if err != nil {
				return
			}
			go func(x net.Conn) { defer x.Close(); _, _ = io.Copy(x, x) }(c)
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	base := func() config.Config {
		c := config.Default()
		c.Discovery.Enabled = false
		c.Directory = config.Directory{Replicas: 5, DescriptorTTL: "30s", PublishInterval: "1h", LookupTimeout: "2s"}
		return c
	}
	hub := startTestNode(t, ctx, base())
	defer hub.Stop()
	relayCfg := base()
	relayCfg.Peers = []config.Peer{{Address: hub.Addresses()[0]}}
	r1 := startTestNode(t, ctx, relayCfg)
	defer r1.Stop()
	r2 := startTestNode(t, ctx, relayCfg)
	defer r2.Stop()

	svcCfg := base()
	svcCfg.Peers = []config.Peer{{Address: hub.Addresses()[0]}}
	svcCfg.Services = []config.Service{{Name: "site", Target: echo.Addr().String(), Allow: []string{"*"}, Publish: true, IntroCount: 1}}
	serviceNode := startTestNode(t, ctx, svcCfg)
	defer serviceNode.Stop()

	clientCfg := base()
	clientCfg.Peers = []config.Peer{{Address: hub.Addresses()[0]}}
	client := startTestNode(t, ctx, clientCfg)
	defer client.Stop()

	var sid serviceid.ID
	serviceNode.directory.mu.RLock()
	for _, p := range serviceNode.directory.local {
		sid = p.identity.ID
	}
	serviceNode.directory.mu.RUnlock()
	if sid == (serviceid.ID{}) {
		t.Fatal("service identity unavailable")
	}

	deadline := time.Now().Add(6 * time.Second)
	for time.Now().Before(deadline) {
		lookupCtx, lookupCancel := context.WithTimeout(ctx, 600*time.Millisecond)
		d, lookupErr := client.lookupService(lookupCtx, sid, false)
		lookupCancel()
		if lookupErr == nil && len(d.IntroductionPoints) > 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("descriptor was not published after topology became available; service=%+v client=%+v", serviceNode.Status(), client.Status())
}
