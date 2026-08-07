package overlay

import (
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"sort"
	"sync"
	"time"

	"github.com/localzet/knotroute/internal/nodeid"
	"github.com/localzet/knotroute/internal/serviceid"
)

const (
	internalIntroRegister = "kr-intro-register"
	internalIntroduce     = "kr-introduce"
	internalRendezvous    = "kr-rendezvous"
)

type introRegistrationMessage struct {
	NetworkID   string `json:"network_id"`
	ServiceID   string `json:"service_id"`
	PublicKey   string `json:"public_key"`
	IntroNode   string `json:"intro_node"`
	ExpiresUnix int64  `json:"expires_unix"`
	Signature   string `json:"signature"`
}
type introduceRequest struct {
	ServiceID       string `json:"service_id"`
	Rendezvous      string `json:"rendezvous"`
	Cookie          string `json:"cookie"`
	ClientEphemeral string `json:"client_ephemeral"`
	ClientNonce     string `json:"client_nonce"`
	TimeUnix        int64  `json:"time_unix"`
}
type simpleAck struct {
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}
type rendezvousHello struct {
	Role   string `json:"role"`
	Cookie string `json:"cookie"`
}
type introRegistration struct {
	conn    net.Conn
	mu      sync.Mutex
	expires time.Time
	source  nodeid.ID
}
type rendezvousWait struct {
	role    string
	conn    net.Conn
	done    chan struct{}
	expires time.Time
	source  nodeid.ID
}
type introSession struct {
	node nodeid.ID
	conn net.Conn
	done chan struct{}
}

type rendezvousState struct {
	internalMu    sync.Mutex
	internalConns map[net.Conn]struct{}
	introMu       sync.Mutex
	registrations map[serviceid.ID]*introRegistration
	rvMu          sync.Mutex
	waiting       map[string]*rendezvousWait
}

func (n *Node) initRendezvous() {
	n.rendezvous.internalConns = map[net.Conn]struct{}{}
	n.rendezvous.registrations = map[serviceid.ID]*introRegistration{}
	n.rendezvous.waiting = map[string]*rendezvousWait{}
	n.directory.mu.Lock()
	for _, s := range n.directory.local {
		s.introSessions = map[nodeid.ID]*introSession{}
		s.introDialing = map[nodeid.ID]bool{}
	}
	n.directory.mu.Unlock()
}

func (n *Node) internalService(name string, source nodeid.ID) (net.Conn, bool) {
	switch name {
	case internalIntroRegister, internalIntroduce, internalRendezvous:
		local, handler := net.Pipe()
		n.rendezvous.internalMu.Lock()
		n.rendezvous.internalConns[handler] = struct{}{}
		n.rendezvous.internalMu.Unlock()
		n.wg.Add(1)
		go func() {
			defer n.wg.Done()
			defer func() {
				n.rendezvous.internalMu.Lock()
				delete(n.rendezvous.internalConns, handler)
				n.rendezvous.internalMu.Unlock()
				_ = handler.Close()
			}()
			switch name {
			case internalIntroRegister:
				n.handleIntroRegister(handler, source)
			case internalIntroduce:
				n.handleIntroduce(handler, source)
			case internalRendezvous:
				n.handleRendezvous(handler, source)
			}
		}()
		return local, true
	default:
		return nil, false
	}
}

func (n *Node) closeInternalServices() {
	n.rendezvous.internalMu.Lock()
	conns := make([]net.Conn, 0, len(n.rendezvous.internalConns))
	for c := range n.rendezvous.internalConns {
		conns = append(conns, c)
	}
	n.rendezvous.internalMu.Unlock()
	for _, c := range conns {
		_ = c.Close()
	}
}

func introRegistrationSignature(serviceID, network, intro string, expires int64) []byte {
	return []byte(fmt.Sprintf("knotroute/intro-registration/v1|%s|%s|%s|%d", network, serviceID, intro, expires))
}
func (n *Node) makeIntroRegistration(s *publishedService, intro nodeid.ID) introRegistrationMessage {
	exp := time.Now().Add(5 * time.Minute).Unix()
	m := introRegistrationMessage{NetworkID: n.network.String(), ServiceID: s.identity.ID.String(), PublicKey: base64.StdEncoding.EncodeToString(s.identity.PublicKey), IntroNode: intro.String(), ExpiresUnix: exp}
	m.Signature = base64.StdEncoding.EncodeToString(s.identity.Sign(introRegistrationSignature(m.ServiceID, m.NetworkID, m.IntroNode, m.ExpiresUnix)))
	return m
}
func (n *Node) verifyIntroRegistration(m introRegistrationMessage) (serviceid.ID, error) {
	if m.NetworkID != n.network.String() || m.IntroNode != n.id.ID.String() {
		return serviceid.ID{}, errors.New("registration target mismatch")
	}
	if m.ExpiresUnix <= time.Now().Unix() || m.ExpiresUnix > time.Now().Add(10*time.Minute).Unix() {
		return serviceid.ID{}, errors.New("invalid registration expiry")
	}
	id, err := serviceid.Parse(m.ServiceID)
	if err != nil {
		return id, err
	}
	pub, err := base64.StdEncoding.DecodeString(m.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize || serviceid.FromPublicKey(pub) != id {
		return id, errors.New("invalid service identity")
	}
	sig, err := base64.StdEncoding.DecodeString(m.Signature)
	if err != nil || !ed25519.Verify(ed25519.PublicKey(pub), introRegistrationSignature(m.ServiceID, m.NetworkID, m.IntroNode, m.ExpiresUnix), sig) {
		return id, errors.New("invalid registration signature")
	}
	return id, nil
}
func (n *Node) handleIntroRegister(conn net.Conn, source nodeid.ID) {
	var m introRegistrationMessage
	if err := readControl(conn, &m); err != nil {
		return
	}
	id, err := n.verifyIntroRegistration(m)
	if err != nil {
		_ = writeControl(conn, simpleAck{Error: err.Error()})
		return
	}
	reg := &introRegistration{conn: conn, expires: time.Unix(m.ExpiresUnix, 0), source: source}
	n.rendezvous.introMu.Lock()
	old := n.rendezvous.registrations[id]
	n.rendezvous.registrations[id] = reg
	n.rendezvous.introMu.Unlock()
	if old != nil {
		_ = old.conn.Close()
	}
	if writeControl(conn, simpleAck{OK: true}) != nil {
		return
	}
	n.addEvent("info", "introduction registration for "+id.Short()+" via "+source.Short())
	buf := make([]byte, 1)
	_, _ = conn.Read(buf)
	n.rendezvous.introMu.Lock()
	if n.rendezvous.registrations[id] == reg {
		delete(n.rendezvous.registrations, id)
	}
	n.rendezvous.introMu.Unlock()
}
func (n *Node) handleIntroduce(conn net.Conn, source nodeid.ID) {
	var req introduceRequest
	if err := readControl(conn, &req); err != nil {
		n.addEvent("warn", "introduce read: "+err.Error())
		return
	}
	id, err := serviceid.Parse(req.ServiceID)
	if err != nil {
		_ = writeControl(conn, simpleAck{Error: "invalid service id"})
		return
	}
	if delta := time.Now().Unix() - req.TimeUnix; delta > 180 || delta < -180 {
		_ = writeControl(conn, simpleAck{Error: "stale introduction"})
		return
	}
	if _, err := nodeid.Parse(req.Rendezvous); err != nil {
		_ = writeControl(conn, simpleAck{Error: "invalid rendezvous"})
		return
	}
	if _, err := base64.StdEncoding.DecodeString(req.Cookie); err != nil {
		_ = writeControl(conn, simpleAck{Error: "invalid cookie"})
		return
	}
	n.rendezvous.introMu.Lock()
	reg := n.rendezvous.registrations[id]
	if reg != nil && time.Now().After(reg.expires) {
		delete(n.rendezvous.registrations, id)
		reg = nil
	}
	n.rendezvous.introMu.Unlock()
	if reg == nil {
		n.addEvent("warn", "introduce: no registration for "+id.Short())
		_ = writeControl(conn, simpleAck{Error: "service is not registered at this introduction point"})
		return
	}
	reg.mu.Lock()
	err = writeControl(reg.conn, req)
	reg.mu.Unlock()
	if err != nil {
		n.addEvent("warn", "introduce forward: "+err.Error())
		_ = writeControl(conn, simpleAck{Error: "introduction channel unavailable"})
		return
	}
	if err := writeControl(conn, simpleAck{OK: true}); err != nil {
		n.addEvent("warn", "introduce ack: "+err.Error())
		return
	}
	n.addEvent("info", "forwarded introduction for "+id.Short()+" from "+source.Short())
}

func (n *Node) handleRendezvous(conn net.Conn, source nodeid.ID) {
	var h rendezvousHello
	if err := readControl(conn, &h); err != nil {
		return
	}
	if h.Role != "client" && h.Role != "service" {
		return
	}
	cookie, err := base64.StdEncoding.DecodeString(h.Cookie)
	if err != nil || len(cookie) != 32 {
		return
	}
	key := h.Cookie
	n.rendezvous.rvMu.Lock()
	for k, w := range n.rendezvous.waiting {
		if time.Now().After(w.expires) {
			delete(n.rendezvous.waiting, k)
			close(w.done)
			_ = w.conn.Close()
		}
	}
	first := n.rendezvous.waiting[key]
	if first == nil {
		w := &rendezvousWait{role: h.Role, conn: conn, done: make(chan struct{}), expires: time.Now().Add(90 * time.Second), source: source}
		n.rendezvous.waiting[key] = w
		n.rendezvous.rvMu.Unlock()
		select {
		case <-w.done:
		case <-n.ctx.Done():
		}
		return
	}
	if first.role == h.Role {
		n.rendezvous.rvMu.Unlock()
		return
	}
	delete(n.rendezvous.waiting, key)
	n.rendezvous.rvMu.Unlock()
	n.addEvent("info", "rendezvous paired "+first.source.Short()+" and "+source.Short())
	proxyBoth(first.conn, conn)
	close(first.done)
}

func (n *Node) ensureIntroductionPoints(s *publishedService) {
	desired := s.cfg.IntroCount
	if desired <= 0 {
		desired = 3
	}
	n.topologyMu.RLock()
	candidates := make([]nodeid.ID, 0, len(n.routes))
	for id := range n.routes {
		candidates = append(candidates, id)
	}
	n.topologyMu.RUnlock()
	sort.Slice(candidates, func(i, j int) bool { return introScoreLess(s.identity.ID, candidates[i], candidates[j]) })
	if desired > len(candidates) {
		desired = len(candidates)
	}
	s.introMu.Lock()
	for _, dst := range candidates[:desired] {
		if s.introSessions[dst] != nil || s.introDialing[dst] {
			continue
		}
		s.introDialing[dst] = true
		n.wg.Add(1)
		go n.maintainIntro(s, dst)
	}
	s.introMu.Unlock()
}
func introScoreLess(id serviceid.ID, a, b nodeid.ID) bool {
	for i := 0; i < 32; i++ {
		x := id[i] ^ a[i]
		y := id[i] ^ b[i]
		if x < y {
			return true
		}
		if x > y {
			return false
		}
	}
	return nodeid.Compare(a, b) < 0
}
func (n *Node) maintainIntro(s *publishedService, dst nodeid.ID) {
	defer n.wg.Done()
	defer func() { s.introMu.Lock(); delete(s.introDialing, dst); s.introMu.Unlock() }()
	backoff := time.Second
	for n.ctx.Err() == nil {
		ctx, cancel := context.WithTimeout(n.ctx, 20*time.Second)
		conn, err := n.OpenCircuitStream(ctx, dst, internalIntroRegister)
		cancel()
		if err == nil {
			if err = writeControl(conn, n.makeIntroRegistration(s, dst)); err == nil {
				var ack simpleAck
				if err = readControl(conn, &ack); err == nil && ack.OK {
					sess := &introSession{node: dst, conn: conn, done: make(chan struct{})}
					s.introMu.Lock()
					s.introSessions[dst] = sess
					s.introMu.Unlock()
					n.addEvent("info", "service "+s.cfg.Name+" introduced at "+dst.Short())
					n.readIntroductions(s, conn)
					s.introMu.Lock()
					if s.introSessions[dst] == sess {
						delete(s.introSessions, dst)
					}
					s.introMu.Unlock()
				}
			}
			_ = conn.Close()
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
		}
	}
}
func (n *Node) readIntroductions(s *publishedService, conn net.Conn) {
	for {
		var req introduceRequest
		if err := readControl(conn, &req); err != nil {
			return
		}
		n.wg.Add(1)
		go func(r introduceRequest) { defer n.wg.Done(); n.acceptIntroduction(s, r) }(req)
	}
}
func (n *Node) activeIntros(s *publishedService) []nodeid.ID {
	s.introMu.Lock()
	defer s.introMu.Unlock()
	out := make([]nodeid.ID, 0, len(s.introSessions))
	for id := range s.introSessions {
		out = append(out, id)
	}
	sort.Slice(out, func(i, j int) bool { return nodeid.Compare(out[i], out[j]) < 0 })
	return out
}

func (n *Node) acceptIntroduction(s *publishedService, req introduceRequest) {
	if req.ServiceID != s.identity.ID.String() || time.Since(time.Unix(req.TimeUnix, 0)) > 3*time.Minute {
		return
	}
	rv, err := nodeid.Parse(req.Rendezvous)
	if err != nil {
		return
	}
	cookie, err := base64.StdEncoding.DecodeString(req.Cookie)
	if err != nil || len(cookie) != 32 {
		return
	}
	clientEph, err := base64.StdEncoding.DecodeString(req.ClientEphemeral)
	if err != nil || len(clientEph) != 32 {
		return
	}
	clientNonce, err := base64.StdEncoding.DecodeString(req.ClientNonce)
	if err != nil || len(clientNonce) != 32 {
		return
	}
	ctx, cancel := context.WithTimeout(n.ctx, 20*time.Second)
	conn, err := n.openInternalEndpoint(ctx, rv, internalRendezvous)
	cancel()
	if err != nil {
		return
	}
	if err := writeControl(conn, rendezvousHello{Role: "service", Cookie: req.Cookie}); err != nil {
		_ = conn.Close()
		return
	}
	priv, err := ecdh.X25519().GenerateKey(randReader{})
	if err != nil {
		_ = conn.Close()
		return
	}
	serviceNonce, err := random32()
	if err != nil {
		_ = conn.Close()
		return
	}
	ack := signServiceAck(s, req.Cookie, clientEph, clientNonce, priv.PublicKey().Bytes(), serviceNonce)
	if err := writeControl(conn, ack); err != nil {
		_ = conn.Close()
		return
	}
	c2s, s2c, err := deriveRendezvousKeys(priv, clientEph, clientNonce, serviceNonce, s.identity.ID, cookie)
	if err != nil {
		_ = conn.Close()
		return
	}
	secure := newRendezvousConn(conn, s.identity.ID, cookie, s2c, c2s)
	dialer := net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	target, err := dialer.DialContext(n.ctx, "tcp", s.cfg.Target)
	if err != nil {
		_ = secure.Close()
		return
	}
	defer target.Close()
	defer secure.Close()
	proxyBoth(secure, target)
}

// randReader keeps the X25519 call testable while still using crypto/rand.
type randReader struct{}

func (randReader) Read(p []byte) (int, error) { return rand.Read(p) }

func (n *Node) openInternalEndpoint(ctx context.Context, dst nodeid.ID, name string) (net.Conn, error) {
	if dst == n.id.ID {
		if conn, ok := n.internalService(name, n.id.ID); ok {
			return conn, nil
		}
		return nil, errors.New("unknown internal service")
	}
	return n.OpenCircuitStream(ctx, dst, name)
}

func (n *Node) OpenService(ctx context.Context, id serviceid.ID) (net.Conn, error) {
	d, err := n.LookupService(ctx, id)
	if err != nil {
		return nil, err
	}
	intros := make([]nodeid.ID, 0, len(d.IntroductionPoints))
	for _, raw := range d.IntroductionPoints {
		nid, e := nodeid.Parse(raw)
		if e == nil {
			if _, ok := n.RouteTo(nid); ok {
				intros = append(intros, nid)
			}
		}
	}
	if len(intros) == 0 {
		return nil, errors.New("no reachable introduction point")
	}
	rv, err := n.chooseRendezvous(intros)
	if err != nil {
		return nil, err
	}
	cookie, err := random32()
	if err != nil {
		return nil, err
	}
	cookieText := base64.StdEncoding.EncodeToString(cookie)
	priv, err := ecdh.X25519().GenerateKey(randReader{})
	if err != nil {
		return nil, err
	}
	clientNonce, err := random32()
	if err != nil {
		return nil, err
	}
	rvConn, err := n.openInternalEndpoint(ctx, rv, internalRendezvous)
	if err != nil {
		return nil, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = rvConn.Close()
		}
	}()
	if err := writeControl(rvConn, rendezvousHello{Role: "client", Cookie: cookieText}); err != nil {
		return nil, fmt.Errorf("rendezvous establish: %w", err)
	}
	var introErr error
	introduced := false
	for _, intro := range intros {
		introConn, err := n.OpenCircuitStream(ctx, intro, internalIntroduce)
		if err != nil {
			introErr = fmt.Errorf("introduction connect via %s: %w", intro.Short(), err)
			continue
		}
		req := introduceRequest{ServiceID: id.String(), Rendezvous: rv.String(), Cookie: cookieText, ClientEphemeral: base64.StdEncoding.EncodeToString(priv.PublicKey().Bytes()), ClientNonce: base64.StdEncoding.EncodeToString(clientNonce), TimeUnix: time.Now().Unix()}
		err = writeControl(introConn, req)
		if err == nil {
			var introAck simpleAck
			err = readControl(introConn, &introAck)
			if err == nil && !introAck.OK {
				err = errors.New(introAck.Error)
			}
		}
		_ = introConn.Close()
		if err == nil {
			introduced = true
			break
		}
		introErr = fmt.Errorf("introduction via %s: %w", intro.Short(), err)
	}
	if !introduced {
		if introErr == nil {
			introErr = errors.New("no introduction point available")
		}
		return nil, introErr
	}
	var ack serviceAck
	if err := readControl(rvConn, &ack); err != nil {
		return nil, fmt.Errorf("rendezvous service acknowledgement: %w", err)
	}
	serviceEph, serviceNonce, err := verifyServiceAck(ack, id, priv.PublicKey().Bytes(), clientNonce)
	if err != nil {
		return nil, err
	}
	c2s, s2c, err := deriveRendezvousKeys(priv, serviceEph, clientNonce, serviceNonce, id, cookie)
	if err != nil {
		return nil, err
	}
	cleanup = false
	n.addEvent("info", "opened rendezvous service "+id.Short()+" via "+rv.Short())
	return newRendezvousConn(rvConn, id, cookie, c2s, s2c), nil
}
func (n *Node) chooseRendezvous(exclude []nodeid.ID) (nodeid.ID, error) {
	skip := map[nodeid.ID]bool{}
	for _, x := range exclude {
		skip[x] = true
	}
	n.topologyMu.RLock()
	ids := make([]nodeid.ID, 0, len(n.routes))
	for id := range n.routes {
		if !skip[id] {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		for id := range n.routes {
			ids = append(ids, id)
		}
	}
	n.topologyMu.RUnlock()
	if len(ids) == 0 {
		return nodeid.ID{}, errors.New("no rendezvous node available")
	}
	sort.Slice(ids, func(i, j int) bool { return nodeid.Compare(ids[i], ids[j]) < 0 })
	seed, _ := random32()
	idx := int(seed[0]) % len(ids)
	return ids[idx], nil
}

func proxyBoth(a, b net.Conn) {
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(a, b)
		if c, ok := a.(interface{ CloseWrite() error }); ok {
			_ = c.CloseWrite()
		}
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(b, a)
		if c, ok := b.(interface{ CloseWrite() error }); ok {
			_ = c.CloseWrite()
		}
		done <- struct{}{}
	}()
	<-done
	<-done
}
