package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/localzet/knotroute/internal/certauth"
	"github.com/localzet/knotroute/internal/naming"
	"github.com/localzet/knotroute/internal/nodeid"
	"github.com/localzet/knotroute/internal/serviceid"
)

func TestHTTPSServiceIdentityTerminatesWithLocalCA(t *testing.T) {
	authority, err := certauth.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	service := serviceid.FromPublicKey([]byte("service-test-key"))
	host := naming.ServiceCanonicalDomain(service)
	gateway := &Gateway{
		Direct: false, Authority: authority, InterceptHTTPS: true,
		DialService: func(ctx context.Context, got serviceid.ID) (net.Conn, error) {
			if got != service {
				return nil, fmt.Errorf("unexpected service %s", got.String())
			}
			client, server := net.Pipe()
			go func() {
				defer server.Close()
				req, err := http.ReadRequest(bufio.NewReader(server))
				if err != nil {
					return
				}
				_ = req.Body.Close()
				_, _ = server.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 4\r\nConnection: close\r\n\r\nknot"))
			}()
			return client, nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addresses, err := gateway.Start(ctx, "", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close()
	proxyAddr := strings.TrimPrefix(addresses[0], "http://")
	conn, err := net.DialTimeout("tcp", proxyAddr, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := fmt.Fprintf(conn, "CONNECT %s:443 HTTP/1.1\r\nHost: %s:443\r\n\r\n", host, host); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT status %d", resp.StatusCode)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(authority.RootPEM()) {
		t.Fatal("append root")
	}
	tlsConn := tls.Client(&bufferedConn{Conn: conn, reader: reader}, &tls.Config{ServerName: host, RootCAs: roots, MinVersion: tls.VersionTLS12})
	if err := tlsConn.Handshake(); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(tlsConn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", host); err != nil {
		t.Fatal(err)
	}
	httpResp, err := http.ReadResponse(bufio.NewReader(tlsConn), &http.Request{Method: http.MethodGet})
	if err != nil {
		t.Fatal(err)
	}
	defer httpResp.Body.Close()
	buf := make([]byte, 4)
	if _, err := io.ReadFull(httpResp.Body, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "knot" {
		t.Fatalf("body %q", buf)
	}
}

type bufferedConn struct {
	net.Conn
	reader *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.reader.Read(p) }

func TestHTTPSNodeServiceTerminatesWithLocalCA(t *testing.T) {
	authority, err := certauth.LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	node := nodeid.FromPublicKey([]byte("node-service-test-key"))
	host, err := naming.ServiceDomain("web", node)
	if err != nil {
		t.Fatal(err)
	}
	gateway := &Gateway{
		Direct: false, Authority: authority, InterceptHTTPS: true,
		DialOverlay: func(ctx context.Context, got nodeid.ID, service string) (net.Conn, error) {
			if got != node {
				return nil, fmt.Errorf("unexpected node %s", got.String())
			}
			if service != "web" {
				return nil, fmt.Errorf("unexpected service %q", service)
			}
			client, server := net.Pipe()
			go func() {
				defer server.Close()
				req, err := http.ReadRequest(bufio.NewReader(server))
				if err != nil {
					return
				}
				_ = req.Body.Close()
				_, _ = server.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 4\r\nConnection: close\r\n\r\nknot"))
			}()
			return client, nil
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addresses, err := gateway.Start(ctx, "", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close()
	proxyAddr := strings.TrimPrefix(addresses[0], "http://")
	conn, err := net.DialTimeout("tcp", proxyAddr, 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := fmt.Fprintf(conn, "CONNECT %s:443 HTTP/1.1\r\nHost: %s:443\r\n\r\n", host, host); err != nil {
		t.Fatal(err)
	}
	reader := bufio.NewReader(conn)
	resp, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("CONNECT status %d", resp.StatusCode)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(authority.RootPEM()) {
		t.Fatal("append root")
	}
	tlsConn := tls.Client(&bufferedConn{Conn: conn, reader: reader}, &tls.Config{ServerName: host, RootCAs: roots, MinVersion: tls.VersionTLS12})
	if err := tlsConn.Handshake(); err != nil {
		t.Fatal(err)
	}
	if _, err := fmt.Fprintf(tlsConn, "GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", host); err != nil {
		t.Fatal(err)
	}
	httpResp, err := http.ReadResponse(bufio.NewReader(tlsConn), &http.Request{Method: http.MethodGet})
	if err != nil {
		t.Fatal(err)
	}
	defer httpResp.Body.Close()
	buf := make([]byte, 4)
	if _, err := io.ReadFull(httpResp.Body, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "knot" {
		t.Fatalf("body %q", buf)
	}
}

func TestHTTPSKnotWithoutLocalCADoesNotBlindTunnelTLS(t *testing.T) {
	node := nodeid.FromPublicKey([]byte("node-service-without-ca"))
	host, err := naming.ServiceDomain("web", node)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	gateway := &Gateway{
		Direct: false,
		DialOverlay: func(ctx context.Context, got nodeid.ID, service string) (net.Conn, error) {
			called = true
			return nil, errors.New("must not dial")
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	addresses, err := gateway.Start(ctx, "", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer gateway.Close()
	conn, err := net.DialTimeout("tcp", strings.TrimPrefix(addresses[0], "http://"), 3*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := fmt.Fprintf(conn, "CONNECT %s:443 HTTP/1.1\r\nHost: %s:443\r\n\r\n", host, host); err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("CONNECT status %d", resp.StatusCode)
	}
	if called {
		t.Fatal(".knot CONNECT was blindly tunneled without a local CA")
	}
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "local .knot HTTPS requires") {
		t.Fatalf("unexpected error body %q", body)
	}
}
