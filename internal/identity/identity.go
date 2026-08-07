package identity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/localzet/knotroute/internal/nodeid"
)

type diskIdentity struct {
	Version    int    `json:"version"`
	PrivateKey string `json:"private_key"`
	PublicKey  string `json:"public_key"`
	NodeID     string `json:"node_id"`
}

type Identity struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
	ID         nodeid.ID
}

func Generate() (*Identity, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate identity: %w", err)
	}
	return &Identity{PrivateKey: priv, PublicKey: pub, ID: nodeid.FromPublicKey(pub)}, nil
}

func Load(path string) (*Identity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read identity: %w", err)
	}
	var d diskIdentity
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("parse identity: %w", err)
	}
	raw, err := base64.StdEncoding.DecodeString(d.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("decode private key: %w", err)
	}
	if len(raw) != ed25519.PrivateKeySize {
		return nil, errors.New("identity private key has invalid length")
	}
	priv := ed25519.PrivateKey(raw)
	pub := priv.Public().(ed25519.PublicKey)
	id := nodeid.FromPublicKey(pub)
	if d.NodeID != "" && d.NodeID != id.String() {
		return nil, errors.New("identity node_id does not match private key")
	}
	if d.PublicKey != "" {
		claimed, err := base64.StdEncoding.DecodeString(d.PublicKey)
		if err != nil || !ed25519.PublicKey(claimed).Equal(pub) {
			return nil, errors.New("identity public_key does not match private key")
		}
	}
	return &Identity{PrivateKey: priv, PublicKey: pub, ID: id}, nil
}

func (i *Identity) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil && filepath.Dir(path) != "." {
		return err
	}
	d := diskIdentity{
		Version:    1,
		PrivateKey: base64.StdEncoding.EncodeToString(i.PrivateKey),
		PublicKey:  base64.StdEncoding.EncodeToString(i.PublicKey),
		NodeID:     i.ID.String(),
	}
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write identity: %w", err)
	}
	return os.Chmod(path, 0o600)
}

func (i *Identity) Sign(message []byte) []byte { return ed25519.Sign(i.PrivateKey, message) }

func Verify(publicKey, message, signature []byte) bool {
	return len(publicKey) == ed25519.PublicKeySize && ed25519.Verify(ed25519.PublicKey(publicKey), message, signature)
}
