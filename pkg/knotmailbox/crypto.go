package knotmailbox

import (
	"crypto/aes"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

type Envelope struct {
	Version      int    `json:"version"`
	RecipientID  string `json:"recipient_id"`
	EphemeralKey string `json:"ephemeral_key"`
	Nonce        string `json:"nonce"`
	Ciphertext   string `json:"ciphertext"`
	MessageID    string `json:"message_id"`
	CreatedUnix  int64  `json:"created_unix"`
}

type innerMessage struct {
	Sender    PublicIdentity `json:"sender"`
	Created   int64          `json:"created_unix"`
	Payload   string         `json:"payload"`
	Signature string         `json:"signature"`
}

type Message struct {
	ID      string
	Sender  PublicIdentity
	Created time.Time
	Payload []byte
}

func Seal(sender *Identity, recipient PublicIdentity, payload []byte, now time.Time) (Envelope, error) {
	if sender == nil {
		return Envelope{}, errors.New("sender identity is nil")
	}
	recipientID, _, recipientKey, err := recipient.Verify()
	if err != nil {
		return Envelope{}, err
	}
	created := now.UTC().Unix()
	inner := innerMessage{Sender: sender.Public(), Created: created, Payload: base64.RawStdEncoding.EncodeToString(payload)}
	inner.Signature = base64.RawStdEncoding.EncodeToString(edSign(sender, recipientID.String(), created, payload))
	plain, err := json.Marshal(inner)
	if err != nil {
		return Envelope{}, err
	}
	ephemeral, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return Envelope{}, err
	}
	shared, err := ephemeral.ECDH(recipientKey)
	if err != nil {
		return Envelope{}, err
	}
	key := hkdf(shared, []byte(recipientID.String()), []byte("knotroute/mailbox/envelope/v1"), 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		return Envelope{}, err
	}
	aead, err := newGCM(block)
	if err != nil {
		return Envelope{}, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return Envelope{}, err
	}
	ciphertext := aead.Seal(nil, nonce, plain, []byte(recipientID.String()))
	messageHash := sha256.New()
	messageHash.Write(ephemeral.PublicKey().Bytes())
	messageHash.Write(nonce)
	messageHash.Write(ciphertext)
	return Envelope{Version: 1, RecipientID: recipientID.String(), EphemeralKey: base64.RawStdEncoding.EncodeToString(ephemeral.PublicKey().Bytes()), Nonce: base64.RawStdEncoding.EncodeToString(nonce), Ciphertext: base64.RawStdEncoding.EncodeToString(ciphertext), MessageID: base64.RawURLEncoding.EncodeToString(messageHash.Sum(nil)), CreatedUnix: created}, nil
}

func Open(recipient *Identity, envelope Envelope) (Message, error) {
	if recipient == nil {
		return Message{}, errors.New("recipient identity is nil")
	}
	if envelope.Version != 1 || envelope.RecipientID != recipient.ID.String() {
		return Message{}, errors.New("mailbox envelope recipient mismatch")
	}
	ephemeralRaw, err := base64.RawStdEncoding.DecodeString(envelope.EphemeralKey)
	if err != nil || len(ephemeralRaw) != 32 {
		return Message{}, errors.New("invalid envelope ephemeral key")
	}
	ephemeral, err := ecdh.X25519().NewPublicKey(ephemeralRaw)
	if err != nil {
		return Message{}, err
	}
	nonce, err := base64.RawStdEncoding.DecodeString(envelope.Nonce)
	if err != nil {
		return Message{}, errors.New("invalid envelope nonce")
	}
	ciphertext, err := base64.RawStdEncoding.DecodeString(envelope.Ciphertext)
	if err != nil {
		return Message{}, errors.New("invalid envelope ciphertext")
	}
	h := sha256.New()
	h.Write(ephemeralRaw)
	h.Write(nonce)
	h.Write(ciphertext)
	if base64.RawURLEncoding.EncodeToString(h.Sum(nil)) != envelope.MessageID {
		return Message{}, errors.New("mailbox envelope id mismatch")
	}
	shared, err := recipient.EncryptionPrivate.ECDH(ephemeral)
	if err != nil {
		return Message{}, err
	}
	key := hkdf(shared, []byte(recipient.ID.String()), []byte("knotroute/mailbox/envelope/v1"), 32)
	block, err := aes.NewCipher(key)
	if err != nil {
		return Message{}, err
	}
	aead, err := newGCM(block)
	if err != nil {
		return Message{}, err
	}
	if len(nonce) != aead.NonceSize() {
		return Message{}, errors.New("invalid envelope nonce size")
	}
	plain, err := aead.Open(nil, nonce, ciphertext, []byte(recipient.ID.String()))
	if err != nil {
		return Message{}, errors.New("decrypt mailbox envelope")
	}
	var inner innerMessage
	if err := json.Unmarshal(plain, &inner); err != nil {
		return Message{}, err
	}
	_, signing, _, err := inner.Sender.Verify()
	if err != nil {
		return Message{}, err
	}
	payload, err := base64.RawStdEncoding.DecodeString(inner.Payload)
	if err != nil {
		return Message{}, errors.New("invalid mailbox payload")
	}
	sig, err := base64.RawStdEncoding.DecodeString(inner.Signature)
	if err != nil || !edVerify(signing, recipient.ID.String(), inner.Created, payload, sig) {
		return Message{}, errors.New("invalid mailbox sender signature")
	}
	return Message{ID: envelope.MessageID, Sender: inner.Sender, Created: time.Unix(inner.Created, 0).UTC(), Payload: payload}, nil
}

func edSign(sender *Identity, recipient string, created int64, payload []byte) []byte {
	return ed25519Sign(sender.SigningPrivate, messageToSign(recipient, created, payload))
}
func edVerify(public []byte, recipient string, created int64, payload, signature []byte) bool {
	return ed25519Verify(public, messageToSign(recipient, created, payload), signature)
}
func messageToSign(recipient string, created int64, payload []byte) []byte {
	h := sha256.Sum256(payload)
	return []byte(fmt.Sprintf("knotroute/mailbox/message/v1|%s|%d|%x", recipient, created, h[:]))
}

func hkdf(secret, salt, info []byte, length int) []byte {
	extract := hmac.New(sha256.New, salt)
	extract.Write(secret)
	prk := extract.Sum(nil)
	out := make([]byte, 0, length)
	var previous []byte
	for counter := byte(1); len(out) < length; counter++ {
		expand := hmac.New(sha256.New, prk)
		expand.Write(previous)
		expand.Write(info)
		expand.Write([]byte{counter})
		previous = expand.Sum(nil)
		out = append(out, previous...)
	}
	return out[:length]
}
