package main

import (
	"strings"
	"testing"
)

func TestTargetValidation(t *testing.T) {
	for _, v := range []string{"app:8080", "api-1:443"} {
		if !validTarget(v) {
			t.Fatalf("expected valid %q", v)
		}
	}
	for _, v := range []string{"/tmp:80", "app", "app:99999"} {
		if validTarget(v) {
			t.Fatalf("expected invalid %q", v)
		}
	}
}

func TestManagedImageTag(t *testing.T) {
	cases := map[string]string{
		"":          "latest",
		"dev":       "latest",
		"3.1.0-dev": "latest",
		"edge":      "edge",
		"v3.0.8":    "3.0.8",
		"3.1.0":     "3.1.0",
	}
	for in, want := range cases {
		if got := managedImageTag(in); got != want {
			t.Fatalf("managedImageTag(%q)=%q want %q", in, got, want)
		}
	}
}

func TestNormalizeStatusEmpty(t *testing.T) {
	if got := normalizeStatus("", ""); got != "unknown" {
		t.Fatalf("got %q", got)
	}
	if got := normalizeStatus("running", "Up 1 minute"); got != "running" {
		t.Fatalf("got %q", got)
	}
}

func TestBoundedNumber(t *testing.T) {
	cases := []struct {
		name    string
		input   map[string]any
		want    int
		wantErr bool
	}{
		{name: "default", input: map[string]any{}, want: 18080},
		{name: "valid", input: map[string]any{"http_port": float64(8080)}, want: 8080},
		{name: "fraction", input: map[string]any{"http_port": 8080.5}, wantErr: true},
		{name: "string", input: map[string]any{"http_port": "8080"}, wantErr: true},
		{name: "zero", input: map[string]any{"http_port": float64(0)}, wantErr: true},
		{name: "too high", input: map[string]any{"http_port": float64(70000)}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := boundedNumber(tc.input, "http_port", 18080, 1, 65535)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %d", got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got=%d err=%v want=%d", got, err, tc.want)
			}
		})
	}
}

func TestRenderSidecarCompose(t *testing.T) {
	raw := renderSidecarCompose(
		"ghcr.io/localzet/knotroute-sidecar:edge", "sidecar-web", "kn_abcdefghijklmnopqrstuvwxyz234567",
		"web", "app:8080", "    ports:\n      - \"17447:7447\"\n",
		[]string{"https://beacon-a.example", "https://beacon-b.example"}, "node.example:17447", "project_private",
	)
	for _, want := range []string{
		`KNOTROUTE_CONFIG_FROM_ENV: "true"`,
		`KNOTROUTE_BEACONS: "https://beacon-a.example,https://beacon-b.example"`,
		`KNOTROUTE_SERVICE_TARGET: "app:8080"`,
		`- "17447:7447"`,
		`name: "project_private"`,
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("compose missing %q:\n%s", want, raw)
		}
	}
	if got := strings.Count(raw, "      - target\n"); got != 1 {
		t.Fatalf("target network attached %d times:\n%s", got, raw)
	}
}

func TestRenderBeaconComposeUsesAdvertisedRelayPort(t *testing.T) {
	raw := renderBeaconCompose(
		"ghcr.io/localzet/knotroute-beacon:edge", "beacon-a", "kn_abcdefghijklmnopqrstuvwxyz234567",
		"https://beacon.example", 18080, 17447, "relay.example:17447",
	)
	for _, want := range []string{
		`- "18080:8080"`,
		`- "17447:7447"`,
		`KNOTROUTE_BEACON_RELAY_ADVERTISE: "relay.example:17447"`,
		`io.knotroute.public-url: "https://beacon.example"`,
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("compose missing %q:\n%s", want, raw)
		}
	}
}
