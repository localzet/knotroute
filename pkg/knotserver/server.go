// Package knotserver embeds a KnotRoute node and publishes in-process services.
package knotserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/identity"
	"github.com/localzet/knotroute/internal/overlay"
)

// Handler serves one incoming KnotRoute stream. The connection is closed when
// the handler returns.
type Handler interface {
	ServeKnot(net.Conn)
}

// HandlerFunc adapts a function to Handler.
type HandlerFunc func(net.Conn)

func (f HandlerFunc) ServeKnot(conn net.Conn) { f(conn) }

type Service struct {
	Name        string
	Description string
	Metadata    map[string]string
	IntroCount  int
	Handler     Handler
}

type Options struct {
	DataDir      string
	NetworkID    string
	Beacons      []string
	Peers        []string
	Listen       []string
	Advertise    []string
	CircuitHops  int
	EnableLAN    bool
	PeerExchange bool
	Services     []Service
}

type Host struct {
	mu        sync.Mutex
	cfg       config.Config
	id        *identity.Identity
	node      *overlay.Node
	cancel    context.CancelFunc
	listeners map[string]net.Listener
	handlers  map[string]Handler
	wg        sync.WaitGroup
}

func New(options Options) (*Host, error) {
	if strings.TrimSpace(options.DataDir) == "" {
		return nil, errors.New("DataDir is required")
	}
	if len(options.Services) == 0 {
		return nil, errors.New("at least one service is required")
	}
	if err := os.MkdirAll(options.DataDir, 0o700); err != nil {
		return nil, err
	}
	cfg := config.Default()
	cfg.Path = filepath.Join(options.DataDir, "embedded-server.json")
	cfg.IdentityFile = filepath.Join(options.DataDir, "identity.json")
	cfg.Dashboard = ""
	cfg.Proxy.HTTP = ""
	cfg.Proxy.SOCKS = ""
	cfg.Proxy.Direct = false
	cfg.CA.Enabled = false
	cfg.Discovery.Beacons = append([]string(nil), options.Beacons...)
	cfg.Discovery.LAN = options.EnableLAN
	cfg.Discovery.PeerExchange = options.PeerExchange
	cfg.Discovery.CacheFile = filepath.Join(options.DataDir, "peers.json")
	if options.NetworkID != "" {
		cfg.NetworkID = options.NetworkID
	}
	if options.CircuitHops > 0 {
		cfg.Privacy.CircuitHops = options.CircuitHops
	}
	if len(options.Listen) > 0 {
		cfg.Listen = append([]string(nil), options.Listen...)
	} else {
		cfg.Listen = []string{"127.0.0.1:0"}
	}
	cfg.Advertise = append([]string(nil), options.Advertise...)
	for _, peer := range options.Peers {
		cfg.Peers = append(cfg.Peers, config.Peer{Address: peer})
	}

	h := &Host{cfg: cfg, listeners: map[string]net.Listener{}, handlers: map[string]Handler{}}
	cleanup := true
	defer func() {
		if cleanup {
			h.closeListeners()
		}
	}()
	seen := map[string]bool{}
	for _, service := range options.Services {
		if strings.TrimSpace(service.Name) == "" {
			return nil, errors.New("service name is required")
		}
		if seen[service.Name] {
			return nil, fmt.Errorf("duplicate service %q", service.Name)
		}
		seen[service.Name] = true
		if service.Handler == nil {
			return nil, fmt.Errorf("service %q has no handler", service.Name)
		}
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return nil, fmt.Errorf("listen for service %q: %w", service.Name, err)
		}
		h.listeners[service.Name] = listener
		h.handlers[service.Name] = service.Handler
		cfg.Services = append(cfg.Services, config.Service{
			Name: service.Name, Target: listener.Addr().String(), Description: service.Description,
			Allow: []string{"*"}, Publish: true, IntroCount: service.IntroCount,
			IdentityFile: filepath.Join(options.DataDir, "services", service.Name+".identity.json"),
			Metadata:     cloneMap(service.Metadata),
		})
	}
	h.cfg = cfg
	if err := h.cfg.Validate(); err != nil {
		return nil, err
	}
	id, err := identity.Load(h.cfg.IdentityFile)
	if errors.Is(err, os.ErrNotExist) || (err != nil && strings.Contains(err.Error(), "no such file")) {
		id, err = identity.Generate()
		if err == nil {
			err = id.Save(h.cfg.IdentityFile)
		}
	}
	if err != nil {
		return nil, err
	}
	h.id = id
	node, err := overlay.New(h.cfg, h.id)
	if err != nil {
		return nil, err
	}
	h.node = node
	cleanup = false
	return h, nil
}

func (h *Host) Start(ctx context.Context) error {
	h.mu.Lock()
	if h.cancel != nil {
		h.mu.Unlock()
		return nil
	}
	runCtx, cancel := context.WithCancel(ctx)
	h.cancel = cancel
	h.mu.Unlock()
	if err := h.node.Start(runCtx); err != nil {
		cancel()
		h.mu.Lock()
		h.cancel = nil
		h.mu.Unlock()
		return err
	}
	for name, listener := range h.listeners {
		h.wg.Add(1)
		go h.acceptLoop(runCtx, name, listener, h.handlers[name])
	}
	return nil
}

func (h *Host) Close() error {
	h.mu.Lock()
	cancel := h.cancel
	h.cancel = nil
	h.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	h.closeListeners()
	if h.node != nil {
		h.node.Stop()
	}
	h.wg.Wait()
	return nil
}

func (h *Host) NodeAddress() string { return h.node.Domain() }

func (h *Host) ServiceAddress(name string) (string, error) {
	for _, service := range h.node.Status().Services {
		if service.Name == name && service.Published && service.Domain != "" {
			return service.Domain, nil
		}
	}
	return "", fmt.Errorf("unknown published service %q", name)
}

func (h *Host) acceptLoop(ctx context.Context, name string, listener net.Listener, handler Handler) {
	defer h.wg.Done()
	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				return
			default:
				return
			}
		}
		h.wg.Add(1)
		go func() {
			defer h.wg.Done()
			defer conn.Close()
			defer func() { _ = recover() }()
			handler.ServeKnot(conn)
		}()
	}
}

func (h *Host) closeListeners() {
	for _, listener := range h.listeners {
		_ = listener.Close()
	}
}

func cloneMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
