package knotmailbox

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

const maxFrame = 4 << 20

type Dialer interface {
	Dial(context.Context, string) (net.Conn, error)
}

type Store interface {
	Put(Envelope) error
	List(ID, int) ([]Envelope, error)
	Delete(ID, []string) error
}

type request struct {
	Version   int             `json:"version"`
	Op        string          `json:"op"`
	Envelope  *Envelope       `json:"envelope,omitempty"`
	Identity  *PublicIdentity `json:"identity,omitempty"`
	TimeUnix  int64           `json:"time_unix,omitempty"`
	Signature string          `json:"signature,omitempty"`
	Limit     int             `json:"limit,omitempty"`
	IDs       []string        `json:"ids,omitempty"`
}
type response struct {
	OK        bool       `json:"ok"`
	Error     string     `json:"error,omitempty"`
	Envelopes []Envelope `json:"envelopes,omitempty"`
}

func Server(store Store, maxEnvelopeBytes int) func(net.Conn) {
	if maxEnvelopeBytes <= 0 {
		maxEnvelopeBytes = 1 << 20
	}
	return func(conn net.Conn) {
		if store == nil {
			return
		}
		var req request
		if err := readFrame(conn, &req); err != nil || req.Version != 1 {
			return
		}
		switch req.Op {
		case "put":
			if req.Envelope == nil {
				_ = writeFrame(conn, response{Error: "missing envelope"})
				return
			}
			raw, _ := json.Marshal(req.Envelope)
			if len(raw) > maxEnvelopeBytes {
				_ = writeFrame(conn, response{Error: "envelope too large"})
				return
			}
			if _, err := ParseID(req.Envelope.RecipientID); err != nil {
				_ = writeFrame(conn, response{Error: "invalid recipient"})
				return
			}
			if err := store.Put(*req.Envelope); err != nil {
				_ = writeFrame(conn, response{Error: err.Error()})
				return
			}
			_ = writeFrame(conn, response{OK: true})
		case "fetch":
			id, err := verifyAuthorization(req, "fetch")
			if err != nil {
				_ = writeFrame(conn, response{Error: err.Error()})
				return
			}
			limit := req.Limit
			if limit <= 0 || limit > 256 {
				limit = 256
			}
			envelopes, err := store.List(id, limit)
			if err != nil {
				_ = writeFrame(conn, response{Error: err.Error()})
				return
			}
			_ = writeFrame(conn, response{OK: true, Envelopes: envelopes})
		case "ack":
			id, err := verifyAuthorization(req, "ack")
			if err != nil {
				_ = writeFrame(conn, response{Error: err.Error()})
				return
			}
			if len(req.IDs) > 256 {
				_ = writeFrame(conn, response{Error: "too many ids"})
				return
			}
			if err := store.Delete(id, req.IDs); err != nil {
				_ = writeFrame(conn, response{Error: err.Error()})
				return
			}
			_ = writeFrame(conn, response{OK: true})
		}
	}
}

func verifyAuthorization(req request, op string) (ID, error) {
	if req.Identity == nil {
		return ID{}, errors.New("missing mailbox identity")
	}
	id, signing, _, err := req.Identity.Verify()
	if err != nil {
		return ID{}, err
	}
	if delta := time.Now().Unix() - req.TimeUnix; delta > 180 || delta < -180 {
		return ID{}, errors.New("mailbox authorization timestamp expired")
	}
	sig, err := base64.RawStdEncoding.DecodeString(req.Signature)
	if err != nil {
		return ID{}, errors.New("invalid mailbox authorization signature")
	}
	message := authorizationMessage(op, id.String(), req.TimeUnix, req.IDs)
	if !ed25519.Verify(signing, message, sig) {
		return ID{}, errors.New("invalid mailbox authorization")
	}
	return id, nil
}

func authorizationMessage(op, id string, at int64, ids []string) []byte {
	raw, _ := json.Marshal(ids)
	return []byte(fmt.Sprintf("knotroute/mailbox/auth/v1|%s|%s|%d|%s", op, id, at, raw))
}

type Client struct{ Dialer Dialer }

func (c Client) Send(ctx context.Context, address string, sender *Identity, recipient PublicIdentity, payload []byte) (string, error) {
	if c.Dialer == nil {
		return "", errors.New("mailbox dialer is nil")
	}
	envelope, err := Seal(sender, recipient, payload, time.Now())
	if err != nil {
		return "", err
	}
	conn, err := c.Dialer.Dial(ctx, address)
	if err != nil {
		return "", err
	}
	defer conn.Close()
	if err := writeFrame(conn, request{Version: 1, Op: "put", Envelope: &envelope}); err != nil {
		return "", err
	}
	var resp response
	if err := readFrame(conn, &resp); err != nil {
		return "", err
	}
	if !resp.OK {
		return "", errors.New(resp.Error)
	}
	return envelope.MessageID, nil
}

func (c Client) Fetch(ctx context.Context, address string, recipient *Identity, limit int) ([]Message, error) {
	if c.Dialer == nil {
		return nil, errors.New("mailbox dialer is nil")
	}
	req := signedRequest("fetch", recipient, limit, nil)
	conn, err := c.Dialer.Dial(ctx, address)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	if err := writeFrame(conn, req); err != nil {
		return nil, err
	}
	var resp response
	if err := readFrame(conn, &resp); err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, errors.New(resp.Error)
	}
	out := make([]Message, 0, len(resp.Envelopes))
	for _, envelope := range resp.Envelopes {
		message, err := Open(recipient, envelope)
		if err != nil {
			continue
		}
		out = append(out, message)
	}
	return out, nil
}

func (c Client) Ack(ctx context.Context, address string, recipient *Identity, ids []string) error {
	if c.Dialer == nil {
		return errors.New("mailbox dialer is nil")
	}
	if len(ids) == 0 {
		return nil
	}
	req := signedRequest("ack", recipient, 0, ids)
	conn, err := c.Dialer.Dial(ctx, address)
	if err != nil {
		return err
	}
	defer conn.Close()
	if err := writeFrame(conn, req); err != nil {
		return err
	}
	var resp response
	if err := readFrame(conn, &resp); err != nil {
		return err
	}
	if !resp.OK {
		return errors.New(resp.Error)
	}
	return nil
}

func signedRequest(op string, identity *Identity, limit int, ids []string) request {
	at := time.Now().Unix()
	req := request{Version: 1, Op: op, Identity: ptrPublic(identity.Public()), TimeUnix: at, Limit: limit, IDs: append([]string(nil), ids...)}
	req.Signature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(identity.SigningPrivate, authorizationMessage(op, identity.ID.String(), at, req.IDs)))
	return req
}
func ptrPublic(value PublicIdentity) *PublicIdentity { return &value }

func writeFrame(w io.Writer, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(raw) > maxFrame {
		return errors.New("mailbox frame too large")
	}
	var h [4]byte
	binary.BigEndian.PutUint32(h[:], uint32(len(raw)))
	if _, err := w.Write(h[:]); err != nil {
		return err
	}
	_, err = w.Write(raw)
	return err
}
func readFrame(r io.Reader, value any) error {
	var h [4]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return err
	}
	size := int(binary.BigEndian.Uint32(h[:]))
	if size <= 0 || size > maxFrame {
		return errors.New("invalid mailbox frame size")
	}
	raw := make([]byte, size)
	if _, err := io.ReadFull(r, raw); err != nil {
		return err
	}
	return json.Unmarshal(raw, value)
}
