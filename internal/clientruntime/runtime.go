// Package clientruntime owns the lifecycle of an interactive KnotRoute client.
// It is intentionally UI-agnostic and is shared by the native desktop client
// and integration tests. Unlike the v3 desktop controller it runs the node in
// process, so a desktop installation can be shipped as one executable.
package clientruntime

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/localzet/knotroute/internal/certauth"
	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/identity"
	"github.com/localzet/knotroute/internal/naming"
	"github.com/localzet/knotroute/internal/overlay"
	"github.com/localzet/knotroute/internal/social"
)

type Runtime struct {
	mu             sync.RWMutex
	dataDir        string
	configPath     string
	cfg            config.Config
	identity       *identity.Identity
	userIdentity   *social.Identity
	socialStore    *social.Store
	socialListener net.Listener
	socialWG       sync.WaitGroup
	node           *overlay.Node
	cancel         context.CancelFunc
	started        time.Time
	lastError      string
}

func Open(dataDir string) (*Runtime, error) {
	if strings.TrimSpace(dataDir) == "" {
		return nil, errors.New("client data directory is required")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dataDir, "knotroute.json")
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		cfg := config.Default()
		cfg.IdentityFile = "identity.json"
		cfg.CA.Directory = "ca"
		cfg.Discovery.CacheFile = "peers.json"
		// The v4 UI is native. The loopback dashboard remains available only as
		// a compatibility API/PAC endpoint; it is no longer the desktop UI.
		cfg.Dashboard = "127.0.0.1:8484"
		if err := config.Save(path, cfg); err != nil {
			return nil, err
		}
	}
	cfg, id, err := loadFiles(path)
	if err != nil {
		return nil, err
	}
	userIdentity, err := social.LoadOrCreate(filepath.Join(dataDir, "user-identity.json"))
	if err != nil {
		return nil, fmt.Errorf("load user identity: %w", err)
	}
	socialStore, err := social.OpenStore(filepath.Join(dataDir, "social.json"))
	if err != nil {
		return nil, fmt.Errorf("load social store: %w", err)
	}
	if strings.TrimSpace(socialStore.Snapshot().DisplayName) == "" {
		if err := socialStore.SetProfile("KnotRoute user", "", ""); err != nil {
			return nil, fmt.Errorf("initialize social profile: %w", err)
		}
	}
	return &Runtime{dataDir: dataDir, configPath: path, cfg: cfg, identity: id, userIdentity: userIdentity, socialStore: socialStore}, nil
}

func loadFiles(path string) (config.Config, *identity.Identity, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return config.Config{}, nil, err
	}
	id, err := identity.Load(cfg.IdentityFile)
	if errors.Is(err, os.ErrNotExist) || (err != nil && strings.Contains(strings.ToLower(err.Error()), "no such file")) {
		id, err = identity.Generate()
		if err == nil {
			err = id.Save(cfg.IdentityFile)
		}
	}
	if err != nil {
		return config.Config{}, nil, fmt.Errorf("load client identity: %w", err)
	}
	return cfg, id, nil
}

func (r *Runtime) Start(ctx context.Context) error {
	r.mu.Lock()
	if r.node != nil {
		r.mu.Unlock()
		return nil
	}
	cfg := r.cfg
	id := r.identity
	runCtx, cancel := context.WithCancel(ctx)
	r.mu.Unlock()

	listener, err := r.startSocialListener()
	if err != nil {
		cancel()
		r.setError(fmt.Errorf("start messenger service: %w", err))
		return err
	}
	chatService := config.Service{
		Name: socialServiceName, Target: listener.Addr().String(), Description: "KnotRoute Messenger",
		Allow: []string{"*"}, Publish: false, Metadata: map[string]string{"kind": "messenger", "protocol": "knotroute-social-v1"},
	}
	for _, svc := range cfg.Services {
		if svc.Name == socialServiceName {
			_ = listener.Close()
			cancel()
			err := fmt.Errorf("service name %q is reserved by the v4 messenger", socialServiceName)
			r.setError(err)
			return err
		}
	}
	cfg.Services = append(cfg.Services, chatService)

	node, err := overlay.New(cfg, id)
	if err != nil {
		_ = listener.Close()
		cancel()
		r.setError(err)
		return err
	}
	if err := node.Start(runCtx); err != nil {
		_ = listener.Close()
		cancel()
		r.setError(err)
		return err
	}

	r.mu.Lock()
	if r.node != nil {
		r.mu.Unlock()
		node.Stop()
		cancel()
		return nil
	}
	r.node = node
	r.cancel = cancel
	r.socialListener = listener
	r.started = time.Now().UTC()
	r.lastError = ""
	r.mu.Unlock()
	go r.serveSocial(runCtx, listener)
	return nil
}

func (r *Runtime) Stop() {
	r.mu.Lock()
	node, cancel, listener := r.node, r.cancel, r.socialListener
	r.node, r.cancel, r.socialListener = nil, nil, nil
	r.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if listener != nil {
		_ = listener.Close()
	}
	if node != nil {
		node.Stop()
	}
	r.socialWG.Wait()
}

func (r *Runtime) Restart(ctx context.Context) error {
	r.Stop()
	cfg, id, err := loadFiles(r.configPath)
	if err != nil {
		r.setError(err)
		return err
	}
	r.mu.Lock()
	r.cfg, r.identity = cfg, id
	r.mu.Unlock()
	return r.Start(ctx)
}

func (r *Runtime) Running() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.node != nil
}

func (r *Runtime) Status() (overlay.Status, bool) {
	r.mu.RLock()
	node := r.node
	r.mu.RUnlock()
	if node == nil {
		return overlay.Status{}, false
	}
	return node.Status(), true
}

func (r *Runtime) Config() config.Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cfg
}

func (r *Runtime) SaveConfig(cfg config.Config) error {
	cfg.Path = r.configPath
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := config.SaveAtomic(r.configPath, cfg); err != nil {
		return err
	}
	loaded, _, err := loadFiles(r.configPath)
	if err != nil {
		return err
	}
	r.mu.Lock()
	r.cfg = loaded
	r.mu.Unlock()
	return nil
}

func (r *Runtime) UpdateNetwork(networkID string, beacons []string, circuitHops int) error {
	cfg := r.Config()
	cfg.NetworkID = strings.TrimSpace(networkID)
	cfg.Discovery.Beacons = append([]string(nil), beacons...)
	if circuitHops > 0 {
		cfg.Privacy.CircuitHops = circuitHops
	}
	return r.SaveConfig(cfg)
}

func (r *Runtime) DataDir() string    { return r.dataDir }
func (r *Runtime) ConfigPath() string { return r.configPath }
func (r *Runtime) NodeDomain() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.node != nil {
		return r.node.Domain()
	}
	if r.identity == nil {
		return ""
	}
	return naming.CanonicalDomain(r.identity.ID)
}

func (r *Runtime) LastError() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.lastError
}

func (r *Runtime) setError(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if err == nil {
		r.lastError = ""
		return
	}
	r.lastError = err.Error()
}

func caProfile(cfg config.Config) certauth.Profile {
	return certauth.Profile{
		ValidityDays: cfg.CA.ValidityDays,
		Subject: certauth.Subject{
			CommonName: cfg.CA.Subject.CommonName, Organization: append([]string(nil), cfg.CA.Subject.Organization...),
			OrganizationalUnit: append([]string(nil), cfg.CA.Subject.OrganizationalUnit...), Country: append([]string(nil), cfg.CA.Subject.Country...),
			Province: append([]string(nil), cfg.CA.Subject.Province...), Locality: append([]string(nil), cfg.CA.Subject.Locality...),
			StreetAddress: append([]string(nil), cfg.CA.Subject.StreetAddress...), PostalCode: append([]string(nil), cfg.CA.Subject.PostalCode...),
		},
	}
}

func (r *Runtime) authority() (*certauth.Authority, error) {
	cfg := r.Config()
	if !cfg.CA.Enabled {
		return nil, errors.New("local CA is disabled")
	}
	return certauth.LoadOrCreateWithProfile(cfg.CA.Directory, caProfile(cfg))
}

func (r *Runtime) CAInfo() (certauth.Info, error) {
	a, err := r.authority()
	if err != nil {
		return certauth.Info{}, err
	}
	return a.Info(), nil
}

func (r *Runtime) RootCAPEM() ([]byte, error) {
	a, err := r.authority()
	if err != nil {
		return nil, err
	}
	return a.RootPEM(), nil
}

func (r *Runtime) InstallCA() error {
	a, err := r.authority()
	if err != nil {
		return err
	}
	return certauth.InstallUserRoot(a)
}

func (r *Runtime) UninstallCA() error {
	a, err := r.authority()
	if err != nil {
		return err
	}
	return certauth.UninstallUserRoot(a)
}

func (r *Runtime) RotateCA() (certauth.Info, error) {
	cfg := r.Config()
	if !cfg.CA.Enabled {
		return certauth.Info{}, errors.New("local CA is disabled")
	}
	if old, err := r.authority(); err == nil {
		_ = certauth.UninstallUserRoot(old)
	}
	a, err := certauth.Regenerate(cfg.CA.Directory, caProfile(cfg))
	if err != nil {
		return certauth.Info{}, err
	}
	return a.Info(), nil
}

// Dial opens a .knot node/service address through the running in-process node.
func (r *Runtime) Dial(ctx context.Context, address string) (net.Conn, error) {
	r.mu.RLock()
	node := r.node
	aliases := append([]naming.Alias(nil), r.cfg.Aliases...)
	r.mu.RUnlock()
	if node == nil {
		return nil, errors.New("KnotRoute node is not running")
	}
	host := strings.TrimSpace(address)
	if h, _, err := net.SplitHostPort(host); err == nil {
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
