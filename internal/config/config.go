package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/localzet/knotroute/internal/naming"
)

type Peer struct {
	Address    string `json:"address"`
	ExpectedID string `json:"expected_id,omitempty"`
}

type Service struct {
	Name        string   `json:"name"`
	Target      string   `json:"target"`
	Description string   `json:"description,omitempty"`
	Allow       []string `json:"allow,omitempty"`
}

type Forward struct {
	Listen  string `json:"listen"`
	Node    string `json:"node"`
	Service string `json:"service"`
}

type Routing struct {
	LSAInterval string `json:"lsa_interval"`
	LSATTL      string `json:"lsa_ttl"`
	MaxHops     int    `json:"max_hops"`
}

type Proxy struct {
	SOCKS        string `json:"socks"`
	HTTP         string `json:"http"`
	Direct       bool   `json:"direct"`
	DefaultHTTP  string `json:"default_http_service"`
	DefaultHTTPS string `json:"default_https_service"`
}

type Config struct {
	IdentityFile string         `json:"identity_file"`
	Listen       []string       `json:"listen"`
	Advertise    []string       `json:"advertise,omitempty"`
	Peers        []Peer         `json:"peers,omitempty"`
	Services     []Service      `json:"services,omitempty"`
	Forwards     []Forward      `json:"forwards,omitempty"`
	Aliases      []naming.Alias `json:"aliases,omitempty"`
	Proxy        Proxy          `json:"proxy"`
	Dashboard    string         `json:"dashboard"`
	Routing      Routing        `json:"routing"`

	Path string `json:"-"`
}

func Default() Config {
	return Config{
		IdentityFile: "identity.json",
		Listen:       []string{"0.0.0.0:7447"},
		Dashboard:    "127.0.0.1:8484",
		Proxy: Proxy{
			SOCKS: "127.0.0.1:9477", HTTP: "127.0.0.1:9478", Direct: true,
			DefaultHTTP: "http", DefaultHTTPS: "https",
		},
		Routing:  Routing{LSAInterval: "20s", LSATTL: "90s", MaxHops: 16},
		Services: []Service{},
		Forwards: []Forward{},
		Peers:    []Peer{},
		Aliases:  []naming.Alias{},
	}
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return Config{}, err
	}
	cfg.Path = abs
	if !filepath.IsAbs(cfg.IdentityFile) {
		cfg.IdentityFile = filepath.Join(filepath.Dir(abs), cfg.IdentityFile)
	}
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func SaveAtomic(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil && dir != "." {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".knotroute-*.json")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

var serviceName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)

func (c Config) Validate() error {
	var errs []error
	if strings.TrimSpace(c.IdentityFile) == "" {
		errs = append(errs, errors.New("identity_file is required"))
	}
	if len(c.Listen) == 0 {
		errs = append(errs, errors.New("at least one listen address is required"))
	}
	for _, address := range c.Listen {
		if err := validateAddress("listen", address); err != nil {
			errs = append(errs, err)
		}
	}
	for _, address := range c.Advertise {
		if err := validateAddress("advertise", address); err != nil {
			errs = append(errs, err)
		}
	}
	for i, p := range c.Peers {
		if err := validateAddress(fmt.Sprintf("peers[%d].address", i), p.Address); err != nil {
			errs = append(errs, err)
		}
		if p.ExpectedID != "" {
			if _, err := naming.ParseNodeReference(p.ExpectedID); err != nil {
				errs = append(errs, fmt.Errorf("peers[%d].expected_id: %w", i, err))
			}
		}
	}
	seenServices := map[string]struct{}{}
	for i, s := range c.Services {
		if !serviceName.MatchString(s.Name) {
			errs = append(errs, fmt.Errorf("services[%d].name is invalid", i))
		}
		if _, ok := seenServices[s.Name]; ok {
			errs = append(errs, fmt.Errorf("duplicate service %q", s.Name))
		}
		seenServices[s.Name] = struct{}{}
		if err := validateAddress(fmt.Sprintf("services[%d].target", i), s.Target); err != nil {
			errs = append(errs, err)
		}
		for _, allowed := range s.Allow {
			if allowed != "*" {
				if _, err := naming.ParseNodeReference(allowed); err != nil {
					errs = append(errs, fmt.Errorf("services[%d].allow: %w", i, err))
				}
			}
		}
	}
	for i, f := range c.Forwards {
		if err := validateAddress(fmt.Sprintf("forwards[%d].listen", i), f.Listen); err != nil {
			errs = append(errs, err)
		}
		if _, err := naming.ResolveNodeReference(f.Node, c.Aliases); err != nil {
			errs = append(errs, fmt.Errorf("forwards[%d].node: %w", i, err))
		}
		if !serviceName.MatchString(f.Service) {
			errs = append(errs, fmt.Errorf("forwards[%d].service is invalid", i))
		}
	}
	seenAliases := map[string]struct{}{}
	for i, a := range c.Aliases {
		if err := naming.ValidateAlias(a); err != nil {
			errs = append(errs, fmt.Errorf("aliases[%d]: %w", i, err))
		}
		key := strings.ToLower(strings.TrimSpace(a.Name))
		if _, ok := seenAliases[key]; ok {
			errs = append(errs, fmt.Errorf("duplicate alias %q", a.Name))
		}
		seenAliases[key] = struct{}{}
	}
	if c.Proxy.SOCKS != "" {
		if err := validateAddress("proxy.socks", c.Proxy.SOCKS); err != nil {
			errs = append(errs, err)
		}
	}
	if c.Proxy.HTTP != "" {
		if err := validateAddress("proxy.http", c.Proxy.HTTP); err != nil {
			errs = append(errs, err)
		}
	}
	if c.Proxy.DefaultHTTP == "" {
		c.Proxy.DefaultHTTP = "http"
	}
	if c.Proxy.DefaultHTTPS == "" {
		c.Proxy.DefaultHTTPS = "https"
	}
	if !serviceName.MatchString(c.Proxy.DefaultHTTP) {
		errs = append(errs, errors.New("proxy.default_http_service is invalid"))
	}
	if !serviceName.MatchString(c.Proxy.DefaultHTTPS) {
		errs = append(errs, errors.New("proxy.default_https_service is invalid"))
	}
	if c.Dashboard != "" {
		if err := validateAddress("dashboard", c.Dashboard); err != nil {
			errs = append(errs, err)
		}
	}
	interval, err := c.LSAInterval()
	if err != nil {
		errs = append(errs, fmt.Errorf("routing.lsa_interval: %w", err))
	}
	ttl, err := c.LSATTL()
	if err != nil {
		errs = append(errs, fmt.Errorf("routing.lsa_ttl: %w", err))
	}
	if interval > 0 && ttl > 0 && ttl < interval*2 {
		errs = append(errs, errors.New("routing.lsa_ttl must be at least twice lsa_interval"))
	}
	if c.Routing.MaxHops < 2 || c.Routing.MaxHops > 64 {
		errs = append(errs, errors.New("routing.max_hops must be between 2 and 64"))
	}
	return errors.Join(errs...)
}

func validateAddress(field, address string) error {
	if strings.TrimSpace(address) == "" {
		return fmt.Errorf("%s is empty", field)
	}
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("%s: %w", field, err)
	}
	if host == "" && !strings.HasPrefix(address, ":") {
		return fmt.Errorf("%s has an empty host", field)
	}
	if port == "" {
		return fmt.Errorf("%s has an empty port", field)
	}
	return nil
}

func (c Config) LSAInterval() (time.Duration, error) {
	return time.ParseDuration(c.Routing.LSAInterval)
}
func (c Config) LSATTL() (time.Duration, error) { return time.ParseDuration(c.Routing.LSATTL) }
