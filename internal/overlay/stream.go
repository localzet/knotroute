package overlay

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/nodeid"
	"github.com/localzet/knotroute/internal/protocol"
)

type openResult struct {
	ack protocol.OpenAck
	src nodeid.ID
	err error
}

type stream struct {
	node      *Node
	id        [16]byte
	peer      nodeid.ID
	initiator bool
	conn      net.Conn
	sendKey   []byte
	recvKey   []byte
	sendSeq   atomic.Uint64
	recvSeq   uint64
	recvMu    sync.Mutex
	incoming  chan protocol.Packet
	done      chan struct{}
	closeOnce sync.Once
	pumpOnce  sync.Once
}

func newStream(n *Node, id [16]byte, peerID nodeid.ID, conn net.Conn, initiator bool, sendKey, recvKey []byte) *stream {
	return &stream{node: n, id: id, peer: peerID, conn: conn, initiator: initiator, sendKey: append([]byte(nil), sendKey...), recvKey: append([]byte(nil), recvKey...), incoming: make(chan protocol.Packet, 128), done: make(chan struct{})}
}

func (s *stream) startReceiver() {
	s.node.wg.Add(1)
	go func() { defer s.node.wg.Done(); s.receiveLoop() }()
}

func (s *stream) startPump() {
	s.pumpOnce.Do(func() {
		s.node.wg.Add(1)
		go func() { defer s.node.wg.Done(); s.pumpLoop() }()
	})
}

func (s *stream) pumpLoop() {
	buf := make([]byte, 32*1024)
	for {
		n, err := s.conn.Read(buf)
		if n > 0 {
			seq := s.sendSeq.Add(1) - 1
			src, dst := s.node.id.ID, s.peer
			ciphertext, sealErr := seal(s.sendKey, s.id, src, dst, seq, buf[:n])
			if sealErr != nil {
				s.closeLocal(sealErr.Error(), true)
				return
			}
			packet := s.node.newPacket(protocol.PacketData, dst, s.id, seq, ciphertext)
			if sendErr := s.node.sendPacket(packet); sendErr != nil {
				s.closeLocal(sendErr.Error(), false)
				return
			}
		}
		if err != nil {
			if !errors.Is(err, io.EOF) && !isClosedConn(err) {
				s.node.addEvent("warn", "stream read failed: "+err.Error())
			}
			s.closeLocal("local endpoint closed", true)
			return
		}
	}
}

func (s *stream) receiveLoop() {
	for {
		select {
		case <-s.done:
			return
		case packet := <-s.incoming:
			if packet.Src != s.peer {
				s.closeLocal("stream source changed", true)
				return
			}
			s.recvMu.Lock()
			if packet.Seq != s.recvSeq {
				s.recvMu.Unlock()
				s.closeLocal(fmt.Sprintf("out-of-order stream packet: got %d, expected %d", packet.Seq, s.recvSeq), true)
				return
			}
			plaintext, err := openCiphertext(s.recvKey, s.id, packet.Src, packet.Dst, packet.Seq, packet.Payload)
			if err == nil {
				s.recvSeq++
			}
			s.recvMu.Unlock()
			if err != nil {
				s.closeLocal(err.Error(), true)
				return
			}
			if err := writeAll(s.conn, plaintext); err != nil {
				s.closeLocal("write local endpoint: "+err.Error(), true)
				return
			}
		}
	}
}

func (s *stream) deliver(packet protocol.Packet) {
	select {
	case s.incoming <- packet:
	case <-s.done:
	case <-s.node.ctx.Done():
	}
}

func (s *stream) closeLocal(reason string, notify bool) {
	s.closeOnce.Do(func() {
		if notify && s.node.ctx.Err() == nil {
			payload, _ := json.Marshal(protocol.CloseMessage{Reason: reason})
			_ = s.node.sendPacket(s.node.newPacket(protocol.PacketClose, s.peer, s.id, 0, payload))
		}
		_ = s.conn.Close()
		close(s.done)
		s.node.streamsMu.Lock()
		if current := s.node.streams[s.id]; current == s {
			delete(s.node.streams, s.id)
		}
		s.node.streamsMu.Unlock()
	})
}

func (n *Node) OpenStream(ctx context.Context, destination nodeid.ID, service string) (net.Conn, error) {
	local, app := net.Pipe()
	if err := n.openWithConn(ctx, destination, service, local); err != nil {
		_ = local.Close()
		_ = app.Close()
		return nil, err
	}
	return app, nil
}

func (n *Node) openWithConn(ctx context.Context, destination nodeid.ID, serviceName string, conn net.Conn) error {
	if destination == n.id.ID {
		return errors.New("opening a local service through the overlay is not supported; connect to its target directly")
	}
	if _, ok := n.RouteTo(destination); !ok {
		return fmt.Errorf("no route to %s", destination)
	}
	var streamID [16]byte
	if _, err := rand.Read(streamID[:]); err != nil {
		return err
	}
	eph, err := newEphemeralKey()
	if err != nil {
		return err
	}
	openNonce := make([]byte, 32)
	if _, err := rand.Read(openNonce); err != nil {
		return err
	}
	req := protocol.SignOpenRequest(n.id, streamID, destination, serviceName, eph.public, openNonce, time.Now())
	payload, err := json.Marshal(req)
	if err != nil {
		return err
	}
	resultCh := make(chan openResult, 1)
	n.streamsMu.Lock()
	n.pending[streamID] = resultCh
	n.streamsMu.Unlock()
	defer func() { n.streamsMu.Lock(); delete(n.pending, streamID); n.streamsMu.Unlock() }()
	if err := n.sendPacket(n.newPacket(protocol.PacketOpen, destination, streamID, 0, payload)); err != nil {
		return err
	}
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	var result openResult
	select {
	case result = <-resultCh:
	case <-ctx.Done():
		return ctx.Err()
	case <-n.ctx.Done():
		return errors.New("node stopped")
	case <-timer.C:
		return errors.New("timed out opening overlay stream")
	}
	if result.err != nil {
		return result.err
	}
	if result.src != destination {
		return errors.New("open acknowledgement came from unexpected node")
	}
	peerEph, ackNonce, err := protocol.VerifyOpenAck(result.ack, streamID, n.id.ID, destination, time.Now())
	if err != nil {
		return err
	}
	shared, err := sharedSecret(eph.private, peerEph)
	if err != nil {
		return err
	}
	c2s, s2c, err := deriveSessionKeys(shared, openNonce, ackNonce, streamID, n.id.ID, destination)
	if err != nil {
		return err
	}
	s := newStream(n, streamID, destination, conn, true, c2s, s2c)
	n.streamsMu.Lock()
	n.streams[streamID] = s
	n.streamsMu.Unlock()
	s.startReceiver()
	if err := n.sendPacket(n.newPacket(protocol.PacketReady, destination, streamID, 0, nil)); err != nil {
		s.closeLocal(err.Error(), false)
		return err
	}
	s.startPump()
	n.addEvent("info", fmt.Sprintf("opened stream %x to %s/%s", streamID[:4], destination.Short(), serviceName))
	return nil
}

func (n *Node) handleLocalPacket(packet protocol.Packet) {
	if n.handleDirectoryPacket(packet) {
		return
	}
	switch packet.Kind {
	case protocol.PacketOpen:
		n.wg.Add(1)
		go func() { defer n.wg.Done(); n.handleOpen(packet) }()
	case protocol.PacketOpenAck:
		var ack protocol.OpenAck
		if err := json.Unmarshal(packet.Payload, &ack); err != nil {
			return
		}
		n.streamsMu.Lock()
		ch := n.pending[packet.StreamID]
		n.streamsMu.Unlock()
		if ch != nil {
			select {
			case ch <- openResult{ack: ack, src: packet.Src}:
			default:
			}
		}
	case protocol.PacketError:
		var message protocol.ErrorMessage
		_ = json.Unmarshal(packet.Payload, &message)
		if message.Message == "" {
			message.Message = "remote node rejected the stream"
		}
		n.streamsMu.Lock()
		ch := n.pending[packet.StreamID]
		n.streamsMu.Unlock()
		if ch != nil {
			select {
			case ch <- openResult{src: packet.Src, err: errors.New(message.Message)}:
			default:
			}
		}
	case protocol.PacketReady:
		n.streamsMu.Lock()
		s := n.streams[packet.StreamID]
		n.streamsMu.Unlock()
		if s != nil && s.peer == packet.Src {
			s.startPump()
		}
	case protocol.PacketData:
		n.streamsMu.Lock()
		s := n.streams[packet.StreamID]
		n.streamsMu.Unlock()
		if s != nil {
			s.deliver(packet)
		}
	case protocol.PacketClose:
		n.streamsMu.Lock()
		s := n.streams[packet.StreamID]
		n.streamsMu.Unlock()
		if s != nil {
			var closeMsg protocol.CloseMessage
			_ = json.Unmarshal(packet.Payload, &closeMsg)
			s.closeLocal(closeMsg.Reason, false)
		}
	}
}

func (n *Node) handleOpen(packet protocol.Packet) {
	var req protocol.OpenRequest
	if err := json.Unmarshal(packet.Payload, &req); err != nil {
		n.sendOpenError(packet, "malformed open request")
		return
	}
	peerEph, openNonce, err := protocol.VerifyOpenRequest(req, packet.StreamID, packet.Src, n.id.ID, time.Now())
	if err != nil {
		n.sendOpenError(packet, err.Error())
		return
	}
	var conn net.Conn
	var service config.Service
	if internal, ok := n.internalService(req.Service, packet.Src); ok {
		conn = internal
	} else {
		var found bool
		service, found = n.service(req.Service)
		if !found {
			n.sendOpenError(packet, "service not found")
			return
		}
		if !n.allowed(service, packet.Src) {
			n.sendOpenError(packet, "source is not allowed to access this service")
			return
		}
		dialer := net.Dialer{Timeout: 10 * time.Second, KeepAlive: 20 * time.Second}
		var err error
		conn, err = dialer.DialContext(n.ctx, "tcp", service.Target)
		if err != nil {
			n.sendOpenError(packet, "service target unavailable: "+err.Error())
			return
		}
	}
	eph, err := newEphemeralKey()
	if err != nil {
		_ = conn.Close()
		n.sendOpenError(packet, err.Error())
		return
	}
	ackNonce := make([]byte, 32)
	if _, err := rand.Read(ackNonce); err != nil {
		_ = conn.Close()
		n.sendOpenError(packet, err.Error())
		return
	}
	shared, err := sharedSecret(eph.private, peerEph)
	if err != nil {
		_ = conn.Close()
		n.sendOpenError(packet, err.Error())
		return
	}
	c2s, s2c, err := deriveSessionKeys(shared, openNonce, ackNonce, packet.StreamID, packet.Src, n.id.ID)
	if err != nil {
		_ = conn.Close()
		n.sendOpenError(packet, err.Error())
		return
	}
	s := newStream(n, packet.StreamID, packet.Src, conn, false, s2c, c2s)
	n.streamsMu.Lock()
	if _, exists := n.streams[packet.StreamID]; exists {
		n.streamsMu.Unlock()
		_ = conn.Close()
		n.sendOpenError(packet, "stream id already exists")
		return
	}
	n.streams[packet.StreamID] = s
	n.streamsMu.Unlock()
	s.startReceiver()
	ack := protocol.SignOpenAck(n.id, packet.StreamID, packet.Src, eph.public, ackNonce, time.Now())
	payload, _ := json.Marshal(ack)
	if err := n.sendPacketWithRetry(n.newPacket(protocol.PacketOpenAck, packet.Src, packet.StreamID, 0, payload), 5*time.Second); err != nil {
		s.closeLocal(err.Error(), false)
		return
	}
	name := service.Name
	if name == "" {
		name = req.Service
	}
	n.addEvent("info", fmt.Sprintf("accepted stream %x from %s to service %s", packet.StreamID[:4], packet.Src.Short(), name))
}

func (n *Node) sendOpenError(open protocol.Packet, message string) {
	payload, _ := json.Marshal(protocol.ErrorMessage{Message: message})
	_ = n.sendPacketWithRetry(n.newPacket(protocol.PacketError, open.Src, open.StreamID, 0, payload), 5*time.Second)
}

// sendPacketWithRetry covers the small convergence window in which a request
// has already reached this node but the reverse link-state route has not yet
// arrived. Data packets remain fail-fast; only stream-control replies use this
// bounded retry so opening a service does not randomly stall during startup.
func (n *Node) sendPacketWithRetry(packet protocol.Packet, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		if err := n.sendPacket(packet); err == nil {
			return nil
		} else {
			lastErr = err
			// A failed write may have reached one relay. A fresh packet ID keeps a
			// later retry from being mistaken for a replay by that relay.
			_, _ = rand.Read(packet.PacketID[:])
		}
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return lastErr
		}
		delay := 25 * time.Millisecond
		if remaining < delay {
			delay = remaining
		}
		timer := time.NewTimer(delay)
		select {
		case <-n.ctx.Done():
			timer.Stop()
			return errors.New("node stopped")
		case <-timer.C:
		}
	}
}

func writeAll(w io.Writer, data []byte) error {
	for len(data) > 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}
func isClosedConn(err error) bool { return errors.Is(err, net.ErrClosed) }
