package overlay

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/nodeid"
	"github.com/localzet/knotroute/internal/protocol"
	"github.com/localzet/knotroute/internal/router"
)

type circuitKey struct {
	peer nodeid.ID
	id   uint64
}
type circuitHop struct {
	id               nodeid.ID
	fwdKey, revKey   []byte
	sendSeq, recvSeq uint64
}
type relayResponse struct {
	cmd  byte
	body []byte
	err  error
}

type clientCircuit struct {
	node      *Node
	id        uint64
	first     *peer
	path      []nodeid.ID
	mu        sync.Mutex
	hops      []circuitHop
	responses chan relayResponse
	done      chan struct{}
	closeOnce sync.Once
}

type relayCircuit struct {
	node           *Node
	incoming       *peer
	inID           uint64
	fwdKey, revKey []byte
	mu             sync.Mutex
	fwdSeq, revSeq uint64
	outgoing       *peer
	outID          uint64
	app            net.Conn
	closed         bool
}

type createResult struct {
	payload []byte
	err     error
}
type circuitState struct {
	mu       sync.RWMutex
	relayIn  map[circuitKey]*relayCircuit
	relayOut map[circuitKey]*relayCircuit
	clients  map[uint64]*clientCircuit
	pending  map[circuitKey]chan createResult
}

func (n *Node) initCircuits() {
	n.circuits.relayIn = map[circuitKey]*relayCircuit{}
	n.circuits.relayOut = map[circuitKey]*relayCircuit{}
	n.circuits.clients = map[uint64]*clientCircuit{}
	n.circuits.pending = map[circuitKey]chan createResult{}
}

func deriveCircuitHop(priv *ecdh.PrivateKey, peerPub, clientNonce, serverNonce []byte) ([]byte, []byte, error) {
	pub, err := ecdh.X25519().NewPublicKey(peerPub)
	if err != nil {
		return nil, nil, err
	}
	shared, err := priv.ECDH(pub)
	if err != nil {
		return nil, nil, err
	}
	salt := append(append([]byte(nil), clientNonce...), serverNonce...)
	m := hkdfSHA256(shared, salt, []byte("knotroute/circuit-hop/v1"), 64)
	return m[:32], m[32:], nil
}
func sealCircuitLayer(key []byte, seq uint64, plain []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	var sb [8]byte
	binary.BigEndian.PutUint64(sb[:], seq)
	aad := append([]byte("knotroute/circuit-layer/v1|"), sb[:]...)
	ct := aead.Seal(nil, makeNonce(key, seq), plain, aad)
	out := make([]byte, 8+len(ct))
	copy(out[:8], sb[:])
	copy(out[8:], ct)
	return out, nil
}
func openCircuitLayer(key []byte, want uint64, in []byte) ([]byte, error) {
	if len(in) < 8 {
		return nil, errors.New("circuit layer truncated")
	}
	seq := binary.BigEndian.Uint64(in[:8])
	if seq != want {
		return nil, fmt.Errorf("circuit sequence mismatch: got %d want %d", seq, want)
	}
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	var sb [8]byte
	binary.BigEndian.PutUint64(sb[:], seq)
	aad := append([]byte("knotroute/circuit-layer/v1|"), sb[:]...)
	return aead.Open(nil, makeNonce(key, seq), in[8:], aad)
}
func randomCircuitID() uint64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	v := binary.BigEndian.Uint64(b[:])
	if v == 0 {
		v = 1
	}
	return v
}

func (n *Node) handleCircuitCell(p *peer, cell protocol.CircuitCell) {
	key := circuitKey{peer: p.id, id: cell.CircuitID}
	switch cell.Kind {
	case protocol.CircuitCreate:
		n.handleCircuitCreate(p, cell)
	case protocol.CircuitCreated:
		n.circuits.mu.RLock()
		ch := n.circuits.pending[key]
		var cc *clientCircuit
		if c := n.circuits.clients[cell.CircuitID]; c != nil && c.first.id == p.id {
			cc = c
		}
		n.circuits.mu.RUnlock()
		if ch != nil {
			select {
			case ch <- createResult{payload: cell.Payload}:
			default:
			}
			return
		}
		if cc != nil {
			select {
			case cc.responses <- relayResponse{cmd: protocol.CircuitCreated, body: cell.Payload}:
			default:
			}
			return
		}
	case protocol.CircuitRelay:
		n.circuits.mu.RLock()
		rcIn := n.circuits.relayIn[key]
		rcOut := n.circuits.relayOut[key]
		cc := n.circuits.clients[cell.CircuitID]
		n.circuits.mu.RUnlock()
		if rcIn != nil {
			n.handleCircuitForward(rcIn, cell.Payload)
			return
		}
		if rcOut != nil {
			n.handleCircuitReverse(rcOut, cell.Payload)
			return
		}
		if cc != nil && cc.first.id == p.id {
			cc.handleReverse(cell.Payload)
		}
	case protocol.CircuitDestroy:
		n.destroyCircuitByKey(key)
	}
}

func (n *Node) handleCircuitCreate(p *peer, cell protocol.CircuitCell) {
	if len(cell.Payload) != 64 {
		return
	}
	clientPub := cell.Payload[:32]
	clientNonce := cell.Payload[32:]
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return
	}
	serverNonce := make([]byte, 32)
	if _, err = rand.Read(serverNonce); err != nil {
		return
	}
	fwd, rev, err := deriveCircuitHop(priv, clientPub, clientNonce, serverNonce)
	if err != nil {
		return
	}
	key := circuitKey{peer: p.id, id: cell.CircuitID}
	rc := &relayCircuit{node: n, incoming: p, inID: cell.CircuitID, fwdKey: fwd, revKey: rev}
	n.circuits.mu.Lock()
	if n.circuits.relayIn[key] != nil {
		n.circuits.mu.Unlock()
		return
	}
	n.circuits.relayIn[key] = rc
	n.circuits.mu.Unlock()
	payload := append(append([]byte(nil), priv.PublicKey().Bytes()...), serverNonce...)
	_ = p.sendCircuit(protocol.CircuitCell{Kind: protocol.CircuitCreated, CircuitID: cell.CircuitID, Payload: payload})
}

func (n *Node) handleCircuitForward(rc *relayCircuit, payload []byte) {
	rc.mu.Lock()
	if rc.closed {
		rc.mu.Unlock()
		return
	}
	plain, err := openCircuitLayer(rc.fwdKey, rc.fwdSeq, payload)
	if err == nil {
		rc.fwdSeq++
	}
	out := rc.outgoing
	outID := rc.outID
	rc.mu.Unlock()
	if err != nil {
		n.closeRelayCircuit(rc)
		return
	}
	if out != nil {
		_ = out.sendCircuit(protocol.CircuitCell{Kind: protocol.CircuitRelay, CircuitID: outID, Payload: plain})
		return
	}
	cmd, body, err := protocol.ParseRelayPayload(plain)
	if err != nil {
		n.closeRelayCircuit(rc)
		return
	}
	switch cmd {
	case protocol.RelayExtend:
		n.extendCircuit(rc, body)
	case protocol.RelayOpen:
		n.openCircuitTarget(rc, body)
	case protocol.RelayData:
		rc.mu.Lock()
		app := rc.app
		rc.mu.Unlock()
		if app != nil {
			if err := writeAll(app, body); err != nil {
				n.closeRelayCircuit(rc)
			}
		}
	case protocol.RelayClose:
		n.closeRelayCircuit(rc)
	}
}
func (n *Node) handleCircuitReverse(rc *relayCircuit, payload []byte) {
	rc.mu.Lock()
	if rc.closed {
		rc.mu.Unlock()
		return
	}
	seq := rc.revSeq
	rc.revSeq++
	key := append([]byte(nil), rc.revKey...)
	incoming := rc.incoming
	inID := rc.inID
	rc.mu.Unlock()
	wrapped, err := sealCircuitLayer(key, seq, payload)
	if err != nil {
		n.closeRelayCircuit(rc)
		return
	}
	_ = incoming.sendCircuit(protocol.CircuitCell{Kind: protocol.CircuitRelay, CircuitID: inID, Payload: wrapped})
}
func (n *Node) sendRelayReverse(rc *relayCircuit, cmd byte, body []byte) error {
	rc.mu.Lock()
	if rc.closed {
		rc.mu.Unlock()
		return net.ErrClosed
	}
	seq := rc.revSeq
	rc.revSeq++
	key := append([]byte(nil), rc.revKey...)
	p := rc.incoming
	cid := rc.inID
	rc.mu.Unlock()
	wrapped, err := sealCircuitLayer(key, seq, protocol.RelayPayload(cmd, body))
	if err != nil {
		return err
	}
	return p.sendCircuit(protocol.CircuitCell{Kind: protocol.CircuitRelay, CircuitID: cid, Payload: wrapped})
}

func (n *Node) extendCircuit(rc *relayCircuit, body []byte) {
	if len(body) != 96 {
		_ = n.sendRelayReverse(rc, protocol.RelayOpenError, []byte("invalid EXTEND"))
		return
	}
	var next nodeid.ID
	copy(next[:], body[:32])
	n.peersMu.RLock()
	p := n.peers[next]
	n.peersMu.RUnlock()
	if p == nil {
		_ = n.sendRelayReverse(rc, protocol.RelayOpenError, []byte("next hop unavailable"))
		return
	}
	cid := randomCircuitID()
	key := circuitKey{peer: p.id, id: cid}
	ch := make(chan createResult, 1)
	n.circuits.mu.Lock()
	n.circuits.pending[key] = ch
	n.circuits.mu.Unlock()
	createPayload := append([]byte(nil), body[32:]...)
	if err := p.sendCircuit(protocol.CircuitCell{Kind: protocol.CircuitCreate, CircuitID: cid, Payload: createPayload}); err != nil {
		n.circuits.mu.Lock()
		delete(n.circuits.pending, key)
		n.circuits.mu.Unlock()
		_ = n.sendRelayReverse(rc, protocol.RelayOpenError, []byte(err.Error()))
		return
	}
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		timer := time.NewTimer(n.cfg.CircuitTimeout())
		defer timer.Stop()
		var res createResult
		select {
		case res = <-ch:
		case <-timer.C:
			res.err = errors.New("extend timed out")
		case <-n.ctx.Done():
			res.err = n.ctx.Err()
		}
		n.circuits.mu.Lock()
		delete(n.circuits.pending, key)
		n.circuits.mu.Unlock()
		if res.err != nil {
			_ = n.sendRelayReverse(rc, protocol.RelayOpenError, []byte(res.err.Error()))
			return
		}
		rc.mu.Lock()
		if rc.closed {
			rc.mu.Unlock()
			return
		}
		rc.outgoing = p
		rc.outID = cid
		rc.mu.Unlock()
		n.circuits.mu.Lock()
		n.circuits.relayOut[key] = rc
		n.circuits.mu.Unlock()
		_ = n.sendRelayReverse(rc, protocol.RelayExtended, res.payload)
	}()
}

func (n *Node) openCircuitTarget(rc *relayCircuit, body []byte) {
	if len(body) < 2 {
		return
	}
	ln := int(binary.BigEndian.Uint16(body[:2]))
	if ln <= 0 || ln > len(body)-2 {
		return
	}
	name := string(body[2 : 2+ln])
	var conn net.Conn
	if internal, ok := n.internalService(name, nodeid.ID{}); ok {
		conn = internal
	} else {
		svc, ok := n.service(name)
		if !ok || !n.allowedAnonymous(svc) {
			_ = n.sendRelayReverse(rc, protocol.RelayOpenError, []byte("service unavailable for anonymous circuit"))
			return
		}
		c, err := n.dialServiceTarget(n.ctx, svc.Target)
		if err != nil {
			_ = n.sendRelayReverse(rc, protocol.RelayOpenError, []byte(err.Error()))
			return
		}
		conn = c
	}
	rc.mu.Lock()
	if rc.closed {
		rc.mu.Unlock()
		_ = conn.Close()
		return
	}
	rc.app = conn
	rc.mu.Unlock()
	if err := n.sendRelayReverse(rc, protocol.RelayOpenOK, nil); err != nil {
		n.closeRelayCircuit(rc)
		return
	}
	n.wg.Add(1)
	go func() {
		defer n.wg.Done()
		buf := make([]byte, 32*1024)
		for {
			nr, err := conn.Read(buf)
			if nr > 0 {
				if e := n.sendRelayReverse(rc, protocol.RelayData, buf[:nr]); e != nil {
					return
				}
			}
			if err != nil {
				_ = n.sendRelayReverse(rc, protocol.RelayClose, nil)
				n.closeRelayCircuit(rc)
				return
			}
		}
	}()
}
func (n *Node) allowedAnonymous(svc config.Service) bool {
	for _, a := range svc.Allow {
		if a == "*" {
			return true
		}
	}
	return false
}

func (n *Node) destroyCircuitByKey(key circuitKey) {
	n.circuits.mu.RLock()
	rc := n.circuits.relayIn[key]
	if rc == nil {
		rc = n.circuits.relayOut[key]
	}
	cc := n.circuits.clients[key.id]
	n.circuits.mu.RUnlock()
	if rc != nil {
		n.closeRelayCircuit(rc)
		return
	}
	if cc != nil {
		cc.closeLocal(false)
	}
}
func (n *Node) closeRelayCircuit(rc *relayCircuit) {
	rc.mu.Lock()
	if rc.closed {
		rc.mu.Unlock()
		return
	}
	rc.closed = true
	app := rc.app
	out := rc.outgoing
	outID := rc.outID
	incoming := rc.incoming
	inID := rc.inID
	rc.mu.Unlock()
	if app != nil {
		_ = app.Close()
	}
	n.circuits.mu.Lock()
	delete(n.circuits.relayIn, circuitKey{peer: incoming.id, id: inID})
	if out != nil {
		delete(n.circuits.relayOut, circuitKey{peer: out.id, id: outID})
	}
	n.circuits.mu.Unlock()
	if out != nil {
		_ = out.sendCircuit(protocol.CircuitCell{Kind: protocol.CircuitDestroy, CircuitID: outID})
	}
}

func (cc *clientCircuit) sendRelay(cmd byte, body []byte) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()
	if len(cc.hops) == 0 {
		return errors.New("circuit has no hops")
	}
	payload := protocol.RelayPayload(cmd, body)
	for i := len(cc.hops) - 1; i >= 0; i-- {
		wrapped, err := sealCircuitLayer(cc.hops[i].fwdKey, cc.hops[i].sendSeq, payload)
		if err != nil {
			return err
		}
		cc.hops[i].sendSeq++
		payload = wrapped
	}
	return cc.first.sendCircuit(protocol.CircuitCell{Kind: protocol.CircuitRelay, CircuitID: cc.id, Payload: payload})
}
func (cc *clientCircuit) handleReverse(payload []byte) {
	cc.mu.Lock()
	for i := 0; i < len(cc.hops); i++ {
		plain, err := openCircuitLayer(cc.hops[i].revKey, cc.hops[i].recvSeq, payload)
		if err != nil {
			cc.mu.Unlock()
			cc.closeLocal(false)
			select {
			case cc.responses <- relayResponse{err: err}:
			default:
			}
			return
		}
		cc.hops[i].recvSeq++
		payload = plain
	}
	cc.mu.Unlock()
	cmd, body, err := protocol.ParseRelayPayload(payload)
	if err != nil {
		select {
		case cc.responses <- relayResponse{err: err}:
		default:
		}
		return
	}
	select {
	case cc.responses <- relayResponse{cmd: cmd, body: body}:
	case <-cc.done:
	}
}
func (cc *clientCircuit) wait(ctx context.Context, cmds ...byte) (relayResponse, error) {
	allowed := map[byte]bool{}
	for _, c := range cmds {
		allowed[c] = true
	}
	timer := time.NewTimer(cc.node.cfg.CircuitTimeout())
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return relayResponse{}, ctx.Err()
		case <-cc.done:
			return relayResponse{}, net.ErrClosed
		case <-timer.C:
			return relayResponse{}, errors.New("circuit operation timed out")
		case r := <-cc.responses:
			if r.err != nil {
				return r, r.err
			}
			if r.cmd == protocol.RelayOpenError {
				return r, errors.New(string(r.body))
			}
			if allowed[r.cmd] {
				return r, nil
			}
		}
	}
}
func (cc *clientCircuit) closeLocal(send bool) {
	cc.closeOnce.Do(func() {
		if send {
			_ = cc.first.sendCircuit(protocol.CircuitCell{Kind: protocol.CircuitDestroy, CircuitID: cc.id})
		}
		cc.node.circuits.mu.Lock()
		delete(cc.node.circuits.clients, cc.id)
		cc.node.circuits.mu.Unlock()
		close(cc.done)
	})
}

func (n *Node) buildCircuit(ctx context.Context, path []nodeid.ID) (*clientCircuit, error) {
	if len(path) < 2 || path[0] != n.id.ID {
		return nil, errors.New("invalid circuit path")
	}
	n.peersMu.RLock()
	first := n.peers[path[1]]
	n.peersMu.RUnlock()
	if first == nil {
		return nil, errors.New("first circuit hop is disconnected")
	}
	cid := randomCircuitID()
	cc := &clientCircuit{node: n, id: cid, first: first, path: append([]nodeid.ID(nil), path...), responses: make(chan relayResponse, 128), done: make(chan struct{})}
	n.circuits.mu.Lock()
	for n.circuits.clients[cid] != nil {
		cid = randomCircuitID()
		cc.id = cid
	}
	n.circuits.clients[cid] = cc
	n.circuits.mu.Unlock()
	fail := true
	defer func() {
		if fail {
			cc.closeLocal(true)
		}
	}()
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, 32)
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	payload := append(append([]byte(nil), priv.PublicKey().Bytes()...), nonce...)
	if err = first.sendCircuit(protocol.CircuitCell{Kind: protocol.CircuitCreate, CircuitID: cid, Payload: payload}); err != nil {
		return nil, err
	}
	r, err := cc.wait(ctx, protocol.CircuitCreated)
	if err != nil {
		return nil, err
	}
	if len(r.body) != 64 {
		return nil, errors.New("invalid CREATED payload")
	}
	fwd, rev, err := deriveCircuitHop(priv, r.body[:32], nonce, r.body[32:])
	if err != nil {
		return nil, err
	}
	cc.mu.Lock()
	cc.hops = append(cc.hops, circuitHop{id: path[1], fwdKey: fwd, revKey: rev})
	cc.mu.Unlock()
	for i := 2; i < len(path); i++ {
		priv, err = ecdh.X25519().GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}
		nonce = make([]byte, 32)
		if _, err = rand.Read(nonce); err != nil {
			return nil, err
		}
		body := make([]byte, 96)
		copy(body[:32], path[i][:])
		copy(body[32:64], priv.PublicKey().Bytes())
		copy(body[64:], nonce)
		if err = cc.sendRelay(protocol.RelayExtend, body); err != nil {
			return nil, err
		}
		r, err = cc.wait(ctx, protocol.RelayExtended)
		if err != nil {
			return nil, fmt.Errorf("extend to %s: %w", path[i].Short(), err)
		}
		if len(r.body) != 64 {
			return nil, errors.New("invalid EXTENDED payload")
		}
		fwd, rev, err = deriveCircuitHop(priv, r.body[:32], nonce, r.body[32:])
		if err != nil {
			return nil, err
		}
		cc.mu.Lock()
		cc.hops = append(cc.hops, circuitHop{id: path[i], fwdKey: fwd, revKey: rev})
		cc.mu.Unlock()
	}
	fail = false
	return cc, nil
}

func (n *Node) circuitPathTo(dst nodeid.ID) ([]nodeid.ID, error) {
	n.topologyMu.RLock()
	shortest, ok := n.routes[dst]
	lsas := make(map[nodeid.ID]protocol.LSA, len(n.lsas))
	for id, l := range n.lsas {
		lsas[id] = l
	}
	n.topologyMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("no route to %s", dst)
	}
	best := append([]nodeid.ID(nil), shortest.Path...)
	min := n.cfg.Privacy.CircuitHops
	if len(best)-1 >= min {
		return best, nil
	}
	candidates := make([]nodeid.ID, 0, len(lsas))
	for id := range lsas {
		if id != n.id.ID && id != dst {
			candidates = append(candidates, id)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return nodeid.Compare(candidates[i], candidates[j]) < 0 })
	for _, way := range candidates {
		r1 := router.Compute(n.id.ID, lsas)[way]
		r2map := router.Compute(way, lsas)
		r2, ok := r2map[dst]
		if r1.Path == nil || !ok {
			continue
		}
		combined := append(append([]nodeid.ID(nil), r1.Path...), r2.Path[1:]...)
		seen := map[nodeid.ID]bool{}
		valid := true
		for _, id := range combined {
			if seen[id] {
				valid = false
				break
			}
			seen[id] = true
		}
		if valid && len(combined)-1 >= min && len(combined)-1 <= n.cfg.Routing.MaxHops {
			return combined, nil
		}
	}
	return best, nil
}

func (n *Node) OpenCircuitStream(ctx context.Context, dst nodeid.ID, service string) (net.Conn, error) {
	if dst == n.id.ID {
		if c, ok := n.internalService(service, n.id.ID); ok {
			return c, nil
		}
		return n.openLocalNamedService(ctx, service)
	}
	path, err := n.circuitPathTo(dst)
	if err != nil {
		return nil, err
	}
	cc, err := n.buildCircuit(ctx, path)
	if err != nil {
		return nil, err
	}
	body := make([]byte, 2+len(service))
	if len(service) > 65535 {
		cc.closeLocal(true)
		return nil, errors.New("service name too long")
	}
	binary.BigEndian.PutUint16(body[:2], uint16(len(service)))
	copy(body[2:], service)
	if err := cc.sendRelay(protocol.RelayOpen, body); err != nil {
		cc.closeLocal(true)
		return nil, err
	}
	if _, err := cc.wait(ctx, protocol.RelayOpenOK); err != nil {
		cc.closeLocal(true)
		return nil, err
	}
	n.addEvent("info", fmt.Sprintf("opened %d-hop onion circuit to %s/%s", len(path)-1, dst.Short(), service))
	return &circuitConn{circuit: cc}, nil
}

type circuitConn struct {
	circuit  *clientCircuit
	mu       sync.Mutex
	buf      []byte
	deadline time.Time
	closed   bool
}

func (c *circuitConn) Read(p []byte) (int, error) {
	c.mu.Lock()
	if len(c.buf) > 0 {
		n := copy(p, c.buf)
		c.buf = c.buf[n:]
		c.mu.Unlock()
		return n, nil
	}
	deadline := c.deadline
	closed := c.closed
	c.mu.Unlock()
	if closed {
		return 0, net.ErrClosed
	}
	var timer <-chan time.Time
	if !deadline.IsZero() {
		d := time.Until(deadline)
		if d <= 0 {
			return 0, circuitTimeout{}
		}
		t := time.NewTimer(d)
		defer t.Stop()
		timer = t.C
	}
	for {
		select {
		case <-c.circuit.done:
			return 0, io.EOF
		case <-timer:
			return 0, circuitTimeout{}
		case r := <-c.circuit.responses:
			if r.err != nil {
				return 0, r.err
			}
			switch r.cmd {
			case protocol.RelayData:
				n := copy(p, r.body)
				if n < len(r.body) {
					c.mu.Lock()
					c.buf = append(c.buf, r.body[n:]...)
					c.mu.Unlock()
				}
				return n, nil
			case protocol.RelayClose:
				return 0, io.EOF
			case protocol.RelayOpenError:
				return 0, errors.New(string(r.body))
			}
		}
	}
}
func (c *circuitConn) Write(p []byte) (int, error) {
	c.mu.Lock()
	closed := c.closed
	deadline := c.deadline
	c.mu.Unlock()
	if closed {
		return 0, net.ErrClosed
	}
	if !deadline.IsZero() && time.Now().After(deadline) {
		return 0, circuitTimeout{}
	}
	total := 0
	for len(p) > 0 {
		n := len(p)
		if n > 24*1024 {
			n = 24 * 1024
		}
		if err := c.circuit.sendRelay(protocol.RelayData, p[:n]); err != nil {
			return total, err
		}
		total += n
		p = p[n:]
	}
	return total, nil
}
func (c *circuitConn) Close() error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()
	_ = c.circuit.sendRelay(protocol.RelayClose, nil)
	c.circuit.closeLocal(true)
	return nil
}
func (c *circuitConn) LocalAddr() net.Addr  { return circuitAddr("knotroute-local") }
func (c *circuitConn) RemoteAddr() net.Addr { return circuitAddr("knotroute-circuit") }
func (c *circuitConn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	c.deadline = t
	c.mu.Unlock()
	return nil
}
func (c *circuitConn) SetReadDeadline(t time.Time) error  { return c.SetDeadline(t) }
func (c *circuitConn) SetWriteDeadline(t time.Time) error { return c.SetDeadline(t) }

type circuitAddr string

func (a circuitAddr) Network() string { return "knotroute" }
func (a circuitAddr) String() string  { return string(a) }

type circuitTimeout struct{}

func (circuitTimeout) Error() string   { return "i/o timeout" }
func (circuitTimeout) Timeout() bool   { return true }
func (circuitTimeout) Temporary() bool { return true }

func (n *Node) closeCircuits() {
	n.circuits.mu.RLock()
	clients := make([]*clientCircuit, 0, len(n.circuits.clients))
	for _, c := range n.circuits.clients {
		clients = append(clients, c)
	}
	relays := make([]*relayCircuit, 0, len(n.circuits.relayIn))
	for _, r := range n.circuits.relayIn {
		relays = append(relays, r)
	}
	n.circuits.mu.RUnlock()
	for _, c := range clients {
		c.closeLocal(true)
	}
	for _, r := range relays {
		n.closeRelayCircuit(r)
	}
}
