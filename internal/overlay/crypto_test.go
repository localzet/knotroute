package overlay

import (
	"crypto/rand"
	"testing"

	"github.com/localzet/knotroute/internal/nodeid"
)

func TestSessionCrypto(t *testing.T) {
	a, _ := newEphemeralKey()
	b, _ := newEphemeralKey()
	ab, err := sharedSecret(a.private, b.public)
	if err != nil {
		t.Fatal(err)
	}
	ba, err := sharedSecret(b.private, a.public)
	if err != nil {
		t.Fatal(err)
	}
	if string(ab) != string(ba) {
		t.Fatal("shared secrets differ")
	}
	openNonce := make([]byte, 32)
	ackNonce := make([]byte, 32)
	rand.Read(openNonce)
	rand.Read(ackNonce)
	var sid [16]byte
	rand.Read(sid[:])
	var src, dst nodeid.ID
	rand.Read(src[:])
	rand.Read(dst[:])
	c2s, _, err := deriveSessionKeys(ab, openNonce, ackNonce, sid, src, dst)
	if err != nil {
		t.Fatal(err)
	}
	ciphertext, err := seal(c2s, sid, src, dst, 7, []byte("hello"))
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := openCiphertext(c2s, sid, src, dst, 7, ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if string(plaintext) != "hello" {
		t.Fatal("wrong plaintext")
	}
}
