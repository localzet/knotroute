package naming

import (
	"github.com/localzet/knotroute/internal/nodeid"
	"github.com/localzet/knotroute/internal/serviceid"
	"testing"
)

func TestCanonicalRoundTrip(t *testing.T) {
	var id nodeid.ID
	for i := range id {
		id[i] = byte(i)
	}
	d := CanonicalDomain(id)
	got, err := ParseNodeReference(d)
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Fatal("node mismatch")
	}
}
func TestServiceCanonicalRoundTrip(t *testing.T) {
	var id serviceid.ID
	for i := range id {
		id[i] = byte(31 - i)
	}
	d := ServiceCanonicalDomain(id)
	r, err := ResolveHost(d, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.Kind != AddressService || r.ServiceID != id {
		t.Fatalf("bad resolve %#v", r)
	}
}
func TestAliasKinds(t *testing.T) {
	var n nodeid.ID
	n[0] = 1
	var s serviceid.ID
	s[0] = 2
	aliases := []Alias{{Name: "host", Node: n.String()}, {Name: "blog", ServiceID: s.String()}}
	r, err := ResolveHost("ssh.host.knot", aliases)
	if err != nil || r.Node != n || r.Service != "ssh" {
		t.Fatalf("node alias: %#v %v", r, err)
	}
	r, err = ResolveHost("blog.knot", aliases)
	if err != nil || r.ServiceID != s {
		t.Fatalf("service alias: %#v %v", r, err)
	}
}
