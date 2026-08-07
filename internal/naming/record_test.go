package naming

import (
	"testing"
	"time"

	"github.com/localzet/knotroute/internal/identity"
)

func TestAliasRecord(t *testing.T) {
	id, err := identity.Generate()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Unix(1700000000, 0)
	record, err := SignAliasRecord(id, "localzet", "test", now, 24*time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	alias, err := record.Verify(now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if alias.Name != "localzet" || alias.Node != id.ID.String() {
		t.Fatalf("unexpected alias: %+v", alias)
	}
	record.Name = "attacker"
	if _, err := record.Verify(now.Add(time.Hour)); err == nil {
		t.Fatal("modified record accepted")
	}
}
