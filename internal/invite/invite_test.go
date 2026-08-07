package invite

import (
	"github.com/localzet/knotroute/internal/identity"
	"github.com/localzet/knotroute/internal/networkid"
	"testing"
	"time"
)

func TestInviteRoundTrip(t *testing.T) {
	id, _ := identity.Generate()
	nw, _ := networkid.Random()
	i, err := New(id, nw, []string{"https://b.example"}, nil, time.Now())
	if err != nil {
		t.Fatal(err)
	}
	got, err := i.Verify(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got != nw {
		t.Fatal("network mismatch")
	}
}
