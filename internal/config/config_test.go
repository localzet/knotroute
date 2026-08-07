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
