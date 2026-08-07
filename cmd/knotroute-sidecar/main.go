package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/identity"
	"github.com/localzet/knotroute/internal/overlay"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "knotroute-sidecar:", err)
		os.Exit(1)
	}
}

func run() error {
	dataDir := env("KNOTROUTE_DATA_DIR", "/data")
	configPath := env("KNOTROUTE_CONFIG", filepath.Join(dataDir, "knotroute.json"))
	_, statErr := os.Stat(configPath)
	fromEnv := envBool("KNOTROUTE_CONFIG_FROM_ENV", false)
	if fromEnv || errors.Is(statErr, os.ErrNotExist) {
		if err := generateConfig(configPath); err != nil {
			return err
		}
	} else if statErr != nil {
		return statErr
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}
	id, err := identity.Load(cfg.IdentityFile)
	if errors.Is(err, os.ErrNotExist) || (err != nil && strings.Contains(err.Error(), "no such file")) {
		id, err = identity.Generate()
		if err == nil {
			err = id.Save(cfg.IdentityFile)
		}
	}
	if err != nil {
		return err
	}
	node, err := overlay.New(cfg, id)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := node.Start(ctx); err != nil {
		return err
	}
	fmt.Println("KnotRoute sidecar", overlay.Version)
	fmt.Println("node", node.Domain())
	for _, svc := range node.Status().Services {
		if svc.Published {
			fmt.Printf("publishing %s as %s -> %s (waiting for introduction points)\n", svc.Name, svc.Domain, svc.Target)
		}
	}
	go logServiceReadiness(ctx, node)
	healthAddr := env("KNOTROUTE_HEALTH_LISTEN", "0.0.0.0:9090")
	health := &http.Server{Addr: healthAddr, ReadHeaderTimeout: 3 * time.Second}
	health.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := node.Status()
		ready := sidecarReady(status)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/healthz":
			_, _ = fmt.Fprintf(w, `{"ok":true,"ready":%t,"node":%q,"version":%q,"descriptors":%d}`, ready, node.Domain(), overlay.Version, status.Descriptors)
		case "/readyz":
			if !ready {
				w.WriteHeader(http.StatusServiceUnavailable)
			}
			_, _ = fmt.Fprintf(w, `{"ok":%t,"node":%q,"version":%q,"descriptors":%d}`, ready, node.Domain(), overlay.Version, status.Descriptors)
		default:
			http.NotFound(w, r)
		}
	})
	go func() {
		if err := health.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintln(os.Stderr, "health server:", err)
		}
	}()
	<-ctx.Done()
	_ = health.Close()
	node.Stop()
	return nil
}

func generateConfig(path string) error {
	cfg := config.Default()
	cfg.IdentityFile = "identity.json"
	cfg.Dashboard = ""
	cfg.Proxy.HTTP = ""
	cfg.Proxy.SOCKS = ""
	cfg.Proxy.Direct = false
	cfg.CA.Enabled = false
	cfg.Discovery.LAN = false
	cfg.Listen = []string{env("KNOTROUTE_LISTEN", "0.0.0.0:7447")}
	if value := strings.TrimSpace(os.Getenv("KNOTROUTE_NETWORK_ID")); value != "" {
		cfg.NetworkID = value
	}
	cfg.Advertise = splitCSV(os.Getenv("KNOTROUTE_ADVERTISE"))
	cfg.Discovery.Beacons = splitCSV(os.Getenv("KNOTROUTE_BEACONS"))
	if value := strings.TrimSpace(os.Getenv("KNOTROUTE_LAN")); value != "" {
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("KNOTROUTE_LAN: %w", err)
		}
		cfg.Discovery.LAN = parsed
	}
	if raw := strings.TrimSpace(os.Getenv("KNOTROUTE_SERVICES_JSON")); raw != "" {
		if err := json.Unmarshal([]byte(raw), &cfg.Services); err != nil {
			return fmt.Errorf("KNOTROUTE_SERVICES_JSON: %w", err)
		}
	} else {
		name := strings.TrimSpace(os.Getenv("KNOTROUTE_SERVICE_NAME"))
		target := strings.TrimSpace(os.Getenv("KNOTROUTE_SERVICE_TARGET"))
		if name != "" || target != "" {
			if name == "" || target == "" {
				return errors.New("KNOTROUTE_SERVICE_NAME and KNOTROUTE_SERVICE_TARGET must be set together")
			}
			metadata := map[string]string{}
			for key, envName := range map[string]string{
				"title":       "KNOTROUTE_SERVICE_TITLE",
				"description": "KNOTROUTE_SERVICE_DESCRIPTION",
				"tags":        "KNOTROUTE_SERVICE_TAGS",
				"category":    "KNOTROUTE_SERVICE_CATEGORY",
				"scheme":      "KNOTROUTE_SERVICE_SCHEME",
			} {
				if value := strings.TrimSpace(os.Getenv(envName)); value != "" {
					metadata[key] = value
				}
			}
			description := strings.TrimSpace(os.Getenv("KNOTROUTE_SERVICE_DESCRIPTION"))
			cfg.Services = []config.Service{{Name: name, Target: target, Description: description, Publish: true, Allow: []string{"*"}, Metadata: metadata}}
		}
	}
	if len(cfg.Services) == 0 {
		return errors.New("no services configured; set KNOTROUTE_SERVICES_JSON or KNOTROUTE_SERVICE_NAME/TARGET")
	}
	cfg.Normalize()
	for i := range cfg.Services {
		if cfg.Services[i].IdentityFile == "" {
			cfg.Services[i].IdentityFile = filepath.Join("services", cfg.Services[i].Name+".identity.json")
		}
		cfg.Services[i].Publish = true
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return config.Save(path, cfg)
}

func sidecarReady(status overlay.Status) bool {
	published := 0
	for _, svc := range status.Services {
		if !svc.Published {
			continue
		}
		published++
		if len(svc.Introduction) == 0 {
			return false
		}
	}
	return published > 0 && status.Descriptors > 0
}

func logServiceReadiness(ctx context.Context, node *overlay.Node) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	ready := map[string]bool{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			status := node.Status()
			for _, svc := range status.Services {
				if !svc.Published {
					continue
				}
				isReady := len(svc.Introduction) > 0 && status.Descriptors > 0
				if isReady && !ready[svc.ServiceID] {
					fmt.Printf("service %s ready as %s with %d introduction point(s)\n", svc.Name, svc.Domain, len(svc.Introduction))
				}
				ready[svc.ServiceID] = isReady
			}
		}
	}
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
func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func envBool(name string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
