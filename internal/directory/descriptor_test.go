package directory

import (
	"github.com/localzet/knotroute/internal/identity"
	"github.com/localzet/knotroute/internal/networkid"
	"github.com/localzet/knotroute/internal/nodeid"
	"github.com/localzet/knotroute/internal/serviceidentity"
	"testing"
	"time"
)

func TestDescriptor(t *testing.T) {
	s, _ := serviceidentity.Generate()
	n, _ := identity.Generate()
	net := networkid.FromSeed("x")
	d, err := New(s, net, []nodeid.ID{n.ID}, 1, time.Minute, nil)
	if err != nil {
		t.Fatal(err)
	}
	id, err := d.Verify(time.Now(), net)
	if err != nil {
		t.Fatal(err)
	}
	if id != s.ID {
		t.Fatal("id mismatch")
	}
}
