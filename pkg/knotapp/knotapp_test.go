package knotapp

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"
)

type pipeDialer struct{ handler func(net.Conn) }

func (d pipeDialer) Dial(context.Context, string) (net.Conn, error) {
	client, server := net.Pipe()
	go func() {
		defer server.Close()
		d.handler(server)
	}()
	return client, nil
}

func TestRPC(t *testing.T) {
	handler := RPCServer(RPCHandlerFunc(func(_ context.Context, method string, raw json.RawMessage) (any, error) {
		if method != "sum" {
			t.Fatalf("unexpected method %q", method)
		}
		var in struct{ A, B int }
		if err := json.Unmarshal(raw, &in); err != nil {
			return nil, err
		}
		return struct {
			Value int `json:"value"`
		}{in.A + in.B}, nil
	}))
	client := RPCClient{Dialer: pipeDialer{handler: handler}}
	var out struct {
		Value int `json:"value"`
	}
	if err := client.Call(context.Background(), "unused.knot", "sum", struct{ A, B int }{2, 5}, &out); err != nil {
		t.Fatal(err)
	}
	if out.Value != 7 {
		t.Fatalf("got %d", out.Value)
	}
}

func TestDatagram(t *testing.T) {
	got := make(chan string, 1)
	server := DatagramServer(1024, func(raw []byte) { got <- string(raw) })
	client := DatagramClient{Dialer: pipeDialer{handler: server}, Limit: 1024}
	if err := client.Send(context.Background(), "unused.knot", []byte("hello")); err != nil {
		t.Fatal(err)
	}
	select {
	case value := <-got:
		if value != "hello" {
			t.Fatalf("got %q", value)
		}
	case <-time.After(time.Second):
		t.Fatal("datagram not delivered")
	}
}

func TestPubSubEncryptedTopic(t *testing.T) {
	broker := NewPubSubBroker(1024)
	dialer := pipeDialer{handler: broker.Handler}
	client := PubSubClient{Dialer: dialer}
	topic, err := NewTopic()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	messages, errorsCh, err := client.Subscribe(ctx, "unused.knot", topic, 2)
	if err != nil {
		t.Fatal(err)
	}
	// Give the broker goroutine a chance to register the subscription.
	time.Sleep(10 * time.Millisecond)
	if err := client.Publish(context.Background(), "unused.knot", topic, []byte("secret event")); err != nil {
		t.Fatal(err)
	}
	select {
	case value := <-messages:
		if string(value) != "secret event" {
			t.Fatalf("got %q", value)
		}
	case err := <-errorsCh:
		t.Fatalf("subscriber failed: %v", err)
	case <-time.After(time.Second):
		t.Fatal("pubsub event not delivered")
	}
}

func TestObjectStoreAndProtocol(t *testing.T) {
	store := FileObjectStore{Directory: t.TempDir(), MaxBytes: 1 << 20}
	server := ObjectServer(store, 1<<20)
	client := ObjectClient{Dialer: pipeDialer{handler: server}, MaxBytes: 1 << 20}
	raw := []byte("content addressed over KnotRoute")
	id, err := client.Put(context.Background(), "unused.knot", raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := client.Get(context.Background(), "unused.knot", id)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(raw) || id != ObjectIDFor(raw) {
		t.Fatal("object changed")
	}
	parsed, err := ParseObjectID(id.String())
	if err != nil || parsed != id {
		t.Fatalf("id roundtrip: %v", err)
	}
}
