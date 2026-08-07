package naming

import (
	"strings"
	"testing"

	"github.com/localzet/knotroute/internal/nodeid"
)

func TestCanonicalRoundTrip(t *testing.T) {
	var id nodeid.ID
	for i := range id {
		id[i] = byte(i + 1)
	}
	domain := CanonicalDomain(id)
	if !strings.HasSuffix(domain, ".knot") {
		t.Fatalf("bad suffix: %s", domain)
	}
	label := strings.TrimSuffix(domain, ".knot")
	got, err := ParseCanonicalLabel(label)
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Fatalf("round trip mismatch")
	}
	resolved, err := ResolveHost("git."+domain, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Node != id || resolved.Service != "git" || !resolved.Canonical {
		t.Fatalf("unexpected resolution: %+v", resolved)
	}
}

func TestChecksumRejectsMutation(t *testing.T) {
	var id nodeid.ID
	label := CanonicalLabel(id)
	replacement := byte('a')
	if label[len(label)-1] == 'a' {
		replacement = 'b'
	}
	mutated := label[:len(label)-1] + string(replacement)
	if _, err := ParseCanonicalLabel(mutated); err == nil {
		t.Fatal("mutated address accepted")
	}
}

func TestAliasResolution(t *testing.T) {
	var id nodeid.ID
	id[0] = 9
	aliases := []Alias{{Name: "localzet", Node: id.String()}}
	got, err := ResolveHost("git.localzet.knot", aliases)
	if err != nil {
		t.Fatal(err)
	}
	if got.Node != id || got.Service != "git" || got.Alias != "localzet" {
		t.Fatalf("unexpected: %+v", got)
	}
}
