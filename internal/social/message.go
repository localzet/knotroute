package social

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Message struct {
	Version     int            `json:"version"`
	ID          string         `json:"id"`
	Sender      PublicIdentity `json:"sender"`
	SenderNode  string         `json:"sender_node"`
	RecipientID string         `json:"recipient_id"`
	CreatedUnix int64          `json:"created_unix"`
	Body        string         `json:"body"`
	ReplyTo     string         `json:"reply_to,omitempty"`
	Signature   string         `json:"signature"`
}

func NewMessage(identity *Identity, profile PublicIdentity, senderNode, recipientID, body, replyTo string, now time.Time) (Message, error) {
	priv, err := identity.private()
	if err != nil {
		return Message{}, err
	}
	if _, err := profile.Verify(); err != nil || profile.ID != identity.ID {
		return Message{}, errors.New("sender profile does not match identity")
	}
	body = strings.TrimSpace(body)
	if body == "" {
		return Message{}, errors.New("message body is empty")
	}
	if len([]byte(body)) > 64<<10 {
		return Message{}, errors.New("message body is too large")
	}
	m := Message{Version: 1, Sender: profile, SenderNode: strings.TrimSpace(senderNode), RecipientID: strings.TrimSpace(recipientID), CreatedUnix: now.Unix(), Body: body, ReplyTo: replyTo}
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return Message{}, err
	}
	h := sha256.Sum256(append(messageToSign(m), nonce...))
	m.ID = base64.RawURLEncoding.EncodeToString(h[:])
	m.Signature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(priv, messageToSign(m)))
	return m, nil
}

func (m Message) Verify() error {
	if m.Version != 1 || m.ID == "" || m.RecipientID == "" || strings.TrimSpace(m.Body) == "" {
		return errors.New("invalid message")
	}
	pub, err := m.Sender.Verify()
	if err != nil {
		return err
	}
	sig, err := base64.RawStdEncoding.DecodeString(m.Signature)
	if err != nil || !ed25519.Verify(pub, messageToSign(m), sig) {
		return errors.New("invalid message signature")
	}
	return nil
}

func messageToSign(m Message) []byte {
	h := sha256.Sum256([]byte(m.Body))
	return []byte(fmt.Sprintf("knotroute/message/v1|%s|%s|%s|%d|%x|%s", m.Sender.ID, m.SenderNode, m.RecipientID, m.CreatedUnix, h[:], m.ReplyTo))
}
