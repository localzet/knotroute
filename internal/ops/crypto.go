package ops

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var idEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

func NewNetworkID() string {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		sum := sha256.Sum256([]byte(fmt.Sprintf("knotroute/network/v3|%d", time.Now().UnixNano())))
		raw = sum[:]
	}
	return "kn_" + strings.ToLower(idEncoding.EncodeToString(raw))
}

func NewID(prefix string) string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		stamp := sha256.Sum256([]byte(fmt.Sprintf("%s|%d", prefix, time.Now().UnixNano())))
		copy(raw[:], stamp[:16])
	}
	return prefix + "_" + strings.ToLower(idEncoding.EncodeToString(raw[:]))
}

func AgentID(public ed25519.PublicKey) string {
	sum := sha256.Sum256(append([]byte("knotroute/agent/v1|"), public...))
	return "ka_" + strings.ToLower(idEncoding.EncodeToString(sum[:20]))
}

func SignRequest(private ed25519.PrivateKey, method, path string, timestamp int64, body []byte) string {
	sig := ed25519.Sign(private, RequestMessage(method, path, timestamp, body))
	return base64.RawStdEncoding.EncodeToString(sig)
}

func VerifyRequest(public ed25519.PublicKey, method, path string, timestamp int64, body []byte, signature string, now time.Time) error {
	if delta := now.Unix() - timestamp; delta > 180 || delta < -180 {
		return errors.New("request timestamp outside allowed window")
	}
	raw, err := base64.RawStdEncoding.DecodeString(signature)
	if err != nil || len(raw) != ed25519.SignatureSize {
		return errors.New("invalid request signature")
	}
	if !ed25519.Verify(public, RequestMessage(method, path, timestamp, body), raw) {
		return errors.New("invalid request signature")
	}
	return nil
}

func RequestMessage(method, path string, timestamp int64, body []byte) []byte {
	hash := sha256.Sum256(body)
	return []byte(strings.Join([]string{
		"knotroute/ops-request/v1",
		strings.ToUpper(method),
		path,
		strconv.FormatInt(timestamp, 10),
		hex.EncodeToString(hash[:]),
	}, "\n"))
}
