package overlay

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/localzet/knotroute/internal/nodeid"
)

const sessionKeySize = 32

type ephemeralKey struct {
	private *ecdh.PrivateKey
	public  []byte
}

func newEphemeralKey() (ephemeralKey, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return ephemeralKey{}, err
	}
	return ephemeralKey{private: priv, public: priv.PublicKey().Bytes()}, nil
}

func sharedSecret(private *ecdh.PrivateKey, peerPublic []byte) ([]byte, error) {
	pub, err := ecdh.X25519().NewPublicKey(peerPublic)
	if err != nil {
		return nil, err
	}
	return private.ECDH(pub)
}

func deriveSessionKeys(shared, openNonce, ackNonce []byte, streamID [16]byte, src, dst nodeid.ID) (c2s, s2c []byte, err error) {
	saltInput := append(append([]byte(nil), openNonce...), ackNonce...)
	salt := sha256.Sum256(saltInput)
	info := make([]byte, 0, 16+32+32+20)
	info = append(info, []byte("knotroute/session/v1")...)
	info = append(info, streamID[:]...)
	info = append(info, src[:]...)
	info = append(info, dst[:]...)
	material := hkdfSHA256(shared, salt[:], info, sessionKeySize*2)
	return material[:sessionKeySize], material[sessionKeySize:], nil
}

// hkdfSHA256 implements RFC 5869 extract-and-expand for the fixed-size session
// material used by KnotRoute. Keeping this tiny primitive local makes offline
// builds possible without weakening the construction.
func hkdfSHA256(secret, salt, info []byte, length int) []byte {
	extract := hmac.New(sha256.New, salt)
	_, _ = extract.Write(secret)
	prk := extract.Sum(nil)
	out := make([]byte, 0, length)
	previous := []byte(nil)
	for counter := byte(1); len(out) < length; counter++ {
		expand := hmac.New(sha256.New, prk)
		_, _ = expand.Write(previous)
		_, _ = expand.Write(info)
		_, _ = expand.Write([]byte{counter})
		previous = expand.Sum(nil)
		out = append(out, previous...)
	}
	return out[:length]
}

func newAEAD(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func makeNonce(key []byte, seq uint64) []byte {
	digest := sha256.Sum256(append(append([]byte(nil), key...), []byte("nonce")...))
	nonce := make([]byte, 12)
	copy(nonce[:4], digest[:4])
	binary.BigEndian.PutUint64(nonce[4:], seq)
	return nonce
}

func makeAAD(streamID [16]byte, src, dst nodeid.ID, seq uint64) []byte {
	aad := make([]byte, 0, 16+32+32+8)
	aad = append(aad, streamID[:]...)
	aad = append(aad, src[:]...)
	aad = append(aad, dst[:]...)
	var seqBytes [8]byte
	binary.BigEndian.PutUint64(seqBytes[:], seq)
	return append(aad, seqBytes[:]...)
}

func seal(key []byte, streamID [16]byte, src, dst nodeid.ID, seq uint64, plaintext []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	return aead.Seal(nil, makeNonce(key, seq), plaintext, makeAAD(streamID, src, dst, seq)), nil
}

func openCiphertext(key []byte, streamID [16]byte, src, dst nodeid.ID, seq uint64, ciphertext []byte) ([]byte, error) {
	aead, err := newAEAD(key)
	if err != nil {
		return nil, err
	}
	plaintext, err := aead.Open(nil, makeNonce(key, seq), ciphertext, makeAAD(streamID, src, dst, seq))
	if err != nil {
		return nil, fmt.Errorf("decrypt stream data: %w", err)
	}
	return plaintext, nil
}
