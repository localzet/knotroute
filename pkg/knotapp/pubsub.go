package knotapp

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io"
	"net"
	"sync"
)

type Topic [32]byte

func NewTopic() (Topic, error) {
	var topic Topic
	_, err := rand.Read(topic[:])
	return topic, err
}

func ParseTopic(value string) (Topic, error) {
	var topic Topic
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(raw) != len(topic) {
		return topic, errors.New("invalid pubsub topic secret")
	}
	copy(topic[:], raw)
	return topic, nil
}

func (t Topic) String() string { return base64.RawURLEncoding.EncodeToString(t[:]) }
func (t Topic) ID() string {
	hash := sha256.Sum256(append([]byte("knotroute/pubsub/topic/v1|"), t[:]...))
	return base64.RawURLEncoding.EncodeToString(hash[:])
}

func (t Topic) seal(payload []byte) ([]byte, error) {
	block, err := aes.NewCipher(t[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return append(nonce, aead.Seal(nil, nonce, payload, []byte(t.ID()))...), nil
}
func (t Topic) open(payload []byte) ([]byte, error) {
	block, err := aes.NewCipher(t[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(payload) < aead.NonceSize() {
		return nil, errors.New("pubsub ciphertext is truncated")
	}
	nonce := payload[:aead.NonceSize()]
	return aead.Open(nil, nonce, payload[aead.NonceSize():], []byte(t.ID()))
}

type pubSubMessage struct {
	Version int    `json:"version"`
	Op      string `json:"op"`
	Topic   string `json:"topic"`
	Data    string `json:"data,omitempty"`
}

type PubSubBroker struct {
	mu          sync.Mutex
	subscribers map[string]map[*pubSubSubscriber]struct{}
	maxMessage  int
}

type pubSubSubscriber struct {
	conn net.Conn
	mu   sync.Mutex
}

func NewPubSubBroker(maxMessage int) *PubSubBroker {
	if maxMessage <= 0 {
		maxMessage = 1 << 20
	}
	return &PubSubBroker{subscribers: map[string]map[*pubSubSubscriber]struct{}{}, maxMessage: maxMessage}
}

func (b *PubSubBroker) Handler(conn net.Conn) {
	var first pubSubMessage
	if err := readJSON(conn, &first); err != nil || first.Version != 1 || first.Topic == "" {
		return
	}
	switch first.Op {
	case "publish":
		raw, err := base64.RawStdEncoding.DecodeString(first.Data)
		if err != nil || len(raw) > b.maxMessage {
			return
		}
		b.publish(first.Topic, raw)
	case "subscribe":
		sub := &pubSubSubscriber{conn: conn}
		b.mu.Lock()
		set := b.subscribers[first.Topic]
		if set == nil {
			set = map[*pubSubSubscriber]struct{}{}
			b.subscribers[first.Topic] = set
		}
		set[sub] = struct{}{}
		b.mu.Unlock()
		defer func() {
			b.mu.Lock()
			delete(set, sub)
			if len(set) == 0 {
				delete(b.subscribers, first.Topic)
			}
			b.mu.Unlock()
		}()
		var sink [1]byte
		_, _ = conn.Read(sink[:])
	}
}

func (b *PubSubBroker) publish(topic string, raw []byte) {
	b.mu.Lock()
	subs := make([]*pubSubSubscriber, 0, len(b.subscribers[topic]))
	for sub := range b.subscribers[topic] {
		subs = append(subs, sub)
	}
	b.mu.Unlock()
	message := pubSubMessage{Version: 1, Op: "event", Topic: topic, Data: base64.RawStdEncoding.EncodeToString(raw)}
	for _, sub := range subs {
		sub.mu.Lock()
		err := writeJSON(sub.conn, message)
		sub.mu.Unlock()
		if err != nil {
			_ = sub.conn.Close()
		}
	}
}

type PubSubClient struct{ Dialer Dialer }

func (c PubSubClient) Publish(ctx context.Context, address string, topic Topic, payload []byte) error {
	if c.Dialer == nil {
		return errors.New("pubsub dialer is nil")
	}
	sealed, err := topic.seal(payload)
	if err != nil {
		return err
	}
	conn, err := c.Dialer.Dial(ctx, address)
	if err != nil {
		return err
	}
	defer conn.Close()
	return writeJSON(conn, pubSubMessage{Version: 1, Op: "publish", Topic: topic.ID(), Data: base64.RawStdEncoding.EncodeToString(sealed)})
}

func (c PubSubClient) Subscribe(ctx context.Context, address string, topic Topic, buffer int) (<-chan []byte, <-chan error, error) {
	if c.Dialer == nil {
		return nil, nil, errors.New("pubsub dialer is nil")
	}
	if buffer <= 0 {
		buffer = 32
	}
	conn, err := c.Dialer.Dial(ctx, address)
	if err != nil {
		return nil, nil, err
	}
	if err := writeJSON(conn, pubSubMessage{Version: 1, Op: "subscribe", Topic: topic.ID()}); err != nil {
		_ = conn.Close()
		return nil, nil, err
	}
	messages := make(chan []byte, buffer)
	errorsCh := make(chan error, 1)
	go func() {
		defer close(messages)
		defer close(errorsCh)
		defer conn.Close()
		go func() {
			<-ctx.Done()
			_ = conn.Close()
		}()
		for {
			var event pubSubMessage
			if err := readJSON(conn, &event); err != nil {
				if ctx.Err() == nil && !errors.Is(err, io.EOF) {
					errorsCh <- err
				}
				return
			}
			if event.Version != 1 || event.Op != "event" || event.Topic != topic.ID() {
				continue
			}
			sealed, err := base64.RawStdEncoding.DecodeString(event.Data)
			if err != nil {
				continue
			}
			plain, err := topic.open(sealed)
			if err != nil {
				continue
			}
			select {
			case messages <- plain:
			case <-ctx.Done():
				return
			}
		}
	}()
	return messages, errorsCh, nil
}
