package overlay

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/localzet/knotroute/internal/certauth"
	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/discovery"
	"github.com/localzet/knotroute/internal/identity"
	"github.com/localzet/knotroute/internal/naming"
	"github.com/localzet/knotroute/internal/networkid"
	"github.com/localzet/knotroute/internal/nodeid"
	"github.com/localzet/knotroute/internal/protocol"
	proxyserver "github.com/localzet/knotroute/internal/proxy"
	"github.com/localzet/knotroute/internal/router"
)

var Version = "3.0.0"

type counters struct {
	bytesSent, bytesReceived, framesSent, framesReceived atomic.Uint64
}

type Node struct {
	cfg     config.Config
	id      *identity.Identity
	network networkid.ID

	ctx       context.Context
	cancel    context.CancelFunc
	startedAt time.Time
	startOnce sync.Once
	stopOnce  sync.Once
	wg        sync.WaitGroup

	serverTLS *tls.Config
	clientTLS *tls.Config

	listenersMu sync.RWMutex
	listeners   []net.Listener
	addresses   []string

	peersMu sync.RWMutex
	peers   map[nodeid.ID]*peer

	topologyMu  sync.RWMutex
	lsas        map[nodeid.ID]protocol.LSA
	routes      map[nodeid.ID]router.Route
	lsaSequence atomic.Uint64

	seenMu      sync.Mutex
	seenPackets map[[16]byte]time.Time

	streamsMu sync.Mutex
	streams   map[[16]byte]*stream
	pending   map[[16]byte]chan openResult

	forwardsMu       sync.RWMutex
	forwardState     []ForwardStatus
	forwardListeners []net.Listener

	eventsMu sync.RWMutex
	events   []Event

	dashboardServer   *http.Server
	proxyGateway      *proxyserver.Gateway
	proxyAddresses    []string
	restartRequested  chan struct{}
	shutdownRequested chan struct{}
	restartOnce       sync.Once
	shutdownOnce      sync.Once
	stats             counters

	discoveryMu sync.Mutex
	discovered  map[nodeid.ID]discovery.Candidate
	dialing     map[nodeid.ID]bool
	directory   directoryState
	rendezvous  rendezvousState
	circuits    circuitState
	ca          *certauth.Authority
}

func New(cfg config.Config, id *identity.Identity) (*Node, error) {
	network, err := cfg.Network()
	if err != nil {
		return nil, err
	}
	cert, err := makeCertificate(id)
	if err != nil {
		return nil, err
	}
	serverTLS := &tls.Config{
		Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13,
		ClientAuth: tls.RequireAnyClientCert,
	}
	clientTLS := &tls.Config{
		Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS13,
		InsecureSkipVerify: true, // self-authenticating key; verified after handshake
	}
	n := &Node{
		cfg: cfg, id: id, network: network, serverTLS: serverTLS, clientTLS: clientTLS,
		peers: map[nodeid.ID]*peer{}, lsas: map[nodeid.ID]protocol.LSA{}, routes: map[nodeid.ID]router.Route{},
		seenPackets: map[[16]byte]time.Time{}, streams: map[[16]byte]*stream{}, pending: map[[16]byte]chan openResult{},
		restartRequested: make(chan struct{}), shutdownRequested: make(chan struct{}), discovered: map[nodeid.ID]discovery.Candidate{}, dialing: map[nodeid.ID]bool{},
	}
	if cfg.CA.Enabled && cfg.CA.Directory != "" && cfg.Path != "" {
		authority, caErr := certauth.LoadOrCreate(cfg.CA.Directory)
		if caErr != nil {
			return nil, fmt.Errorf("initialize local CA: %w", caErr)
		}
		n.ca = authority
	}
	if err := n.initDirectory(); err != nil {
		return nil, err
	}
	n.initRendezvous()
	n.initCircuits()
	return n, nil
}

func makeCertificate(id *identity.Identity) (tls.Certificate, error) {
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return tls.Certificate{}, err
	}
	now := time.Now()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkixName(id.ID.String()),
		NotBefore:    now.Add(-time.Hour), NotAfter: now.AddDate(10, 0, 0),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, ed25519.PublicKey(id.PublicKey), ed25519.PrivateKey(id.PrivateKey))
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("create node certificate: %w", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: id.PrivateKey}, nil
}

// pkixName is kept tiny so node certificates do not leak host metadata.
func pkixName(commonName string) pkix.Name {
	return pkix.Name{CommonName: commonName, Organization: []string{"KnotRoute"}}
}

func (n *Node) Start(parent context.Context) error {
	var startErr error
	n.startOnce.Do(func() {
		n.ctx, n.cancel = context.WithCancel(parent)
		n.startedAt = time.Now().UTC()
		for _, address := range n.cfg.Listen {
			raw, err := net.Listen("tcp", address)
			if err != nil {
				startErr = fmt.Errorf("listen %s: %w", address, err)
				break
			}
			listener := raw
			n.listeners = append(n.listeners, listener)
			n.addresses = append(n.addresses, raw.Addr().String())
		}
		if startErr != nil {
			for _, l := range n.listeners {
				_ = l.Close()
			}
			n.cancel()
			return
		}
		n.addEvent("info", "node started as "+n.id.ID.String())
		for _, listener := range n.listeners {
			n.wg.Add(1)
			go n.acceptLoop(listener)
		}
		n.emitLocalLSA()
		for _, p := range n.cfg.Peers {
			n.wg.Add(1)
			go n.dialLoop(p)
		}
		if n.cfg.Discovery.Enabled {
			n.startDiscovery()
		}
		n.wg.Add(1)
		go n.maintenanceLoop()
		n.wg.Add(1)
		go n.directoryLoop()
		n.startForwards()
		gateway := &proxyserver.Gateway{
			Aliases: n.cfg.Aliases, Direct: n.cfg.Proxy.Direct,
			DefaultHTTP: n.cfg.Proxy.DefaultHTTP, DefaultHTTPS: n.cfg.Proxy.DefaultHTTPS,
			DialOverlay:    n.OpenStream,
			DialService:    n.OpenService,
			Authority:      n.ca,
			InterceptHTTPS: n.cfg.CA.Enabled && n.cfg.CA.InterceptHTTPS,
			Event:          n.addEvent,
		}
		addresses, err := gateway.Start(n.ctx, n.cfg.Proxy.SOCKS, n.cfg.Proxy.HTTP)
		if err != nil {
			startErr = err
			n.Stop()
			return
		}
		n.proxyGateway = gateway
		n.proxyAddresses = addresses
		for _, address := range addresses {
			n.addEvent("info", "local gateway available at "+address)
		}
		if n.cfg.Dashboard != "" {
			if err := n.startDashboard(); err != nil {
				startErr = err
				n.Stop()
				return
			}
		}
	})
	return startErr
}

func (n *Node) Stop() {
	n.stopOnce.Do(func() {
		if n.cancel != nil {
			n.cancel()
		}
		if n.proxyGateway != nil {
			n.proxyGateway.Close()
		}
		n.closeCircuits()
		n.closeInternalServices()
		n.listenersMu.Lock()
		for _, l := range n.listeners {
			_ = l.Close()
		}
		n.listenersMu.Unlock()
		n.forwardsMu.Lock()
		for _, l := range n.forwardListeners {
			_ = l.Close()
		}
		n.forwardsMu.Unlock()
		n.peersMu.RLock()
		peers := make([]*peer, 0, len(n.peers))
		for _, p := range n.peers {
			peers = append(peers, p)
		}
		n.peersMu.RUnlock()
		for _, p := range peers {
			p.close("node stopping")
		}
		n.streamsMu.Lock()
		streams := make([]*stream, 0, len(n.streams))
		for _, s := range n.streams {
			streams = append(streams, s)
		}
		n.streamsMu.Unlock()
		for _, s := range streams {
			s.closeLocal("node stopping", false)
		}
		if n.dashboardServer != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			_ = n.dashboardServer.Shutdown(ctx)
			cancel()
		}
		n.wg.Wait()
	})
}

func (n *Node) ID() nodeid.ID                      { return n.id.ID }
func (n *Node) Domain() string                     { return naming.CanonicalDomain(n.id.ID) }
func (n *Node) RestartRequested() <-chan struct{}  { return n.restartRequested }
func (n *Node) ShutdownRequested() <-chan struct{} { return n.shutdownRequested }
func (n *Node) RequestRestart()                    { n.restartOnce.Do(func() { close(n.restartRequested) }) }
func (n *Node) RequestShutdown()                   { n.shutdownOnce.Do(func() { close(n.shutdownRequested) }) }
func (n *Node) Addresses() []string {
	n.listenersMu.RLock()
	defer n.listenersMu.RUnlock()
	return append([]string(nil), n.addresses...)
}

func (n *Node) advertisedAddresses() []string {
	if len(n.cfg.Advertise) > 0 {
		return append([]string(nil), n.cfg.Advertise...)
	}

	out := make([]string, 0, len(n.Addresses()))
	for _, address := range n.Addresses() {
		host, port, err := net.SplitHostPort(address)
		if err != nil || port == "" {
			continue
		}
		ip := net.ParseIP(host)
		if ip == nil || ip.IsUnspecified() || ip.IsLoopback() {
			continue
		}
		out = append(out, address)
	}
	return out
}

func (n *Node) acceptLoop(listener net.Listener) {
	defer n.wg.Done()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if n.ctx.Err() != nil {
				return
			}
			n.addEvent("warn", "accept failed: "+err.Error())
			continue
		}
		n.wg.Add(1)
		go func() {
			defer n.wg.Done()
			p, err := n.establishPeer(conn, false, nil)
			if err != nil {
				_ = conn.Close()
				if n.ctx.Err() == nil {
					n.addEvent("warn", "inbound peer rejected: "+err.Error())
				}
				return
			}
			n.addEvent("info", "peer "+p.id.Short()+" connected inbound")
			p.run()
		}()
	}
}

func (n *Node) dialLoop(cfgPeer config.Peer) {
	defer n.wg.Done()
	var expected *nodeid.ID
	if cfgPeer.ExpectedID != "" {
		id, _ := naming.ParseNodeReference(cfgPeer.ExpectedID)
		expected = &id
	}
	backoff := time.Second
	for n.ctx.Err() == nil {
		dialer := net.Dialer{Timeout: 8 * time.Second, KeepAlive: 20 * time.Second}
		raw, err := dialer.DialContext(n.ctx, "tcp", cfgPeer.Address)
		if err == nil {
			p, establishErr := n.establishPeer(raw, true, expected)
			if establishErr == nil {
				n.addEvent("info", "peer "+p.id.Short()+" connected via "+cfgPeer.Address)
				backoff = time.Second
				p.run()
				<-p.done
			} else {
				_ = raw.Close()
				if n.ctx.Err() == nil {
					n.addEvent("warn", "dial "+cfgPeer.Address+": "+establishErr.Error())
				}
			}
		} else if n.ctx.Err() == nil {
			n.addEvent("warn", "dial "+cfgPeer.Address+": "+err.Error())
		}
		timer := time.NewTimer(backoff)
		select {
		case <-n.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		if backoff < 30*time.Second {
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		}
	}
}

func (n *Node) registerPeer(p *peer) bool {
	n.peersMu.Lock()
	old := n.peers[p.id]
	if old != nil {
		preferredOutbound := nodeid.Compare(n.id.ID, p.id) < 0
		newPreferred := p.outbound == preferredOutbound
		oldPreferred := old.outbound == preferredOutbound
		if !newPreferred || oldPreferred {
			n.peersMu.Unlock()
			return false
		}
		n.peers[p.id] = p
		n.peersMu.Unlock()
		old.close("replaced duplicate connection")
	} else {
		n.peers[p.id] = p
		n.peersMu.Unlock()
	}
	n.emitLocalLSA()
	n.sendAllLSAs(p)
	if n.cfg.Discovery.Enabled && n.cfg.Discovery.PeerExchange {
		raw, _ := json.Marshal(discovery.Response{Peers: n.pexCandidates()})
		_ = p.send(protocol.FramePEX, raw)
	}
	return true
}

func (n *Node) unregisterPeer(p *peer) {
	n.peersMu.Lock()
	if current := n.peers[p.id]; current == p {
		delete(n.peers, p.id)
	}
	n.peersMu.Unlock()
	n.emitLocalLSA()
}

func (n *Node) peerIDs() []nodeid.ID {
	n.peersMu.RLock()
	defer n.peersMu.RUnlock()
	ids := make([]nodeid.ID, 0, len(n.peers))
	for id := range n.peers {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return nodeid.Compare(ids[i], ids[j]) < 0 })
	return ids
}

func (n *Node) emitLocalLSA() {
	if n.ctx == nil {
		return
	}
	ttl, _ := n.cfg.LSATTL()
	services := make([]protocol.ServiceInfo, 0, len(n.cfg.Services))
	for _, s := range n.cfg.Services {
		services = append(services, protocol.ServiceInfo{Name: s.Name, Description: s.Description})
	}
	lsa, err := protocol.NewLSA(n.id, n.lsaSequence.Add(1), ttl, n.advertisedAddresses(), n.peerIDs(), services)
	if err != nil {
		n.addEvent("error", "create LSA: "+err.Error())
		return
	}
	n.topologyMu.Lock()
	n.lsas[n.id.ID] = lsa
	n.recomputeRoutesLocked()
	n.topologyMu.Unlock()
	raw, _ := json.Marshal(lsa)
	n.broadcast(protocol.FrameLSA, raw, nil)
}

func (n *Node) handleLSA(raw []byte, source *peer) {
	var lsa protocol.LSA
	if err := json.Unmarshal(raw, &lsa); err != nil {
		n.addEvent("warn", "invalid LSA JSON: "+err.Error())
		return
	}
	id, err := lsa.Verify(time.Now())
	if err != nil {
		n.addEvent("warn", "invalid LSA: "+err.Error())
		return
	}
	if id == n.id.ID {
		return
	}
	n.topologyMu.Lock()
	old, exists := n.lsas[id]
	if exists && old.Sequence >= lsa.Sequence {
		n.topologyMu.Unlock()
		return
	}
	n.lsas[id] = lsa
	n.recomputeRoutesLocked()
	n.topologyMu.Unlock()
	n.broadcast(protocol.FrameLSA, raw, source)
}

func (n *Node) sendAllLSAs(p *peer) {
	n.topologyMu.RLock()
	raw := make([][]byte, 0, len(n.lsas))
	for _, lsa := range n.lsas {
		b, _ := json.Marshal(lsa)
		raw = append(raw, b)
	}
	n.topologyMu.RUnlock()
	for _, b := range raw {
		_ = p.send(protocol.FrameLSA, b)
	}
}

func (n *Node) broadcast(typ byte, payload []byte, except *peer) {
	n.peersMu.RLock()
	peers := make([]*peer, 0, len(n.peers))
	for _, p := range n.peers {
		if p != except {
			peers = append(peers, p)
		}
	}
	n.peersMu.RUnlock()
	for _, p := range peers {
		if err := p.send(typ, payload); err != nil {
			p.close("write failed: " + err.Error())
		}
	}
}

func (n *Node) recomputeRoutesLocked() { n.routes = router.Compute(n.id.ID, n.lsas) }

func (n *Node) maintenanceLoop() {
	defer n.wg.Done()
	interval, _ := n.cfg.LSAInterval()
	lsaTicker := time.NewTicker(interval)
	cleanTicker := time.NewTicker(15 * time.Second)
	pingTicker := time.NewTicker(20 * time.Second)
	defer lsaTicker.Stop()
	defer cleanTicker.Stop()
	defer pingTicker.Stop()
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-lsaTicker.C:
			n.emitLocalLSA()
		case <-cleanTicker.C:
			n.cleanupExpired()
		case <-pingTicker.C:
			var nonce [8]byte
			_, _ = rand.Read(nonce[:])
			n.broadcast(protocol.FramePing, nonce[:], nil)
		}
	}
}

func (n *Node) cleanupExpired() {
	now := time.Now()
	n.topologyMu.Lock()
	changed := false
	for id, lsa := range n.lsas {
		if id != n.id.ID && lsa.ExpiresUnix <= now.Unix() {
			delete(n.lsas, id)
			changed = true
		}
	}
	if changed {
		n.recomputeRoutesLocked()
	}
	n.topologyMu.Unlock()
	n.seenMu.Lock()
	for id, seen := range n.seenPackets {
		if now.Sub(seen) > 2*time.Minute {
			delete(n.seenPackets, id)
		}
	}
	n.seenMu.Unlock()
}

func (n *Node) sendPacket(packet protocol.Packet) error {
	if packet.Dst == n.id.ID {
		n.handleLocalPacket(packet)
		return nil
	}
	n.topologyMu.RLock()
	route, ok := n.routes[packet.Dst]
	n.topologyMu.RUnlock()
	if !ok {
		return fmt.Errorf("no route to %s", packet.Dst)
	}
	n.peersMu.RLock()
	p := n.peers[route.NextHop]
	n.peersMu.RUnlock()
	if p == nil {
		return fmt.Errorf("next hop %s is disconnected", route.NextHop)
	}
	return p.sendPacket(packet)
}

func (n *Node) handlePacket(packet protocol.Packet) {
	if packet.Dst != n.id.ID {
		if packet.TTL <= 1 {
			n.stats.framesReceived.Add(0)
			return
		}
		n.seenMu.Lock()
		if _, exists := n.seenPackets[packet.PacketID]; exists {
			n.seenMu.Unlock()
			return
		}
		n.seenPackets[packet.PacketID] = time.Now()
		n.seenMu.Unlock()
		packet.TTL--
		if isStreamControlPacket(packet.Kind) {
			// Topology announcements and stream-control packets can cross during
			// startup. Forward control traffic from a worker so a temporary missing
			// route is retried without blocking this peer's frame reader (and thus
			// without blocking the LSA that may establish the route).
			n.wg.Add(1)
			go func(p protocol.Packet) {
				defer n.wg.Done()
				if err := n.sendPacketWithRetry(p, 5*time.Second); err != nil && n.ctx.Err() == nil {
					n.addEvent("warn", "relay control packet: "+err.Error())
				}
			}(packet)
			return
		}
		if err := n.sendPacket(packet); err != nil {
			n.addEvent("warn", "relay packet: "+err.Error())
		}
		return
	}
	n.handleLocalPacket(packet)
}

func isStreamControlPacket(kind byte) bool {
	switch kind {
	case protocol.PacketOpen, protocol.PacketOpenAck, protocol.PacketError, protocol.PacketReady, protocol.PacketClose:
		return true
	default:
		return false
	}
}

func (n *Node) newPacket(kind byte, dst nodeid.ID, streamID [16]byte, seq uint64, payload []byte) protocol.Packet {
	var packetID [16]byte
	_, _ = rand.Read(packetID[:])
	return protocol.Packet{Version: protocol.ProtocolVersion, Kind: kind, TTL: byte(n.cfg.Routing.MaxHops), Src: n.id.ID, Dst: dst, PacketID: packetID, StreamID: streamID, Seq: seq, Payload: payload}
}

func (n *Node) addEvent(level, message string) {
	n.eventsMu.Lock()
	n.events = append(n.events, Event{Time: time.Now().UTC(), Level: level, Message: message})
	if len(n.events) > 200 {
		n.events = append([]Event(nil), n.events[len(n.events)-200:]...)
	}
	n.eventsMu.Unlock()
}

func (n *Node) Status() Status {
	status := Status{Name: "KnotRoute", Version: Version, NetworkID: n.network.String(), NodeID: n.id.ID.String(), Domain: n.Domain(), ShortID: n.id.ID.Short(), StartedAt: n.startedAt, Listen: n.Addresses(), Peers: []PeerStatus{}, Routes: []RouteStatus{}, Services: []ServiceStatus{}, Forwards: []ForwardStatus{}, Aliases: []AliasStatus{}, Events: []Event{}, BytesSent: n.stats.bytesSent.Load(), BytesReceived: n.stats.bytesReceived.Load(), FramesSent: n.stats.framesSent.Load(), FramesReceived: n.stats.framesReceived.Load()}
	status.Proxy = ProxyStatus{SOCKS: n.cfg.Proxy.SOCKS, HTTP: n.cfg.Proxy.HTTP, Direct: n.cfg.Proxy.Direct, Listeners: append([]string(nil), n.proxyAddresses...)}
	for _, address := range n.proxyAddresses {
		if strings.HasPrefix(address, "socks5://") {
			status.Proxy.SOCKS = strings.TrimPrefix(address, "socks5://")
		}
		if strings.HasPrefix(address, "http://") {
			status.Proxy.HTTP = strings.TrimPrefix(address, "http://")
		}
	}
	if n.cfg.Dashboard != "" {
		status.Proxy.PAC = "http://" + n.cfg.Dashboard + "/proxy.pac"
	}
	n.peersMu.RLock()
	for _, p := range n.peers {
		direction := "inbound"
		if p.outbound {
			direction = "outbound"
		}
		status.Peers = append(status.Peers, PeerStatus{ID: p.id.String(), ShortID: p.id.Short(), Direction: direction, RemoteAddr: p.remoteAddr, Advertise: append([]string(nil), p.advertise...)})
	}
	n.peersMu.RUnlock()
	sort.Slice(status.Peers, func(i, j int) bool { return status.Peers[i].ID < status.Peers[j].ID })
	n.topologyMu.RLock()
	for dest, route := range n.routes {
		r := RouteStatus{Destination: dest.String(), Domain: naming.CanonicalDomain(dest), ShortID: dest.Short(), NextHop: route.NextHop.String(), Hops: route.Hops}
		for _, id := range route.Path {
			r.Path = append(r.Path, id.String())
		}
		if lsa, ok := n.lsas[dest]; ok {
			for _, service := range lsa.Services {
				r.Services = append(r.Services, service.Name)
			}
		}
		status.Routes = append(status.Routes, r)
	}
	n.topologyMu.RUnlock()
	sort.Slice(status.Routes, func(i, j int) bool {
		if status.Routes[i].Hops != status.Routes[j].Hops {
			return status.Routes[i].Hops < status.Routes[j].Hops
		}
		return status.Routes[i].Destination < status.Routes[j].Destination
	})
	domains := n.serviceDomains()
	for _, s := range n.cfg.Services {
		entry := ServiceStatus{Name: s.Name, Target: s.Target, Description: s.Description, Published: s.Publish, Domain: domains[s.Name]}
		if s.Publish {
			n.directory.mu.RLock()
			pub := n.directory.local[s.Name]
			n.directory.mu.RUnlock()
			if pub != nil {
				entry.ServiceID = pub.identity.ID.String()
				for _, intro := range n.activeIntros(pub) {
					entry.Introduction = append(entry.Introduction, intro.String())
				}
			}
		}
		status.Services = append(status.Services, entry)
	}
	for _, a := range n.cfg.Aliases {
		resolved, err := naming.ResolveHost(a.Name+naming.Suffix, n.cfg.Aliases)
		if err != nil {
			continue
		}
		entry := AliasStatus{Name: a.Name, Domain: a.Name + naming.Suffix, Description: a.Description}
		if resolved.Kind == naming.AddressService {
			entry.Kind = "service"
			entry.Target = resolved.ServiceID.String()
		} else {
			entry.Kind = "node"
			entry.Target = resolved.Node.String()
		}
		status.Aliases = append(status.Aliases, entry)
	}
	n.forwardsMu.RLock()
	status.Forwards = append(status.Forwards, n.forwardState...)
	n.forwardsMu.RUnlock()
	n.streamsMu.Lock()
	status.ActiveStreams = len(n.streams)
	n.streamsMu.Unlock()
	n.circuits.mu.RLock()
	status.ActiveCircuits = len(n.circuits.clients) + len(n.circuits.relayIn)
	n.circuits.mu.RUnlock()
	n.directory.mu.RLock()
	status.Descriptors = len(n.directory.descriptors)
	n.directory.mu.RUnlock()
	n.eventsMu.RLock()
	status.Events = append([]Event(nil), n.events...)
	n.eventsMu.RUnlock()
	return status
}

func (n *Node) RouteTo(id nodeid.ID) (router.Route, bool) {
	n.topologyMu.RLock()
	defer n.topologyMu.RUnlock()
	r, ok := n.routes[id]
	return r, ok
}

func (n *Node) service(name string) (config.Service, bool) {
	for _, s := range n.cfg.Services {
		if s.Name == name {
			return s, true
		}
	}
	return config.Service{}, false
}

func (n *Node) allowed(service config.Service, src nodeid.ID) bool {
	if len(service.Allow) == 0 {
		return true
	}
	for _, allowed := range service.Allow {
		if allowed == "*" {
			return true
		}
		id, err := naming.ParseNodeReference(allowed)
		if err == nil && id == src {
			return true
		}
	}
	return false
}
