package knotmailbox

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"
)

type pipeDialer struct{ handler func(net.Conn) }

func (d pipeDialer) Dial(context.Context, string) (net.Conn, error) {
	client, server := net.Pipe()
	go func() { defer server.Close(); d.handler(server) }()
	return client, nil
}

func TestIdentityRoundTrip(t *testing.T) {
	identity, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "mailbox.json")
	if err := identity.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != identity.ID || loaded.Public() != identity.Public() {
		t.Fatal("identity changed")
	}
}

func TestSealOpen(t *testing.T) {
	sender, _ := Generate()
	recipient, _ := Generate()
	envelope, err := Seal(sender, recipient.Public(), []byte("hello offline"), time.Now())
	if err != nil {
		t.Fatal(err)
	}
	message, err := Open(recipient, envelope)
	if err != nil {
		t.Fatal(err)
	}
	if string(message.Payload) != "hello offline" || message.Sender.ID != sender.ID.String() {
		t.Fatal("message changed")
	}
	envelope.Ciphertext = envelope.Ciphertext[:len(envelope.Ciphertext)-1] + "A"
	if _, err := Open(recipient, envelope); err == nil {
		t.Fatal("tampering was accepted")
	}
}

func TestMailboxProtocol(t *testing.T) {
	store := &FileStore{Directory: t.TempDir(), TTL: time.Hour, MaxPerRecipient: 10}
	server := Server(store, 1<<20)
	client := Client{Dialer: pipeDialer{handler: server}}
	sender, _ := Generate()
	recipient, _ := Generate()
	id, err := client.Send(context.Background(), "mailbox.knot", sender, recipient.Public(), []byte("queued"))
	if err != nil {
		t.Fatal(err)
	}
	messages, err := client.Fetch(context.Background(), "mailbox.knot", recipient, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 || messages[0].ID != id || string(messages[0].Payload) != "queued" {
		t.Fatalf("unexpected messages %#v", messages)
	}
	if err := client.Ack(context.Background(), "mailbox.knot", recipient, []string{id}); err != nil {
		t.Fatal(err)
	}
	messages, err = client.Fetch(context.Background(), "mailbox.knot", recipient, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 0 {
		t.Fatalf("message not deleted: %#v", messages)
	}
}
