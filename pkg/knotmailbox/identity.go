package knotmailbox

import (
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var idEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

type ID [32]byte

func (id ID) String() string { return "km_" + strings.ToLower(idEncoding.EncodeToString(id[:])) }
func ParseID(value string) (ID, error) {
	var id ID
	if !strings.HasPrefix(value, "km_") {
		return id, errors.New("invalid mailbox id prefix")
	}
	raw, err := idEncoding.DecodeString(strings.ToUpper(strings.TrimPrefix(value, "km_")))
	if err != nil || len(raw) != len(id) {
		return id, errors.New("invalid mailbox id")
	}
	copy(id[:], raw)
	return id, nil
}

type PublicIdentity struct {
	ID            string `json:"id"`
	SigningKey    string `json:"signing_key"`
	EncryptionKey string `json:"encryption_key"`
}

type Identity struct {
	SigningPrivate    ed25519.PrivateKey
	SigningPublic     ed25519.PublicKey
	EncryptionPrivate *ecdh.PrivateKey
	EncryptionPublic  *ecdh.PublicKey
	ID                ID
}

type diskIdentity struct {
	Version           int    `json:"version"`
	SigningPrivate    string `json:"signing_private"`
	EncryptionPrivate string `json:"encryption_private"`
}

func Generate() (*Identity, error) {
	signPub, signPriv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	encPriv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return newIdentity(signPriv, signPub, encPriv), nil
}

func newIdentity(signPriv ed25519.PrivateKey, signPub ed25519.PublicKey, encPriv *ecdh.PrivateKey) *Identity {
	encPub := encPriv.PublicKey()
	return &Identity{SigningPrivate: signPriv, SigningPublic: signPub, EncryptionPrivate: encPriv, EncryptionPublic: encPub, ID: publicID(signPub, encPub.Bytes())}
}

func publicID(signing, encryption []byte) ID {
	payload := make([]byte, 0, len(signing)+len(encryption)+24)
	payload = append(payload, []byte("knotroute/mailbox/v1|")...)
	payload = append(payload, signing...)
	payload = append(payload, encryption...)
	return sha256.Sum256(payload)
}

func (i *Identity) Public() PublicIdentity {
	return PublicIdentity{ID: i.ID.String(), SigningKey: base64.RawStdEncoding.EncodeToString(i.SigningPublic), EncryptionKey: base64.RawStdEncoding.EncodeToString(i.EncryptionPublic.Bytes())}
}

func (p PublicIdentity) Verify() (ID, ed25519.PublicKey, *ecdh.PublicKey, error) {
	id, err := ParseID(p.ID)
	if err != nil {
		return ID{}, nil, nil, err
	}
	signing, err := base64.RawStdEncoding.DecodeString(p.SigningKey)
	if err != nil || len(signing) != ed25519.PublicKeySize {
		return ID{}, nil, nil, errors.New("invalid mailbox signing key")
	}
	encryption, err := base64.RawStdEncoding.DecodeString(p.EncryptionKey)
	if err != nil || len(encryption) != 32 {
		return ID{}, nil, nil, errors.New("invalid mailbox encryption key")
	}
	encPub, err := ecdh.X25519().NewPublicKey(encryption)
	if err != nil {
		return ID{}, nil, nil, errors.New("invalid mailbox encryption key")
	}
	if publicID(signing, encryption) != id {
		return ID{}, nil, nil, errors.New("mailbox id does not match public keys")
	}
	return id, ed25519.PublicKey(signing), encPub, nil
}

func (i *Identity) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil && filepath.Dir(path) != "." {
		return err
	}
	raw, err := json.MarshalIndent(diskIdentity{Version: 1, SigningPrivate: base64.RawStdEncoding.EncodeToString(i.SigningPrivate), EncryptionPrivate: base64.RawStdEncoding.EncodeToString(i.EncryptionPrivate.Bytes())}, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func Load(path string) (*Identity, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var disk diskIdentity
	if err := json.Unmarshal(raw, &disk); err != nil {
		return nil, err
	}
	if disk.Version != 1 {
		return nil, fmt.Errorf("unsupported mailbox identity version %d", disk.Version)
	}
	signing, err := base64.RawStdEncoding.DecodeString(disk.SigningPrivate)
	if err != nil || len(signing) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid mailbox signing private key")
	}
	encRaw, err := base64.RawStdEncoding.DecodeString(disk.EncryptionPrivate)
	if err != nil || len(encRaw) != 32 {
		return nil, errors.New("invalid mailbox encryption private key")
	}
	encPriv, err := ecdh.X25519().NewPrivateKey(encRaw)
	if err != nil {
		return nil, err
	}
	signPriv := ed25519.PrivateKey(signing)
	signPub := signPriv.Public().(ed25519.PublicKey)
	return newIdentity(signPriv, signPub, encPriv), nil
}
