package knotmailbox

import (
	"crypto/cipher"
	"crypto/ed25519"
)

func newGCM(block cipher.Block) (cipher.AEAD, error) { return cipher.NewGCM(block) }
func ed25519Sign(private ed25519.PrivateKey, message []byte) []byte {
	return ed25519.Sign(private, message)
}
func ed25519Verify(public, message, signature []byte) bool {
	return len(public) == ed25519.PublicKeySize && ed25519.Verify(ed25519.PublicKey(public), message, signature)
}
