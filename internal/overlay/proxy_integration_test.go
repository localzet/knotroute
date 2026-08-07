package overlay

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/identity"
)

func TestHTTPProxyResolvesKnotDomainAcrossRelay(t *testing.T) {
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "host=%s path=%s", r.Host, r.URL.Path)
	})}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go server.Serve(listener)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := startTestNode(t, ctx, config.Default())
	defer b.Stop()
	cfgC := config.Default()
	cfgC.Peers = []config.Peer{{Address: b.Addresses()[0]}}
	cfgC.Services = []config.Service{{Name: "http", Target: listener.Addr().String(), Allow: []string{"*"}}}
	c := startTestNode(t, ctx, cfgC)
	defer c.Stop()
	cfgA := config.Default()
	cfgA.Peers = []config.Peer{{Address: b.Addresses()[0]}}
	cfgA.Proxy.HTTP = "127.0.0.1:0"
	cfgA.Proxy.SOCKS = ""
	a := startTestNodeWithProxy(t, ctx, cfgA)
	defer a.Stop()
	logStatusesOnFailure(t, a, b, c)

	waitRoute(t, a, c, 2)
	var rawProxy string
	for _, address := range a.Status().Proxy.Listeners {
		if strings.HasPrefix(address, "http://") {
			rawProxy = address
		}
	}
	if rawProxy == "" {
		t.Fatal("HTTP proxy listener missing")
	}
	proxyURL, err := url.Parse(rawProxy)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: 8 * time.Second}
	response, err := client.Get("http://" + c.Domain() + "/through-overlay")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.StatusCode, body)
	}
	if !strings.Contains(string(body), "path=/through-overlay") {
		t.Fatalf("unexpected body: %s", body)
	}
}

func logStatusesOnFailure(t *testing.T, nodes ...*Node) {
	t.Helper()
	t.Cleanup(func() {
		if !t.Failed() {
			return
		}
		for i, node := range nodes {
			t.Logf("node[%d] status: %+v", i, node.Status())
		}
	})
}

func startTestNodeWithProxy(t *testing.T, ctx context.Context, cfg config.Config) *Node {
	t.Helper()
	cfg.Listen = []string{"127.0.0.1:0"}
	cfg.Dashboard = ""
	cfg.Routing = config.Routing{LSAInterval: "200ms", LSATTL: "3s", MaxHops: 16}
	cfg.Discovery.Enabled = false
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

func waitRoute(t *testing.T, from, to *Node, hops int) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for {
		if route, ok := from.RouteTo(to.ID()); ok && route.Hops == hops {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("route not learned: %+v", from.Status())
		}
		time.Sleep(40 * time.Millisecond)
	}
}

func TestSOCKS5ResolvesServiceKnotDomainAcrossRelay(t *testing.T) {
	echo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	go func() {
		for {
			conn, err := echo.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) { defer c.Close(); _, _ = io.Copy(c, c) }(conn)
		}
	}()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := startTestNode(t, ctx, config.Default())
	defer b.Stop()
	cfgC := config.Default()
	cfgC.Peers = []config.Peer{{Address: b.Addresses()[0]}}
	cfgC.Services = []config.Service{{Name: "echo", Target: echo.Addr().String(), Allow: []string{"*"}}}
	c := startTestNode(t, ctx, cfgC)
	defer c.Stop()
	cfgA := config.Default()
	cfgA.Peers = []config.Peer{{Address: b.Addresses()[0]}}
	cfgA.Proxy.SOCKS = "127.0.0.1:0"
	cfgA.Proxy.HTTP = ""
	a := startTestNodeWithProxy(t, ctx, cfgA)
	defer a.Stop()
	logStatusesOnFailure(t, a, b, c)
	waitRoute(t, a, c, 2)
	var address string
	for _, item := range a.Status().Proxy.Listeners {
		if strings.HasPrefix(item, "socks5://") {
			address = strings.TrimPrefix(item, "socks5://")
		}
	}
	if address == "" {
		t.Fatal("SOCKS listener missing")
	}
	conn, err := net.DialTimeout("tcp", address, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(8 * time.Second))
	if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
		t.Fatal(err)
	}
	method := make([]byte, 2)
	if _, err := io.ReadFull(conn, method); err != nil {
		t.Fatal(err)
	}
	if method[0] != 5 || method[1] != 0 {
		t.Fatalf("bad method reply: %v", method)
	}
	host := "echo." + c.Domain()
	request := []byte{5, 1, 0, 3, byte(len(host))}
	request = append(request, host...)
	request = append(request, 0x12, 0x34)
	if _, err := conn.Write(request); err != nil {
		t.Fatal(err)
	}
	reply := make([]byte, 10)
	if _, err := io.ReadFull(conn, reply); err != nil {
		t.Fatal(err)
	}
	if reply[1] != 0 {
		t.Fatalf("SOCKS connect failed: %v", reply)
	}
	payload := []byte("hello through SOCKS and .knot")
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
}

func TestHTTPProxyOpensLocalPublishedService(t *testing.T) {
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "local=%s", r.URL.Path)
	})}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go server.Serve(listener)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := config.Default()
	cfg.Proxy.HTTP = "127.0.0.1:0"
	cfg.Proxy.SOCKS = ""
	cfg.Services = []config.Service{{Name: "http", Target: listener.Addr().String(), Allow: []string{"*"}, Publish: true}}
	node := startTestNodeWithProxy(t, ctx, cfg)
	defer node.Stop()

	var serviceDomain string
	for _, service := range node.Status().Services {
		if service.Name == "http" && service.Published {
			serviceDomain = service.Domain
			break
		}
	}
	if serviceDomain == "" {
		t.Fatal("published service domain missing")
	}
	var rawProxy string
	for _, address := range node.Status().Proxy.Listeners {
		if strings.HasPrefix(address, "http://") {
			rawProxy = address
			break
		}
	}
	proxyURL, err := url.Parse(rawProxy)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}, Timeout: 5 * time.Second}
	response, err := client.Get("http://" + serviceDomain + "/self-check")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK || !strings.Contains(string(body), "local=/self-check") {
		t.Fatalf("status=%d body=%q", response.StatusCode, body)
	}
}
