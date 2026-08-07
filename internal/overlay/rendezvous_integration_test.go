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
