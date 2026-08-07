// Package knotclient embeds a KnotRoute client node into Go applications.
package knotclient

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/identity"
	"github.com/localzet/knotroute/internal/naming"
	"github.com/localzet/knotroute/internal/networkid"
	"github.com/localzet/knotroute/internal/overlay"
)

type Options struct {
	DataDir       string
	NetworkID     string
	Beacons       []string
	Peers         []string
	Aliases       []naming.Alias
	CircuitHops   int
	EnableLAN     bool
	PeerExchange  bool
	HTTPProxy     string
	SOCKSProxy    string
	ProxyDirect   bool
	EnableLocalCA bool
	Ephemeral     bool
}

type Client struct {
	mu     sync.Mutex
	node   *overlay.Node
	cancel context.CancelFunc
	cfg    config.Config
	id     *identity.Identity
}

func New(options Options) (*Client, error) {
	cfg := config.Default()
	cfg.Listen = []string{"127.0.0.1:0"}
	cfg.Dashboard = ""
	cfg.Proxy.HTTP = options.HTTPProxy
	cfg.Proxy.SOCKS = options.SOCKSProxy
	cfg.Proxy.Direct = options.ProxyDirect
	cfg.CA.Enabled = options.EnableLocalCA
	cfg.Discovery.LAN = options.EnableLAN
	cfg.Discovery.PeerExchange = options.PeerExchange
	cfg.Discovery.Beacons = append([]string(nil), options.Beacons...)
	cfg.Aliases = append([]naming.Alias(nil), options.Aliases...)
	if options.CircuitHops > 0 {
		cfg.Privacy.CircuitHops = options.CircuitHops
	}
	if options.NetworkID != "" {
		if _, err := networkid.Parse(options.NetworkID); err != nil {
			return nil, fmt.Errorf("network id: %w", err)
		}
		cfg.NetworkID = options.NetworkID
	}
	for _, address := range options.Peers {
		cfg.Peers = append(cfg.Peers, config.Peer{Address: address})
	}
	var id *identity.Identity
	var err error
	if options.Ephemeral {
		id, err = identity.Generate()
		cfg.Discovery.CacheFile = ""
	} else {
		if strings.TrimSpace(options.DataDir) == "" {
			return nil, errors.New("DataDir is required unless Ephemeral is enabled")
		}
		if err = os.MkdirAll(options.DataDir, 0o700); err != nil {
			return nil, err
		}
		cfg.Path = filepath.Join(options.DataDir, "embedded.json")
		if options.EnableLocalCA {
			cfg.CA.Directory = filepath.Join(options.DataDir, "ca")
		}
		cfg.IdentityFile = filepath.Join(options.DataDir, "identity.json")
		cfg.Discovery.CacheFile = filepath.Join(options.DataDir, "peers.json")
		id, err = identity.Load(cfg.IdentityFile)
		if errors.Is(err, os.ErrNotExist) || (err != nil && strings.Contains(err.Error(), "no such file")) {
			id, err = identity.Generate()
			if err == nil {
				err = id.Save(cfg.IdentityFile)
			}
		}
	}
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &Client{cfg: cfg, id: id}, nil
}

func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.node != nil {
		return nil
	}
	node, err := overlay.New(c.cfg, c.id)
	if err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	if err := node.Start(runCtx); err != nil {
		cancel()
		return err
	}
	c.node, c.cancel = node, cancel
	return nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	node, cancel := c.node, c.cancel
	c.node, c.cancel = nil, nil
	c.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if node != nil {
		node.Stop()
	}
	return nil
}

func (c *Client) RootCAPath() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.cfg.CA.Enabled {
		return ""
	}
	return filepath.Join(c.cfg.CA.Directory, "root-ca.pem")
}

func (c *Client) NodeAddress() string {
	return naming.CanonicalDomain(c.id.ID)
}

func (c *Client) Dial(ctx context.Context, address string) (net.Conn, error) {
	c.mu.Lock()
	node := c.node
	aliases := append([]naming.Alias(nil), c.cfg.Aliases...)
	c.mu.Unlock()
	if node == nil {
		return nil, errors.New("KnotRoute client is not started")
	}
	host := address
	if h, _, err := net.SplitHostPort(address); err == nil {
		host = h
	}
	resolved, err := naming.ResolveHost(host, aliases)
	if err != nil {
		return nil, err
	}
	if resolved.Kind == naming.AddressService {
		return node.OpenService(ctx, resolved.ServiceID)
	}
	return node.OpenCircuitStream(ctx, resolved.Node, resolved.Service)
}

// DialContext implements the net/http Transport dial hook. Plain HTTP over a
// service identity is already protected by KnotRoute's end-to-end encryption.
func (c *Client) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if network != "tcp" && network != "tcp4" && network != "tcp6" {
		return nil, fmt.Errorf("unsupported network %q", network)
	}
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		host = address
	}
	if strings.HasSuffix(strings.ToLower(strings.TrimSuffix(host, ".")), naming.Suffix) {
		return c.Dial(ctx, host)
	}
	return (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext(ctx, network, address)
}

// HTTPClient returns a client that reaches http://*.knot through the embedded
// node while preserving normal Internet behavior for other hosts.
func (c *Client) HTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = c.DialContext
	return &http.Client{Transport: transport, Timeout: 60 * time.Second}
}
