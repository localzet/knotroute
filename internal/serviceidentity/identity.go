package serviceidentity

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/localzet/knotroute/internal/serviceid"
	"os"
	"path/filepath"
)

type Identity struct {
	ID         serviceid.ID
	PublicKey  ed25519.PublicKey
	PrivateKey ed25519.PrivateKey
}
type disk struct {
	Version    int    `json:"version"`
	PublicKey  string `json:"public_key"`
	PrivateKey string `json:"private_key"`
}

func Generate() (*Identity, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &Identity{ID: serviceid.FromPublicKey(pub), PublicKey: pub, PrivateKey: priv}, nil
}
func Load(path string) (*Identity, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var d disk
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, err
	}
	if d.Version != 1 {
		return nil, fmt.Errorf("unsupported service identity version %d", d.Version)
	}
	pub, err := base64.StdEncoding.DecodeString(d.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return nil, errors.New("invalid service public key")
	}
	priv, err := base64.StdEncoding.DecodeString(d.PrivateKey)
	if err != nil || len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid service private key")
	}
	if !ed25519.PrivateKey(priv).Public().(ed25519.PublicKey).Equal(ed25519.PublicKey(pub)) {
		return nil, errors.New("service keypair mismatch")
	}
	return &Identity{ID: serviceid.FromPublicKey(pub), PublicKey: ed25519.PublicKey(pub), PrivateKey: ed25519.PrivateKey(priv)}, nil
}
func LoadOrCreate(path string) (*Identity, error) {
	if x, err := Load(path); err == nil {
		return x, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	x, err := Generate()
	if err != nil {
		return nil, err
	}
	if err := x.Save(path); err != nil {
		return nil, err
	}
	return x, nil
}
func (i *Identity) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil && filepath.Dir(path) != "." {
		return err
	}
	d := disk{Version: 1, PublicKey: base64.StdEncoding.EncodeToString(i.PublicKey), PrivateKey: base64.StdEncoding.EncodeToString(i.PrivateKey)}
	raw, _ := json.MarshalIndent(d, "", "  ")
	raw = append(raw, '\n')
	return os.WriteFile(path, raw, 0o600)
}
func (i *Identity) Sign(msg []byte) []byte { return ed25519.Sign(i.PrivateKey, msg) }
