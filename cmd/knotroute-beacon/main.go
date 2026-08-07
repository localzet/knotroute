package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/discovery"
	"github.com/localzet/knotroute/internal/identity"
	"github.com/localzet/knotroute/internal/networkid"
	"github.com/localzet/knotroute/internal/overlay"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	addr := flag.String("listen", env("KNOTROUTE_BEACON_LISTEN", "0.0.0.0:8080"), "HTTP listen address")
	ttl := flag.Duration("ttl", envDuration("KNOTROUTE_BEACON_TTL", 2*time.Minute), "peer registration TTL")
	maxPeers := flag.Int("max-network-peers", envInt("KNOTROUTE_BEACON_MAX_NETWORK_PEERS", 10000), "maximum live peers per network")
	rate := flag.Float64("rate", envFloat("KNOTROUTE_BEACON_RATE", 2), "per-IP registration requests per second")
	burst := flag.Int("burst", envInt("KNOTROUTE_BEACON_BURST", 8), "per-IP registration burst")
	relayEnabled := flag.Bool("relay", envBool("KNOTROUTE_BEACON_RELAY", true), "also run a KnotRoute bootstrap relay")
	relayListen := flag.String("relay-listen", env("KNOTROUTE_BEACON_RELAY_LISTEN", "0.0.0.0:7447"), "bootstrap relay listen address")
	relayAdvertise := flag.String("relay-advertise", env("KNOTROUTE_BEACON_RELAY_ADVERTISE", ""), "comma-separated relay endpoints; empty derives the Beacon hostname and relay port")
	dataDir := flag.String("data", env("KNOTROUTE_BEACON_DATA", "/data"), "persistent relay identity directory")
	networkValue := flag.String("network", env("KNOTROUTE_NETWORK_ID", networkid.Default().String()), "network id for the bundled relay")
	flag.Parse()

	server := discovery.NewServer(*ttl, *maxPeers)
	server.SetRateLimit(*rate, *burst)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	var relay *overlay.Node
	if *relayEnabled {
		network, err := networkid.Parse(*networkValue)
		if err != nil {
			return fmt.Errorf("network id: %w", err)
		}
		if err := os.MkdirAll(*dataDir, 0o700); err != nil {
			return fmt.Errorf("create beacon data dir: %w", err)
		}
		identityPath := filepath.Join(*dataDir, "relay.identity.json")
		id, err := loadOrCreateIdentity(identityPath)
		if err != nil {
			return err
		}
		cfg := config.Default()
		cfg.Path = filepath.Join(*dataDir, "relay.runtime.json")
		cfg.NetworkID = network.String()
		cfg.IdentityFile = identityPath
		cfg.Listen = []string{*relayListen}
		cfg.Advertise = splitCSV(*relayAdvertise)
		cfg.Peers = nil
		cfg.Services = nil
		cfg.Forwards = nil
		cfg.Aliases = nil
		cfg.Dashboard = ""
		cfg.Proxy.HTTP = ""
		cfg.Proxy.SOCKS = ""
		cfg.Proxy.Direct = false
		cfg.CA.Enabled = false
		cfg.Discovery.Enabled = false
		if err := cfg.Validate(); err != nil {
			return fmt.Errorf("relay config: %w", err)
		}
		relay, err = overlay.New(cfg, id)
		if err != nil {
			return fmt.Errorf("create bundled relay: %w", err)
		}
		if err := relay.Start(ctx); err != nil {
			return fmt.Errorf("start bundled relay: %w", err)
		}
		_, relayPort, err := net.SplitHostPort(*relayListen)
		if err != nil || relayPort == "" {
			relay.Stop()
			return fmt.Errorf("invalid relay listen address %q", *relayListen)
		}
		if err := server.SetBootstrap(network, id.ID, cfg.Advertise, relayPort); err != nil {
			relay.Stop()
			return fmt.Errorf("configure bootstrap relay: %w", err)
		}
		log.Printf("KnotRoute bootstrap relay %s listening on %s", id.ID.String(), *relayListen)
	}

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Printf("KnotRoute Beacon listening on %s", *addr)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-errCh:
		stop()
		if relay != nil {
			relay.Stop()
		}
		return err
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpServer.Shutdown(shutdownCtx)
	if relay != nil {
		relay.Stop()
	}
	return nil
}

func loadOrCreateIdentity(path string) (*identity.Identity, error) {
	id, err := identity.Load(path)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, os.ErrNotExist) && !strings.Contains(err.Error(), "no such file") {
		return nil, err
	}
	id, err = identity.Generate()
	if err != nil {
		return nil, err
	}
	if err := id.Save(path); err != nil {
		return nil, err
	}
	return id, nil
}

func splitCSV(raw string) []string {
	var out []string
	for _, item := range strings.Split(raw, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

func env(k, d string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return d
}
func envDuration(k string, d time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if x, e := time.ParseDuration(v); e == nil {
			return x
		}
	}
	return d
}
func envInt(k string, d int) int {
	if v := os.Getenv(k); v != "" {
		if x, e := strconv.Atoi(v); e == nil {
			return x
		}
	}
	return d
}
func envFloat(k string, d float64) float64 {
	if v := os.Getenv(k); v != "" {
		if x, e := strconv.ParseFloat(v, 64); e == nil {
			return x
		}
	}
	return d
}
func envBool(k string, d bool) bool {
	if v := os.Getenv(k); v != "" {
		if x, e := strconv.ParseBool(v); e == nil {
			return x
		}
	}
	return d
}
