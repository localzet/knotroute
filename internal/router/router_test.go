package router

import (
	"testing"

	"github.com/localzet/knotroute/internal/nodeid"
	"github.com/localzet/knotroute/internal/protocol"
)

func fakeID(seed byte) nodeid.ID { var id nodeid.ID; id[0] = seed; return id }
func lsa(id nodeid.ID, neighbors ...nodeid.ID) protocol.LSA {
	out := protocol.LSA{LSABody: protocol.LSABody{NodeID: id.String()}}
	for _, n := range neighbors {
		out.Neighbors = append(out.Neighbors, n.String())
	}
	return out
}

func TestComputeMultiHopAndRejectOneSidedEdge(t *testing.T) {
	a, b, c, d := fakeID(1), fakeID(2), fakeID(3), fakeID(4)
	routes := Compute(a, map[nodeid.ID]protocol.LSA{
		a: lsa(a, b), b: lsa(b, a, c, d), c: lsa(c, b), d: lsa(d),
	})
	r, ok := routes[c]
	if !ok || r.NextHop != b || r.Hops != 2 {
		t.Fatalf("unexpected route: %#v", r)
	}
	if _, ok := routes[d]; ok {
		t.Fatal("accepted one-sided edge")
	}
}
