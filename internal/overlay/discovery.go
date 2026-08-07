package overlay

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"time"

	"github.com/localzet/knotroute/internal/discovery"
	"github.com/localzet/knotroute/internal/nodeid"
	"github.com/localzet/knotroute/internal/protocol"
)

func (n *Node) startDiscovery() {
	for _, candidate := range discovery.LoadCache(n.cfg.Discovery.CacheFile, 7*24*time.Hour) {
		n.considerCandidate(candidate, "cache")
	}
	n.wg.Add(1)
	go func() { defer n.wg.Done(); n.discoveryLoop() }()
	if n.cfg.Discovery.LAN {
		endpoints := discovery.LANEndpoints(n.cfg.Listen, n.cfg.Advertise)
		if len(endpoints) > 0 {
			n.wg.Add(1)
			go func() {
				defer n.wg.Done()
				if err := discovery.RunLAN(n.ctx, n.id, n.network, endpoints, n.cfg.DiscoveryInterval(), func(c discovery.Candidate) { n.considerCandidate(c, "lan") }); err != nil && n.ctx.Err() == nil {
					n.addEvent("warn", "LAN discovery: "+err.Error())
				}
			}()
		}
	}
}

func (n *Node) discoveryLoop() {
	interval := n.cfg.DiscoveryInterval()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	client := discovery.Client{}
	run := func() {
		endpoints := n.advertisedAddresses()
		for _, url := range n.cfg.Discovery.Beacons {
			ctx, cancel := context.WithTimeout(n.ctx, 8*time.Second)
			peers, err := client.Exchange(ctx, url, n.id, n.network, endpoints)
			cancel()
			if err != nil {
				if n.ctx.Err() == nil {
					n.addEvent("warn", "beacon "+url+": "+err.Error())
				}
				continue
			}
			for _, c := range peers {
				n.considerCandidate(c, "beacon")
			}
		}
		if n.cfg.Discovery.PeerExchange {
			n.broadcastPEX()
		}
		n.savePeerCache()
	}
	run()
	for {
		select {
		case <-n.ctx.Done():
			n.savePeerCache()
			return
		case <-ticker.C:
			run()
		}
	}
}

func (n *Node) considerCandidate(c discovery.Candidate, source string) {
	id, err := nodeid.Parse(c.NodeID)
	if err != nil || id == n.id.ID || len(c.Endpoints) == 0 {
		return
	}
	n.peersMu.RLock()
	_, connected := n.peers[id]
	n.peersMu.RUnlock()
	if connected {
		return
	}
	n.discoveryMu.Lock()
	old, exists := n.discovered[id]
	if !exists || c.SeenUnix >= old.SeenUnix {
		n.discovered[id] = c
	}
	if n.dialing[id] {
		n.discoveryMu.Unlock()
		return
	}
	n.dialing[id] = true
	n.discoveryMu.Unlock()
	n.addEvent("info", fmt.Sprintf("discovered peer %s via %s", id.Short(), source))
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		defer func() { n.discoveryMu.Lock(); delete(n.dialing, id); n.discoveryMu.Unlock() }()
		n.dialCandidate(id)
	}()
}

func (n *Node) dialCandidate(id nodeid.ID) {
	n.discoveryMu.Lock()
	c := n.discovered[id]
	n.discoveryMu.Unlock()
	for _, address := range c.Endpoints {
		if n.ctx.Err() != nil {
			return
		}
		dialer := net.Dialer{Timeout: 5 * time.Second, KeepAlive: 20 * time.Second}
		raw, err := dialer.DialContext(n.ctx, "tcp", address)
		if err != nil {
			continue
		}
		p, err := n.establishPeer(raw, true, &id)
		if err != nil {
			_ = raw.Close()
			continue
		}
		n.addEvent("info", "auto-peer "+p.id.Short()+" connected via "+address)
		p.run()
		<-p.done
		return
	}
}

func (n *Node) pexCandidates() []discovery.Candidate {
	now := time.Now().Unix()
	out := []discovery.Candidate{}
	n.peersMu.RLock()
	for id, p := range n.peers {
		if len(p.advertise) > 0 {
			out = append(out, discovery.Candidate{NodeID: id.String(), Endpoints: append([]string(nil), p.advertise...), SeenUnix: now})
		}
	}
	n.peersMu.RUnlock()
	self := n.advertisedAddresses()
	if len(self) > 0 {
		out = append(out, discovery.Candidate{NodeID: n.id.ID.String(), Endpoints: self, SeenUnix: now})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].NodeID < out[j].NodeID })
	if len(out) > 128 {
		out = out[:128]
	}
	return out
}
func (n *Node) broadcastPEX() {
	raw, _ := json.Marshal(discovery.Response{Peers: n.pexCandidates()})
	n.broadcast(protocol.FramePEX, raw, nil)
}
func (n *Node) handlePEX(raw []byte) {
	if !n.cfg.Discovery.Enabled || !n.cfg.Discovery.PeerExchange {
		return
	}
	var r discovery.Response
	if json.Unmarshal(raw, &r) != nil {
		return
	}
	if len(r.Peers) > 128 {
		return
	}
	for _, c := range r.Peers {
		n.considerCandidate(c, "pex")
	}
}
func (n *Node) savePeerCache() {
	if n.cfg.Discovery.CacheFile == "" {
		return
	}
	n.discoveryMu.Lock()
	peers := make([]discovery.Candidate, 0, len(n.discovered))
	for _, c := range n.discovered {
		peers = append(peers, c)
	}
	n.discoveryMu.Unlock()
	if err := discovery.SaveCache(n.cfg.Discovery.CacheFile, peers); err != nil && n.ctx.Err() == nil {
		n.addEvent("warn", "save peer cache: "+err.Error())
	}
}
