package router

import (
	"sort"

	"github.com/localzet/knotroute/internal/nodeid"
	"github.com/localzet/knotroute/internal/protocol"
)

type Route struct {
	Destination nodeid.ID   `json:"-"`
	NextHop     nodeid.ID   `json:"-"`
	Hops        int         `json:"hops"`
	Path        []nodeid.ID `json:"-"`
}

// Compute builds shortest paths using only bidirectional links. A node cannot
// invent a usable edge by advertising another node that does not advertise it.
func Compute(local nodeid.ID, lsas map[nodeid.ID]protocol.LSA) map[nodeid.ID]Route {
	adjacency := make(map[nodeid.ID][]nodeid.ID, len(lsas))
	advertised := make(map[nodeid.ID]map[nodeid.ID]struct{}, len(lsas))
	for id, lsa := range lsas {
		set := map[nodeid.ID]struct{}{}
		for _, raw := range lsa.Neighbors {
			n, err := nodeid.Parse(raw)
			if err == nil && n != id {
				set[n] = struct{}{}
			}
		}
		advertised[id] = set
	}
	for a, ns := range advertised {
		for b := range ns {
			if reverse, ok := advertised[b]; ok {
				if _, ok := reverse[a]; ok {
					adjacency[a] = append(adjacency[a], b)
				}
			}
		}
		sort.Slice(adjacency[a], func(i, j int) bool { return nodeid.Compare(adjacency[a][i], adjacency[a][j]) < 0 })
	}

	routes := map[nodeid.ID]Route{}
	queue := []nodeid.ID{local}
	paths := map[nodeid.ID][]nodeid.ID{local: {local}}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, next := range adjacency[current] {
			if _, seen := paths[next]; seen {
				continue
			}
			path := append(append([]nodeid.ID(nil), paths[current]...), next)
			paths[next] = path
			queue = append(queue, next)
			if next != local {
				routes[next] = Route{Destination: next, NextHop: path[1], Hops: len(path) - 1, Path: path}
			}
		}
	}
	return routes
}
