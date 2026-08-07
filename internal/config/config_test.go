package config

import "testing"

func TestDefaultValid(t *testing.T) {
	cfg := Default()
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestRejectsDuplicateService(t *testing.T) {
	cfg := Default()
	cfg.Services = []Service{{Name: "ssh", Target: "127.0.0.1:22"}, {Name: "ssh", Target: "127.0.0.1:23"}}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected validation error")
	}
}

func TestNormalizeExplicitEmptyDefaults(t *testing.T) {
	cfg := Default()
	cfg.Proxy.DefaultHTTP = ""
	cfg.Proxy.DefaultHTTPS = ""
	cfg.Discovery.Interval = ""
	cfg.Normalize()
	if cfg.Proxy.DefaultHTTP != "http" || cfg.Proxy.DefaultHTTPS != "https" || cfg.Discovery.Interval != "30s" {
		t.Fatalf("unexpected normalized defaults: %+v %+v", cfg.Proxy, cfg.Discovery)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}
