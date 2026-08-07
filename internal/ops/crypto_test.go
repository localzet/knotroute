package ops

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"
	"time"
)

func TestSignedRequest(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"hello":"world"}`)
	now := time.Now()
	sig := SignRequest(priv, "POST", "/api/v1/agents/heartbeat", now.Unix(), body)
	if err := VerifyRequest(pub, "POST", "/api/v1/agents/heartbeat", now.Unix(), body, sig, now); err != nil {
		t.Fatal(err)
	}
	if err := VerifyRequest(pub, "POST", "/api/v1/agents/heartbeat", now.Unix(), append(body, '!'), sig, now); err == nil {
		t.Fatal("tampered body verified")
	}
}
