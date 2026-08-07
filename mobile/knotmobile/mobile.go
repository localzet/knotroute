// Package knotmobile exposes a gomobile-friendly KnotRoute client API.
package knotmobile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"sync"

	"github.com/localzet/knotroute/internal/overlay"
	"github.com/localzet/knotroute/pkg/knotclient"
)

type options struct {
	DataDir       string   `json:"data_dir"`
	NetworkID     string   `json:"network_id,omitempty"`
	Beacons       []string `json:"beacons,omitempty"`
	Peers         []string `json:"peers,omitempty"`
	CircuitHops   int      `json:"circuit_hops,omitempty"`
	HTTPProxyPort int      `json:"http_proxy_port,omitempty"`
}

type Client struct {
	mu        sync.Mutex
	core      *knotclient.Client
	cancel    context.CancelFunc
	listeners []net.Listener
	proxyPort int
}

func CreateClient(optionsJSON string) (*Client, error) {
	var opts options
	if err := json.Unmarshal([]byte(optionsJSON), &opts); err != nil {
		return nil, err
	}
	if opts.HTTPProxyPort == 0 {
		opts.HTTPProxyPort = 19478
	}
	core, err := knotclient.New(knotclient.Options{
		DataDir: opts.DataDir, NetworkID: opts.NetworkID, Beacons: opts.Beacons,
		Peers: opts.Peers, CircuitHops: opts.CircuitHops, EnableLAN: true,
		PeerExchange: true, ProxyDirect: true, EnableLocalCA: true, HTTPProxy: "127.0.0.1:" + strconv.Itoa(opts.HTTPProxyPort),
	})
	if err != nil {
		return nil, err
	}
	return &Client{core: core, proxyPort: opts.HTTPProxyPort}, nil
}

func (c *Client) Start() error {
	c.mu.Lock()
	if c.cancel != nil {
		c.mu.Unlock()
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	c.mu.Unlock()
	if err := c.core.Start(ctx); err != nil {
		cancel()
		c.mu.Lock()
		c.cancel = nil
		c.mu.Unlock()
		return err
	}
	return nil
}

func (c *Client) Stop() {
	c.mu.Lock()
	cancel := c.cancel
	listeners := append([]net.Listener(nil), c.listeners...)
	c.listeners = nil
	c.cancel = nil
	c.mu.Unlock()
	for _, l := range listeners {
		_ = l.Close()
	}
	if cancel != nil {
		cancel()
	}
	_ = c.core.Close()
}

func (c *Client) NodeAddress() string  { return c.core.NodeAddress() }
func (c *Client) HTTPProxyURL() string { return "http://127.0.0.1:" + strconv.Itoa(c.proxyPort) }
func (c *Client) RootCAPEM() (string, error) {
	data, err := os.ReadFile(c.core.RootCAPath())
	return string(data), err
}

// OpenForward creates a loopback TCP endpoint and returns its port. Each
// connection is forwarded to the supplied .knot service through the embedded core.
func (c *Client) OpenForward(address string) (int, error) {
	if address == "" {
		return 0, errors.New("address is required")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	c.mu.Lock()
	c.listeners = append(c.listeners, listener)
	c.mu.Unlock()
	go func() {
		for {
			local, err := listener.Accept()
			if err != nil {
				return
			}
			go c.bridge(local, address)
		}
	}()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func (c *Client) bridge(local net.Conn, address string) {
	defer local.Close()
	remote, err := c.core.Dial(context.Background(), address)
	if err != nil {
		return
	}
	defer remote.Close()
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(remote, local); done <- struct{}{} }()
	go func() { _, _ = io.Copy(local, remote); done <- struct{}{} }()
	<-done
}

func Version() string { return overlay.Version }
func ValidateOptions(optionsJSON string) error {
	var opts options
	if err := json.Unmarshal([]byte(optionsJSON), &opts); err != nil {
		return fmt.Errorf("invalid options: %w", err)
	}
	if opts.DataDir == "" {
		return errors.New("data_dir is required")
	}
	return nil
}
