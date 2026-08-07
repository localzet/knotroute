package main

import (
	"github.com/localzet/knotroute/internal/ops"
	"strings"
	"testing"
)

func TestOnboarding(t *testing.T) {
	n := ops.Network{ID: "kn_abcdefghijklmnopqrstuvwxyz234567", Name: "Prod", Beacons: []string{"https://beacon.example"}}
	out, err := buildOnboarding(n, "android", "", "en")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out.URI, "knotroute://join?") {
		t.Fatalf("bad uri %q", out.URI)
	}
	if !strings.Contains(out.Instructions, n.ID) {
		t.Fatal("network id missing")
	}
}
