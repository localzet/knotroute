package main

import (
	"path/filepath"
	"testing"

	"github.com/localzet/knotroute/internal/config"
)

func TestGenerateConfigUsesEnvironmentAsManagedSource(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "knotroute.json")
	t.Setenv("KNOTROUTE_SERVICE_NAME", "web")
	t.Setenv("KNOTROUTE_SERVICE_TARGET", "app:8080")
	t.Setenv("KNOTROUTE_BEACONS", "https://beacon.example.test")
	if err := generateConfig(path); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Discovery.LAN {
		t.Fatal("container sidecar should not enable LAN multicast by default")
	}
	if len(cfg.Discovery.Beacons) != 1 || cfg.Discovery.Beacons[0] != "https://beacon.example.test" {
		t.Fatalf("unexpected beacons: %#v", cfg.Discovery.Beacons)
	}
	if len(cfg.Services) != 1 || cfg.Services[0].Target != "app:8080" || !cfg.Services[0].Publish {
		t.Fatalf("unexpected services: %#v", cfg.Services)
	}
}
