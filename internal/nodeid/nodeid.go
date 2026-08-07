package nodeid

import (
	"crypto/sha256"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const Prefix = "kr_"

var encoding = base32.StdEncoding.WithPadding(base32.NoPadding)

type ID [32]byte

func FromPublicKey(publicKey []byte) ID {
	return sha256.Sum256(publicKey)
}

func Parse(s string) (ID, error) {
	var id ID
	s = strings.TrimSpace(strings.ToLower(s))
	if !strings.HasPrefix(s, Prefix) {
		return id, fmt.Errorf("node id must start with %q", Prefix)
	}
	raw, err := encoding.DecodeString(strings.ToUpper(strings.TrimPrefix(s, Prefix)))
	if err != nil {
		return id, fmt.Errorf("decode node id: %w", err)
	}
	if len(raw) != len(id) {
		return id, fmt.Errorf("node id has %d bytes, expected %d", len(raw), len(id))
	}
	copy(id[:], raw)
	return id, nil
}

func MustParse(s string) ID {
	id, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return id
}

func (id ID) String() string {
	return Prefix + strings.ToLower(encoding.EncodeToString(id[:]))
}

func (id ID) Short() string {
	s := id.String()
	if len(s) <= 15 {
		return s
	}
	return s[:10] + "…" + s[len(s)-4:]
}

func (id ID) IsZero() bool {
	return id == ID{}
}

func (id ID) MarshalText() ([]byte, error) { return []byte(id.String()), nil }
func (id *ID) UnmarshalText(text []byte) error {
	parsed, err := Parse(string(text))
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}
func (id ID) MarshalJSON() ([]byte, error) { return json.Marshal(id.String()) }
func (id *ID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	return id.UnmarshalText([]byte(s))
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

func FromBytes(raw []byte) (ID, error) {
	var id ID
	if len(raw) != len(id) {
		return id, errors.New("invalid node id length")
	}
	copy(id[:], raw)
	return id, nil
}
