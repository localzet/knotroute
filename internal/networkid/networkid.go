package networkid

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"strings"
)

const Prefix = "kn_"

type ID [32]byte

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

func FromSeed(seed string) ID { return sha256.Sum256([]byte("knotroute/network/v3|" + seed)) }

func Default() ID { return FromSeed("public") }

func Random() (ID, error) {
	var id ID
	_, err := rand.Read(id[:])
	return id, err
}

func (id ID) String() string { return Prefix + strings.ToLower(b32.EncodeToString(id[:])) }

func Parse(s string) (ID, error) {
	var id ID
	s = strings.TrimSpace(strings.ToLower(s))
	if !strings.HasPrefix(s, Prefix) {
		return id, errors.New("network id must start with kn_")
	}
	raw, err := b32.DecodeString(strings.ToUpper(strings.TrimPrefix(s, Prefix)))
	if err != nil || len(raw) != len(id) {
		return id, errors.New("invalid network id")
	}
	copy(id[:], raw)
	return id, nil
}
