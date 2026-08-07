package overlay

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/localzet/knotroute/internal/config"
)

func TestStatusJSONUsesEmptyArraysInsteadOfNull(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config.Default()
	cfg.Proxy.SOCKS = ""
	cfg.Proxy.HTTP = ""
	cfg.Services = nil
	cfg.Forwards = nil
	cfg.Aliases = nil

	node := startTestNode(t, ctx, cfg)
	defer node.Stop()

	payload, err := json.Marshal(node.Status())
	if err != nil {
		t.Fatal(err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}

	for _, key := range []string{"listen", "peers", "routes", "services", "forwards", "aliases", "events"} {
		if _, ok := decoded[key].([]any); !ok {
			t.Fatalf("status.%s must be a JSON array, got %T (%v)", key, decoded[key], decoded[key])
		}
	}

	proxy, ok := decoded["proxy"].(map[string]any)
	if !ok {
		t.Fatalf("status.proxy must be an object, got %T", decoded["proxy"])
	}
	if _, ok := proxy["listeners"].([]any); !ok {
		t.Fatalf("status.proxy.listeners must be a JSON array, got %T (%v)", proxy["listeners"], proxy["listeners"])
	}
}
