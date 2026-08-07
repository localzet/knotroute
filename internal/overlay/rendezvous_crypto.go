package overlay

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/localzet/knotroute/internal/serviceid"
)

type serviceAck struct {
	ServiceID    string `json:"service_id"`
	PublicKey    string `json:"public_key"`
	EphemeralKey string `json:"ephemeral_key"`
	Nonce        string `json:"nonce"`
	Cookie       string `json:"cookie"`
	TimeUnix     int64  `json:"time_unix"`
	Signature    string `json:"signature"`
}

func signServiceAck(s *publishedService, cookie string, clientEph, clientNonce, serviceEph, serviceNonce []byte) serviceAck {
	a := serviceAck{ServiceID: s.identity.ID.String(), PublicKey: base64.StdEncoding.EncodeToString(s.identity.PublicKey), EphemeralKey: base64.StdEncoding.EncodeToString(serviceEph), Nonce: base64.StdEncoding.EncodeToString(serviceNonce), Cookie: cookie, TimeUnix: time.Now().Unix()}
	a.Signature = base64.StdEncoding.EncodeToString(s.identity.Sign(serviceAckMessage(a, clientEph, clientNonce)))
	return a
}
func verifyServiceAck(a serviceAck, want serviceid.ID, clientEph, clientNonce []byte) ([]byte, []byte, error) {
	if a.ServiceID != want.String() {
		return nil, nil, errors.New("service acknowledgement identity mismatch")
	}
	if delta := time.Now().Unix() - a.TimeUnix; delta > 180 || delta < -180 {
		return nil, nil, errors.New("service acknowledgement timestamp outside allowed window")
	}
	pub, err := base64.StdEncoding.DecodeString(a.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize || serviceid.FromPublicKey(pub) != want {
		return nil, nil, errors.New("invalid service public key")
	}
	eph, err := base64.StdEncoding.DecodeString(a.EphemeralKey)
	if err != nil || len(eph) != 32 {
		return nil, nil, errors.New("invalid service ephemeral key")
	}
	nonce, err := base64.StdEncoding.DecodeString(a.Nonce)
	if err != nil || len(nonce) != 32 {
		return nil, nil, errors.New("invalid service nonce")
	}
	sig, err := base64.StdEncoding.DecodeString(a.Signature)
	if err != nil || !ed25519.Verify(ed25519.PublicKey(pub), serviceAckMessage(a, clientEph, clientNonce), sig) {
		return nil, nil, errors.New("invalid service acknowledgement signature")
	}
	return eph, nonce, nil
}
func serviceAckMessage(a serviceAck, clientEph, clientNonce []byte) []byte {
	return []byte("knotroute/service-ack/v1|" + a.ServiceID + "|" + a.EphemeralKey + "|" + a.Nonce + "|" + a.Cookie + "|" + base64.StdEncoding.EncodeToString(clientEph) + "|" + base64.StdEncoding.EncodeToString(clientNonce) + "|" + fmt.Sprint(a.TimeUnix))
}
func deriveRendezvousKeys(priv *ecdh.PrivateKey, peer, clientNonce, serviceNonce []byte, id serviceid.ID, cookie []byte) ([]byte, []byte, error) {
	pub, err := ecdh.X25519().NewPublicKey(peer)
	if err != nil {
		return nil, nil, err
	}
	shared, err := priv.ECDH(pub)
	if err != nil {
		return nil, nil, err
	}
	saltInput := append(append([]byte(nil), clientNonce...), serviceNonce...)
	salt := sha256.Sum256(saltInput)
	info := append([]byte("knotroute/rendezvous/v1|"), id[:]...)
	info = append(info, cookie...)
	m := hkdfSHA256(shared, salt[:], info, 64)
	return m[:32], m[32:], nil
}

type rendezvousConn struct {
	net.Conn
	id               serviceid.ID
	cookie           [32]byte
	sendKey, recvKey []byte
	sendMu, recvMu   sync.Mutex
	sendSeq, recvSeq uint64
	readBuf          []byte
}

func newRendezvousConn(c net.Conn, id serviceid.ID, cookie []byte, send, recv []byte) *rendezvousConn {
	r := &rendezvousConn{Conn: c, id: id, sendKey: append([]byte(nil), send...), recvKey: append([]byte(nil), recv...)}
	copy(r.cookie[:], cookie)
	return r
}
func (r *rendezvousConn) Write(p []byte) (int, error) {
	r.sendMu.Lock()
	defer r.sendMu.Unlock()
	total := 0
	for len(p) > 0 {
		n := len(p)
		if n > 32*1024 {
			n = 32 * 1024
		}
		aead, err := newAEAD(r.sendKey)
		if err != nil {
			return total, err
		}
		nonce := makeNonce(r.sendKey, r.sendSeq)
		aad := r.aad(r.sendSeq)
		ct := aead.Seal(nil, nonce, p[:n], aad)
		frameLen := 8 + len(ct)
		if frameLen > 1<<20 {
			return total, errors.New("rendezvous frame too large")
		}
		var h [12]byte
		binary.BigEndian.PutUint32(h[:4], uint32(frameLen))
		binary.BigEndian.PutUint64(h[4:], r.sendSeq)
		if err := writeAll(r.Conn, h[:]); err != nil {
			return total, err
		}
		if err := writeAll(r.Conn, ct); err != nil {
			return total, err
		}
		r.sendSeq++
		total += n
		p = p[n:]
	}
	return total, nil
}
func (r *rendezvousConn) Read(p []byte) (int, error) {
	r.recvMu.Lock()
	defer r.recvMu.Unlock()
	if len(r.readBuf) > 0 {
		n := copy(p, r.readBuf)
		r.readBuf = r.readBuf[n:]
		return n, nil
	}
	var h [12]byte
	if _, err := io.ReadFull(r.Conn, h[:]); err != nil {
		return 0, err
	}
	ln := int(binary.BigEndian.Uint32(h[:4]))
	seq := binary.BigEndian.Uint64(h[4:])
	if seq != r.recvSeq {
		return 0, fmt.Errorf("rendezvous sequence mismatch: got %d want %d", seq, r.recvSeq)
	}
	if ln < 8+16 || ln > 1<<20 {
		return 0, errors.New("invalid rendezvous frame size")
	}
	ct := make([]byte, ln-8)
	if _, err := io.ReadFull(r.Conn, ct); err != nil {
		return 0, err
	}
	aead, err := newAEAD(r.recvKey)
	if err != nil {
		return 0, err
	}
	pt, err := aead.Open(nil, makeNonce(r.recvKey, seq), ct, r.aad(seq))
	if err != nil {
		return 0, errors.New("rendezvous authentication failed")
	}
	r.recvSeq++
	n := copy(p, pt)
	if n < len(pt) {
		r.readBuf = append(r.readBuf[:0], pt[n:]...)
	}
	return n, nil
}
func (r *rendezvousConn) aad(seq uint64) []byte {
	out := make([]byte, 0, 32+32+8)
	out = append(out, r.id[:]...)
	out = append(out, r.cookie[:]...)
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], seq)
	return append(out, b[:]...)
}

func writeControl(w io.Writer, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	if len(raw) > 64<<10 {
		return errors.New("control message too large")
	}
	var h [4]byte
	binary.BigEndian.PutUint32(h[:], uint32(len(raw)))
	if err := writeAll(w, h[:]); err != nil {
		return err
	}
	return writeAll(w, raw)
}
func readControl(r io.Reader, v any) error {
	var h [4]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return err
	}
	n := int(binary.BigEndian.Uint32(h[:]))
	if n <= 0 || n > 64<<10 {
		return errors.New("invalid control message size")
	}
	raw := make([]byte, n)
	if _, err := io.ReadFull(r, raw); err != nil {
		return err
	}
	return json.Unmarshal(raw, v)
}
func random32() ([]byte, error) { b := make([]byte, 32); _, err := rand.Read(b); return b, err }
