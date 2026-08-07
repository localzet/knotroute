package serviceid

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const Prefix = "ks_"

var encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

type ID [32]byte

func FromPublicKey(pub []byte) ID { return sha256.Sum256(pub) }
func (id ID) String() string      { return Prefix + strings.ToLower(encoding.EncodeToString(id[:])) }
func (id ID) Short() string {
	s := id.String()
	if len(s) <= 15 {
		return s
	}
	return s[:10] + "…" + s[len(s)-4:]
}
func Parse(s string) (ID, error) {
	var id ID
	s = strings.TrimSpace(strings.ToLower(s))
	if !strings.HasPrefix(s, Prefix) {
		return id, fmt.Errorf("service id must start with %q", Prefix)
	}
	raw, err := encoding.DecodeString(strings.ToUpper(strings.TrimPrefix(s, Prefix)))
	if err != nil {
		return id, fmt.Errorf("decode service id: %w", err)
	}
	if len(raw) != len(id) {
		return id, errors.New("invalid service id length")
	}
	copy(id[:], raw)
	return id, nil
}
func FromBytes(raw []byte) (ID, error) {
	var id ID
	if len(raw) != len(id) {
		return id, errors.New("invalid service id length")
	}
	copy(id[:], raw)
	return id, nil
}
func Compare(a, b ID) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}
func (id ID) MarshalJSON() ([]byte, error) { return json.Marshal(id.String()) }
func (id *ID) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	x, err := Parse(s)
	if err == nil {
		*id = x
	}
	return err
}
