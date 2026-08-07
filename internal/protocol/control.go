package protocol

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/localzet/knotroute/internal/identity"
	"github.com/localzet/knotroute/internal/nodeid"
)

type Hello struct {
	Version   int      `json:"version"`
	NodeID    string   `json:"node_id"`
	PublicKey string   `json:"public_key"`
	Advertise []string `json:"advertise,omitempty"`
	TimeUnix  int64    `json:"time_unix"`
}

type ServiceInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

type LSABody struct {
	Version     int           `json:"version"`
	Sequence    uint64        `json:"sequence"`
	TimeUnix    int64         `json:"time_unix"`
	ExpiresUnix int64         `json:"expires_unix"`
	NodeID      string        `json:"node_id"`
	PublicKey   string        `json:"public_key"`
	Advertise   []string      `json:"advertise,omitempty"`
	Neighbors   []string      `json:"neighbors"`
	Services    []ServiceInfo `json:"services,omitempty"`
}

type LSA struct {
	LSABody
	Signature string `json:"signature"`
}

func NewLSA(id *identity.Identity, sequence uint64, ttl time.Duration, advertise []string, neighbors []nodeid.ID, services []ServiceInfo) (LSA, error) {
	neighborStrings := make([]string, 0, len(neighbors))
	for _, n := range neighbors {
		neighborStrings = append(neighborStrings, n.String())
	}
	sort.Strings(neighborStrings)
	advertise = append([]string(nil), advertise...)
	sort.Strings(advertise)
	services = append([]ServiceInfo(nil), services...)
	sort.Slice(services, func(i, j int) bool { return services[i].Name < services[j].Name })
	now := time.Now().UTC()
	body := LSABody{
		Version: ProtocolVersion, Sequence: sequence, TimeUnix: now.Unix(), ExpiresUnix: now.Add(ttl).Unix(),
		NodeID: id.ID.String(), PublicKey: base64.StdEncoding.EncodeToString(id.PublicKey),
		Advertise: advertise, Neighbors: neighborStrings, Services: services,
	}
	canonical, err := json.Marshal(body)
	if err != nil {
		return LSA{}, err
	}
	return LSA{LSABody: body, Signature: base64.StdEncoding.EncodeToString(id.Sign(canonical))}, nil
}

func (l LSA) Verify(now time.Time) (nodeid.ID, error) {
	if l.Version != ProtocolVersion {
		return nodeid.ID{}, fmt.Errorf("unsupported LSA version %d", l.Version)
	}
	id, err := nodeid.Parse(l.NodeID)
	if err != nil {
		return nodeid.ID{}, err
	}
	pub, err := base64.StdEncoding.DecodeString(l.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return nodeid.ID{}, errors.New("invalid LSA public key")
	}
	if nodeid.FromPublicKey(pub) != id {
		return nodeid.ID{}, errors.New("LSA node id does not match public key")
	}
	sig, err := base64.StdEncoding.DecodeString(l.Signature)
	if err != nil {
		return nodeid.ID{}, errors.New("invalid LSA signature encoding")
	}
	canonical, err := json.Marshal(l.LSABody)
	if err != nil {
		return nodeid.ID{}, err
	}
	if !identity.Verify(pub, canonical, sig) {
		return nodeid.ID{}, errors.New("invalid LSA signature")
	}
	if l.ExpiresUnix <= now.Unix() {
		return nodeid.ID{}, errors.New("LSA expired")
	}
	if l.TimeUnix > now.Add(5*time.Minute).Unix() {
		return nodeid.ID{}, errors.New("LSA timestamp is too far in the future")
	}
	return id, nil
}

type OpenRequest struct {
	Service      string `json:"service"`
	EphemeralKey string `json:"ephemeral_key"`
	Nonce        string `json:"nonce"`
	PublicKey    string `json:"public_key"`
	TimeUnix     int64  `json:"time_unix"`
	Signature    string `json:"signature"`
}

type OpenAck struct {
	EphemeralKey string `json:"ephemeral_key"`
	Nonce        string `json:"nonce"`
	PublicKey    string `json:"public_key"`
	TimeUnix     int64  `json:"time_unix"`
	Signature    string `json:"signature"`
}

type ErrorMessage struct {
	Message string `json:"message"`
}
type CloseMessage struct {
	Reason string `json:"reason,omitempty"`
}

func SignOpenRequest(id *identity.Identity, streamID [16]byte, dst nodeid.ID, service string, ephemeral, nonce []byte, now time.Time) OpenRequest {
	req := OpenRequest{
		Service: service, EphemeralKey: base64.StdEncoding.EncodeToString(ephemeral), Nonce: base64.StdEncoding.EncodeToString(nonce),
		PublicKey: base64.StdEncoding.EncodeToString(id.PublicKey), TimeUnix: now.Unix(),
	}
	req.Signature = base64.StdEncoding.EncodeToString(id.Sign(openRequestMessage(streamID, id.ID, dst, req)))
	return req
}

func VerifyOpenRequest(req OpenRequest, streamID [16]byte, src, dst nodeid.ID, now time.Time) ([]byte, []byte, error) {
	if delta := now.Unix() - req.TimeUnix; delta > 180 || delta < -180 {
		return nil, nil, errors.New("open request timestamp outside allowed window")
	}
	pub, err := base64.StdEncoding.DecodeString(req.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return nil, nil, errors.New("invalid source public key")
	}
	if nodeid.FromPublicKey(pub) != src {
		return nil, nil, errors.New("source node id does not match public key")
	}
	eph, err := base64.StdEncoding.DecodeString(req.EphemeralKey)
	if err != nil || len(eph) != 32 {
		return nil, nil, errors.New("invalid ephemeral key")
	}
	nonce, err := base64.StdEncoding.DecodeString(req.Nonce)
	if err != nil || len(nonce) != 32 {
		return nil, nil, errors.New("invalid nonce")
	}
	sig, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil || !identity.Verify(pub, openRequestMessage(streamID, src, dst, req), sig) {
		return nil, nil, errors.New("invalid open request signature")
	}
	return eph, nonce, nil
}

func SignOpenAck(id *identity.Identity, streamID [16]byte, src nodeid.ID, ephemeral, nonce []byte, now time.Time) OpenAck {
	ack := OpenAck{EphemeralKey: base64.StdEncoding.EncodeToString(ephemeral), Nonce: base64.StdEncoding.EncodeToString(nonce), PublicKey: base64.StdEncoding.EncodeToString(id.PublicKey), TimeUnix: now.Unix()}
	ack.Signature = base64.StdEncoding.EncodeToString(id.Sign(openAckMessage(streamID, src, id.ID, ack)))
	return ack
}

func VerifyOpenAck(ack OpenAck, streamID [16]byte, src, dst nodeid.ID, now time.Time) ([]byte, []byte, error) {
	if delta := now.Unix() - ack.TimeUnix; delta > 180 || delta < -180 {
		return nil, nil, errors.New("open ack timestamp outside allowed window")
	}
	pub, err := base64.StdEncoding.DecodeString(ack.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return nil, nil, errors.New("invalid destination public key")
	}
	if nodeid.FromPublicKey(pub) != dst {
		return nil, nil, errors.New("destination node id does not match public key")
	}
	eph, err := base64.StdEncoding.DecodeString(ack.EphemeralKey)
	if err != nil || len(eph) != 32 {
		return nil, nil, errors.New("invalid ephemeral key")
	}
	nonce, err := base64.StdEncoding.DecodeString(ack.Nonce)
	if err != nil || len(nonce) != 32 {
		return nil, nil, errors.New("invalid nonce")
	}
	sig, err := base64.StdEncoding.DecodeString(ack.Signature)
	if err != nil || !identity.Verify(pub, openAckMessage(streamID, src, dst, ack), sig) {
		return nil, nil, errors.New("invalid open ack signature")
	}
	return eph, nonce, nil
}

func openRequestMessage(streamID [16]byte, src, dst nodeid.ID, req OpenRequest) []byte {
	return []byte("knotroute/open/v1|" + hex.EncodeToString(streamID[:]) + "|" + src.String() + "|" + dst.String() + "|" + req.Service + "|" + req.EphemeralKey + "|" + req.Nonce + "|" + fmt.Sprint(req.TimeUnix))
}

func openAckMessage(streamID [16]byte, src, dst nodeid.ID, ack OpenAck) []byte {
	return []byte("knotroute/ack/v1|" + hex.EncodeToString(streamID[:]) + "|" + src.String() + "|" + dst.String() + "|" + ack.EphemeralKey + "|" + ack.Nonce + "|" + fmt.Sprint(ack.TimeUnix))
}
