// Package knotmobile exposes a gomobile-friendly KnotRoute v4 client API.
package knotmobile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/localzet/knotroute/internal/clientruntime"
	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/overlay"
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
	runtime   *clientruntime.Runtime
	cancel    context.CancelFunc
	listeners []net.Listener
	proxyPort int
}

func CreateClient(optionsJSON string) (*Client, error) {
	var opts options
	if err := json.Unmarshal([]byte(optionsJSON), &opts); err != nil {
		return nil, err
	}
	if strings.TrimSpace(opts.DataDir) == "" {
		return nil, errors.New("data_dir is required")
	}
	if opts.HTTPProxyPort == 0 {
		opts.HTTPProxyPort = 19478
	}
	rt, err := clientruntime.Open(opts.DataDir)
	if err != nil {
		return nil, err
	}
	cfg := rt.Config()
	cfg.Listen = []string{"127.0.0.1:0"}
	cfg.Dashboard = ""
	cfg.Proxy.SOCKS = ""
	cfg.Proxy.HTTP = "127.0.0.1:" + strconv.Itoa(opts.HTTPProxyPort)
	cfg.Proxy.Direct = true
	cfg.CA.Enabled = true
	cfg.Discovery.LAN = true
	cfg.Discovery.PeerExchange = true
	if opts.NetworkID != "" {
		cfg.NetworkID = opts.NetworkID
	}
	cfg.Discovery.Beacons = append([]string(nil), opts.Beacons...)
	if opts.CircuitHops > 0 {
		cfg.Privacy.CircuitHops = opts.CircuitHops
	}
	cfg.Peers = nil
	for _, peer := range opts.Peers {
		cfg.Peers = append(cfg.Peers, config.Peer{Address: peer})
	}
	if err := rt.SaveConfig(cfg); err != nil {
		return nil, err
	}
	return &Client{runtime: rt, proxyPort: opts.HTTPProxyPort}, nil
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
	if err := c.runtime.Start(ctx); err != nil {
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
	c.runtime.Stop()
}
func (c *Client) NodeAddress() string  { return c.runtime.NodeDomain() }
func (c *Client) UserID() string       { return c.runtime.UserID() }
func (c *Client) HTTPProxyURL() string { return "http://127.0.0.1:" + strconv.Itoa(c.proxyPort) }
func (c *Client) RootCAPEM() (string, error) {
	raw, err := c.runtime.RootCAPEM()
	return string(raw), err
}
func (c *Client) CAProfileJSON() (string, error) {
	cfg := c.runtime.Config()
	raw, err := json.Marshal(cfg.CA)
	return string(raw), err
}
func (c *Client) SetCAProfile(commonName, organizationCSV, organizationalUnitCSV, countryCSV, provinceCSV, localityCSV, streetAddressCSV, postalCodeCSV string, validityDays int) error {
	cfg := c.runtime.Config()
	cfg.CA.Subject.CommonName = strings.TrimSpace(commonName)
	cfg.CA.Subject.Organization = splitCSV(organizationCSV)
	cfg.CA.Subject.OrganizationalUnit = splitCSV(organizationalUnitCSV)
	cfg.CA.Subject.Country = splitCSV(countryCSV)
	cfg.CA.Subject.Province = splitCSV(provinceCSV)
	cfg.CA.Subject.Locality = splitCSV(localityCSV)
	cfg.CA.Subject.StreetAddress = splitCSV(streetAddressCSV)
	cfg.CA.Subject.PostalCode = splitCSV(postalCodeCSV)
	cfg.CA.ValidityDays = validityDays
	return c.runtime.SaveConfig(cfg)
}
func (c *Client) RotateCA() (string, error) {
	c.mu.Lock()
	wasRunning := c.cancel != nil
	c.mu.Unlock()
	if wasRunning {
		c.Stop()
	}
	info, err := c.runtime.RotateCA()
	if err != nil {
		if wasRunning {
			_ = c.Start()
		}
		return "", err
	}
	if wasRunning {
		if err := c.Start(); err != nil {
			return "", fmt.Errorf("CA rotated but client restart failed: %w", err)
		}
	}
	raw, err := json.Marshal(info)
	return string(raw), err
}
func (c *Client) StatusJSON() (string, error) {
	st, ok := c.runtime.Status()
	if !ok {
		return "", errors.New("KnotRoute node is not running")
	}
	raw, err := json.Marshal(st)
	return string(raw), err
}
func (c *Client) SocialStateJSON() (string, error) {
	raw, err := json.Marshal(c.runtime.SocialState())
	return string(raw), err
}
func (c *Client) UserProfileJSON() (string, error) {
	p, err := c.runtime.UserProfile()
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(p)
	return string(raw), err
}
func (c *Client) SetUserProfile(name, bio string) error { return c.runtime.SetUserProfile(name, bio) }
func (c *Client) AddContact(nodeReference, alias string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	v, err := c.runtime.AddContact(ctx, nodeReference, alias)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(v)
	return string(raw), err
}
func (c *Client) SendMessage(userID, body string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	v, err := c.runtime.SendMessage(ctx, userID, body)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(v)
	return string(raw), err
}
func (c *Client) CreatePost(text, tagsCSV string) (string, error) {
	tags := strings.Split(tagsCSV, ",")
	v, err := c.runtime.CreatePost(text, tags)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(v)
	return string(raw), err
}
func (c *Client) FetchContactFeed(userID string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	v, err := c.runtime.FetchContactFeed(ctx, userID)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(v)
	return string(raw), err
}

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
	remote, err := c.runtime.Dial(context.Background(), address)
	if err != nil {
		return
	}
	defer remote.Close()
	done := make(chan struct{}, 2)
	go func() { _, _ = io.Copy(remote, local); done <- struct{}{} }()
	go func() { _, _ = io.Copy(local, remote); done <- struct{}{} }()
	<-done
}
func splitCSV(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			out = append(out, value)
		}
	}
	return out
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
