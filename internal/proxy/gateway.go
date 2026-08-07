package proxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/localzet/knotroute/internal/certauth"
	"github.com/localzet/knotroute/internal/naming"
	"github.com/localzet/knotroute/internal/nodeid"
	"github.com/localzet/knotroute/internal/serviceid"
)

type OverlayDialer func(context.Context, nodeid.ID, string) (net.Conn, error)
type ServiceDialer func(context.Context, serviceid.ID) (net.Conn, error)

type EventFunc func(level, message string)

type Gateway struct {
	Aliases        []naming.Alias
	Direct         bool
	DefaultHTTP    string
	DefaultHTTPS   string
	DialOverlay    OverlayDialer
	DialService    ServiceDialer
	Authority      *certauth.Authority
	InterceptHTTPS bool
	Event          EventFunc

	mu          sync.Mutex
	listeners   []net.Listener
	connections map[net.Conn]struct{}
	wg          sync.WaitGroup
}

func (g *Gateway) Start(ctx context.Context, socksAddr, httpAddr string) ([]string, error) {
	g.mu.Lock()
	if g.connections == nil {
		g.connections = make(map[net.Conn]struct{})
	}
	g.mu.Unlock()
	var started []string
	if socksAddr != "" {
		l, err := net.Listen("tcp", socksAddr)
		if err != nil {
			g.Close()
			return nil, fmt.Errorf("SOCKS5 listen %s: %w", socksAddr, err)
		}
		g.addListener(l)
		started = append(started, "socks5://"+l.Addr().String())
		g.wg.Add(1)
		go func() { defer g.wg.Done(); g.acceptLoop(ctx, l, g.handleSOCKS) }()
	}
	if httpAddr != "" {
		l, err := net.Listen("tcp", httpAddr)
		if err != nil {
			g.Close()
			return nil, fmt.Errorf("HTTP proxy listen %s: %w", httpAddr, err)
		}
		g.addListener(l)
		started = append(started, "http://"+l.Addr().String())
		g.wg.Add(1)
		go func() { defer g.wg.Done(); g.acceptLoop(ctx, l, g.handleHTTP) }()
	}
	return started, nil
}

func (g *Gateway) Close() {
	g.mu.Lock()
	listeners := append([]net.Listener(nil), g.listeners...)
	connections := make([]net.Conn, 0, len(g.connections))
	for conn := range g.connections {
		connections = append(connections, conn)
	}
	g.listeners = nil
	g.mu.Unlock()
	for _, l := range listeners {
		_ = l.Close()
	}
	for _, conn := range connections {
		_ = conn.Close()
	}
	g.wg.Wait()
}

func (g *Gateway) addListener(l net.Listener) {
	g.mu.Lock()
	g.listeners = append(g.listeners, l)
	g.mu.Unlock()
}

func (g *Gateway) acceptLoop(ctx context.Context, listener net.Listener, handler func(context.Context, net.Conn)) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			g.event("warn", "proxy accept: "+err.Error())
			continue
		}
		g.mu.Lock()
		g.connections[conn] = struct{}{}
		g.mu.Unlock()
		g.wg.Add(1)
		go func() {
			defer g.wg.Done()
			defer func() {
				g.mu.Lock()
				delete(g.connections, conn)
				g.mu.Unlock()
			}()
			handler(ctx, conn)
		}()
	}
}

func (g *Gateway) event(level, message string) {
	if g.Event != nil {
		g.Event(level, message)
	}
}

func (g *Gateway) dial(ctx context.Context, host, port, scheme string) (net.Conn, string, error) {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if strings.HasSuffix(host, naming.Suffix) {
		resolved, err := naming.ResolveHost(host, g.Aliases)
		if err != nil {
			return nil, "", err
		}
		if resolved.Kind == naming.AddressService {
			if g.DialService == nil {
				return nil, "", errors.New("service dialer is unavailable")
			}
			conn, err := g.DialService(ctx, resolved.ServiceID)
			return conn, resolved.ServiceID.Short(), err
		}
		service := resolved.Service
		if service == naming.DefaultService {
			if scheme == "https" && g.DefaultHTTPS != "" {
				service = g.DefaultHTTPS
			}
			if scheme == "http" && g.DefaultHTTP != "" {
				service = g.DefaultHTTP
			}
		}
		if g.DialOverlay == nil {
			return nil, "", errors.New("overlay dialer is unavailable")
		}
		conn, err := g.DialOverlay(ctx, resolved.Node, service)
		return conn, resolved.Node.Short() + "/" + service, err
	}
	if !g.Direct {
		return nil, "", fmt.Errorf("direct proxying is disabled for %s", host)
	}
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	dialer := net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(host, port))
	return conn, net.JoinHostPort(host, port), err
}

func (g *Gateway) handleSOCKS(ctx context.Context, client net.Conn) {
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(30 * time.Second))
	reader := bufio.NewReader(client)
	version, err := reader.ReadByte()
	if err != nil || version != 5 {
		return
	}
	methods, err := reader.ReadByte()
	if err != nil {
		return
	}
	methodList := make([]byte, int(methods))
	if _, err = io.ReadFull(reader, methodList); err != nil {
		return
	}
	noAuth := false
	for _, method := range methodList {
		if method == 0 {
			noAuth = true
		}
	}
	if !noAuth {
		_, _ = client.Write([]byte{5, 0xff})
		return
	}
	if _, err = client.Write([]byte{5, 0}); err != nil {
		return
	}
	header := make([]byte, 4)
	if _, err = io.ReadFull(reader, header); err != nil {
		return
	}
	if header[0] != 5 || header[1] != 1 {
		g.writeSOCKSReply(client, 7)
		return
	}
	var host string
	switch header[3] {
	case 1:
		addr := make([]byte, 4)
		if _, err = io.ReadFull(reader, addr); err != nil {
			return
		}
		host = net.IP(addr).String()
	case 3:
		length, e := reader.ReadByte()
		if e != nil {
			return
		}
		addr := make([]byte, int(length))
		if _, err = io.ReadFull(reader, addr); err != nil {
			return
		}
		host = string(addr)
	case 4:
		addr := make([]byte, 16)
		if _, err = io.ReadFull(reader, addr); err != nil {
			return
		}
		host = net.IP(addr).String()
	default:
		g.writeSOCKSReply(client, 8)
		return
	}
	portRaw := make([]byte, 2)
	if _, err = io.ReadFull(reader, portRaw); err != nil {
		return
	}
	port := strconv.Itoa(int(binary.BigEndian.Uint16(portRaw)))
	scheme := "tcp"
	if port == "80" {
		scheme = "http"
	} else if port == "443" {
		scheme = "https"
	}
	openCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	upstream, target, err := g.dial(openCtx, host, port, scheme)
	if err != nil {
		g.event("warn", "SOCKS "+host+":"+port+": "+err.Error())
		g.writeSOCKSReply(client, 4)
		return
	}
	defer upstream.Close()
	if err := g.writeSOCKSReply(client, 0); err != nil {
		return
	}
	_ = client.SetDeadline(time.Time{})
	_ = upstream.SetDeadline(time.Time{})
	g.event("info", "SOCKS connected "+host+":"+port+" -> "+target)
	proxyBoth(client, upstream)
}

func (g *Gateway) writeSOCKSReply(conn net.Conn, code byte) error {
	_, err := conn.Write([]byte{5, code, 0, 1, 0, 0, 0, 0, 0, 0})
	return err
}

func (g *Gateway) handleHTTP(ctx context.Context, client net.Conn) {
	defer client.Close()
	_ = client.SetDeadline(time.Now().Add(30 * time.Second))
	reader := bufio.NewReader(client)
	req, err := http.ReadRequest(reader)
	if err != nil {
		writeProxyError(client, http.StatusBadRequest, err.Error())
		return
	}
	defer req.Body.Close()
	if strings.EqualFold(req.Method, http.MethodConnect) {
		g.handleConnect(ctx, client, req)
		return
	}
	host, port := splitRequestHost(req)
	if host == "" {
		writeProxyError(client, http.StatusBadRequest, "request has no host")
		return
	}
	scheme := req.URL.Scheme
	if scheme == "" {
		scheme = "http"
	}
	openCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	upstream, target, err := g.dial(openCtx, host, port, scheme)
	if err != nil {
		g.event("warn", "HTTP "+host+": "+err.Error())
		writeProxyError(client, http.StatusBadGateway, err.Error())
		return
	}
	defer upstream.Close()
	req.RequestURI = ""
	req.URL.Scheme = ""
	req.URL.Host = ""
	req.Header.Del("Proxy-Connection")
	req.Header.Del("Proxy-Authorization")
	if err := req.Write(upstream); err != nil {
		writeProxyError(client, http.StatusBadGateway, err.Error())
		return
	}
	_ = client.SetDeadline(time.Time{})
	_ = upstream.SetDeadline(time.Time{})
	g.event("info", "HTTP connected "+host+" -> "+target)
	proxyBoth(client, upstream)
}

func (g *Gateway) handleConnect(ctx context.Context, client net.Conn, req *http.Request) {
	host, port, err := net.SplitHostPort(req.Host)
	if err != nil {
		host = req.Host
		port = "443"
	}
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if g.InterceptHTTPS && g.Authority != nil && strings.HasSuffix(host, naming.Suffix) {
		resolved, resolveErr := naming.ResolveHost(host, g.Aliases)
		if resolveErr == nil && resolved.Kind == naming.AddressService {
			g.handleKnotTLS(ctx, client, host, resolved.ServiceID)
			return
		}
	}
	openCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	upstream, target, err := g.dial(openCtx, host, port, "https")
	if err != nil {
		g.event("warn", "CONNECT "+req.Host+": "+err.Error())
		writeProxyError(client, http.StatusBadGateway, err.Error())
		return
	}
	defer upstream.Close()
	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\nProxy-Agent: KnotRoute\r\n\r\n"); err != nil {
		return
	}
	_ = client.SetDeadline(time.Time{})
	_ = upstream.SetDeadline(time.Time{})
	g.event("info", "CONNECT "+req.Host+" -> "+target)
	proxyBoth(client, upstream)
}

func (g *Gateway) handleKnotTLS(ctx context.Context, client net.Conn, host string, service serviceid.ID) {
	if g.DialService == nil {
		writeProxyError(client, http.StatusBadGateway, "service dialer is unavailable")
		return
	}
	openCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	upstream, err := g.DialService(openCtx, service)
	if err != nil {
		g.event("warn", "Knot TLS "+host+": "+err.Error())
		writeProxyError(client, http.StatusBadGateway, err.Error())
		return
	}
	defer upstream.Close()
	cert, err := g.Authority.Certificate(host)
	if err != nil {
		writeProxyError(client, http.StatusBadGateway, err.Error())
		return
	}
	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\nProxy-Agent: KnotRoute\r\n\r\n"); err != nil {
		return
	}
	_ = client.SetDeadline(time.Now().Add(20 * time.Second))
	tlsClient := tls.Server(client, &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12, NextProtos: []string{"http/1.1"}})
	if err := tlsClient.HandshakeContext(openCtx); err != nil {
		g.event("warn", "Knot TLS handshake "+host+": "+err.Error())
		return
	}
	_ = tlsClient.SetDeadline(time.Time{})
	_ = upstream.SetDeadline(time.Time{})
	g.event("info", "HTTPS terminated locally for "+host+" -> "+service.Short())
	proxyBoth(tlsClient, upstream)
}

func splitRequestHost(req *http.Request) (string, string) {
	hostPort := req.URL.Host
	if hostPort == "" {
		hostPort = req.Host
	}
	if host, port, err := net.SplitHostPort(hostPort); err == nil {
		return host, port
	}
	return hostPort, ""
}

func writeProxyError(conn net.Conn, status int, message string) {
	message = strings.ReplaceAll(message, "\r", " ")
	message = strings.ReplaceAll(message, "\n", " ")
	body := http.StatusText(status) + ": " + message + "\n"
	_, _ = fmt.Fprintf(conn, "HTTP/1.1 %d %s\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s", status, http.StatusText(status), len(body), body)
}

func proxyBoth(a, b net.Conn) {
	done := make(chan struct{}, 2)
	copyOne := func(dst, src net.Conn) {
		_, _ = io.Copy(dst, src)
		if tcp, ok := dst.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
		done <- struct{}{}
	}
	go copyOne(a, b)
	go copyOne(b, a)
	<-done
}

func PAC(httpProxy string) string {
	// The normal path is DIRECT. If another product owns a TUN interface, those
	// direct connections still follow the operating system's routes and remain
	// under that product's control.
	return fmt.Sprintf(`function FindProxyForURL(url, host) {
  host = host.toLowerCase();
  if (host === "knot" || dnsDomainIs(host, ".knot")) return "PROXY %s";
  return "DIRECT";
}
`, httpProxy)
}

// ParseProxyURL is used by the desktop controller and tests.
func ParseProxyURL(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Scheme != "http" || u.Host == "" {
		return "", errors.New("proxy URL must be http://host:port")
	}
	return u.Host, nil
}
