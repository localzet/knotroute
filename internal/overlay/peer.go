package overlay

import (
	"crypto/ed25519"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/localzet/knotroute/internal/nodeid"
	"github.com/localzet/knotroute/internal/protocol"
)

type peer struct {
	node       *Node
	id         nodeid.ID
	conn       *tls.Conn
	outbound   bool
	remoteAddr string
	advertise  []string
	sendMu     sync.Mutex
	closeOnce  sync.Once
	done       chan struct{}
}

func (n *Node) establishPeer(raw net.Conn, outbound bool, expected *nodeid.ID) (*peer, error) {
	var conn *tls.Conn
	if outbound {
		conn = tls.Client(raw, n.clientTLS)
	} else {
		conn = tls.Server(raw, n.serverTLS)
	}
	_ = conn.SetDeadline(time.Now().Add(12 * time.Second))
	if err := conn.HandshakeContext(n.ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("TLS handshake: %w", err)
	}
	state := conn.ConnectionState()
	if len(state.PeerCertificates) != 1 {
		_ = conn.Close()
		return nil, errors.New("peer did not provide exactly one certificate")
	}
	pub, ok := state.PeerCertificates[0].PublicKey.(ed25519.PublicKey)
	if !ok {
		_ = conn.Close()
		return nil, errors.New("peer certificate does not use Ed25519")
	}
	peerID := nodeid.FromPublicKey(pub)
	if expected != nil && peerID != *expected {
		_ = conn.Close()
		return nil, fmt.Errorf("peer identity mismatch: got %s, expected %s", peerID, *expected)
	}

	hello := protocol.Hello{
		Version:   protocol.ProtocolVersion,
		NodeID:    n.id.ID.String(),
		PublicKey: base64.StdEncoding.EncodeToString(n.id.PublicKey),
		Advertise: n.advertisedAddresses(),
		TimeUnix:  time.Now().Unix(),
	}
	helloRaw, _ := json.Marshal(hello)
	if err := protocol.WriteFrame(conn, protocol.FrameHello, helloRaw); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("send hello: %w", err)
	}
	typ, payload, err := protocol.ReadFrame(conn)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("read hello: %w", err)
	}
	if typ != protocol.FrameHello {
		_ = conn.Close()
		return nil, errors.New("first peer frame is not hello")
	}
	var remote protocol.Hello
	if err := json.Unmarshal(payload, &remote); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("decode hello: %w", err)
	}
	if remote.Version != protocol.ProtocolVersion {
		_ = conn.Close()
		return nil, fmt.Errorf("unsupported peer protocol version %d", remote.Version)
	}
	claimedID, err := nodeid.Parse(remote.NodeID)
	if err != nil || claimedID != peerID {
		_ = conn.Close()
		return nil, errors.New("peer hello identity does not match TLS certificate")
	}
	claimedPub, err := base64.StdEncoding.DecodeString(remote.PublicKey)
	if err != nil || !ed25519.PublicKey(claimedPub).Equal(pub) {
		_ = conn.Close()
		return nil, errors.New("peer hello public key does not match TLS certificate")
	}
	if delta := time.Now().Unix() - remote.TimeUnix; delta > 300 || delta < -300 {
		_ = conn.Close()
		return nil, errors.New("peer clock differs by more than five minutes")
	}
	if peerID == n.id.ID {
		_ = conn.Close()
		return nil, errors.New("refusing self-connection")
	}
	_ = conn.SetDeadline(time.Time{})

	p := &peer{node: n, id: peerID, conn: conn, outbound: outbound, remoteAddr: raw.RemoteAddr().String(), advertise: append([]string(nil), remote.Advertise...), done: make(chan struct{})}
	if !n.registerPeer(p) {
		_ = conn.Close()
		return nil, errors.New("duplicate peer connection rejected")
	}
	return p, nil
}

func (p *peer) run() {
	defer p.close("connection closed")
	for {
		typ, payload, err := protocol.ReadFrame(p.conn)
		if err != nil {
			return
		}
		p.node.stats.framesReceived.Add(1)
		p.node.stats.bytesReceived.Add(uint64(len(payload) + 5))
		switch typ {
		case protocol.FrameLSA:
			p.node.handleLSA(payload, p)
		case protocol.FramePacket:
			packet, err := protocol.ParsePacket(payload)
			if err != nil {
				p.node.addEvent("warn", "discarded malformed packet from "+p.id.Short()+": "+err.Error())
				continue
			}
			p.node.handlePacket(packet)
		case protocol.FramePing:
			_ = p.send(protocol.FramePong, payload)
		case protocol.FramePong:
		default:
			p.node.addEvent("warn", fmt.Sprintf("peer %s sent unknown frame type %d", p.id.Short(), typ))
		}
	}
}

func (p *peer) send(typ byte, payload []byte) error {
	p.sendMu.Lock()
	defer p.sendMu.Unlock()
	if err := protocol.WriteFrame(p.conn, typ, payload); err != nil {
		return err
	}
	p.node.stats.framesSent.Add(1)
	p.node.stats.bytesSent.Add(uint64(len(payload) + 5))
	return nil
}

func (p *peer) sendPacket(packet protocol.Packet) error {
	raw, err := packet.MarshalBinary()
	if err != nil {
		return err
	}
	return p.send(protocol.FramePacket, raw)
}

func (p *peer) close(reason string) {
	p.closeOnce.Do(func() {
		_ = p.conn.Close()
		p.node.unregisterPeer(p)
		close(p.done)
		p.node.addEvent("info", fmt.Sprintf("peer %s disconnected: %s", p.id.Short(), reason))
	})
}
