package networkid

import "testing"

func TestRoundTrip(t *testing.T) {
	id := FromSeed("example")
	got, err := Parse(id.String())
	if err != nil {
		t.Fatal(err)
	}
	if got != id {
		t.Fatal("mismatch")
	}
}
