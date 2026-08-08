package social

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const userPrefix = "ku_"

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

type Identity struct {
	ID         string `json:"id"`
	PrivateKey string `json:"private_key"`
}

type PublicIdentity struct {
	Version     int    `json:"version"`
	ID          string `json:"id"`
	SigningKey  string `json:"signing_key"`
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio,omitempty"`
	AvatarHash  string `json:"avatar_hash,omitempty"`
	UpdatedUnix int64  `json:"updated_unix"`
	Signature   string `json:"signature"`
}

func Generate() (*Identity, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return &Identity{ID: idFromPublic(pub), PrivateKey: base64.RawStdEncoding.EncodeToString(priv)}, nil
}

func LoadOrCreate(path string) (*Identity, error) {
	raw, err := os.ReadFile(path)
	if err == nil {
		var id Identity
		if json.Unmarshal(raw, &id) != nil {
			return nil, errors.New("invalid user identity JSON")
		}
		if _, err := id.private(); err != nil {
			return nil, err
		}
		return &id, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	id, err := Generate()
	if err != nil {
		return nil, err
	}
	if err := id.Save(path); err != nil {
		return nil, err
	}
	return id, nil
}

func (i *Identity) Save(path string) error {
	if _, err := i.private(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil && filepath.Dir(path) != "." {
		return err
	}
	raw, err := json.MarshalIndent(i, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}

func (i *Identity) private() (ed25519.PrivateKey, error) {
	if i == nil {
		return nil, errors.New("user identity is nil")
	}
	raw, err := base64.RawStdEncoding.DecodeString(i.PrivateKey)
	if err != nil || len(raw) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid user private key")
	}
	priv := ed25519.PrivateKey(raw)
	if idFromPublic(priv.Public().(ed25519.PublicKey)) != i.ID {
		return nil, errors.New("user identity id does not match private key")
	}
	return priv, nil
}

func (i *Identity) Public(displayName, bio, avatarHash string, now time.Time) (PublicIdentity, error) {
	priv, err := i.private()
	if err != nil {
		return PublicIdentity{}, err
	}
	pub := priv.Public().(ed25519.PublicKey)
	p := PublicIdentity{
		Version: 1, ID: i.ID, SigningKey: base64.RawStdEncoding.EncodeToString(pub),
		DisplayName: strings.TrimSpace(displayName), Bio: strings.TrimSpace(bio), AvatarHash: strings.TrimSpace(avatarHash), UpdatedUnix: now.Unix(),
	}
	if p.DisplayName == "" {
		p.DisplayName = "KnotRoute user"
	}
	p.Signature = base64.RawStdEncoding.EncodeToString(ed25519.Sign(priv, profileMessage(p)))
	return p, nil
}

func (p PublicIdentity) Verify() (ed25519.PublicKey, error) {
	if p.Version != 1 || !strings.HasPrefix(p.ID, userPrefix) {
		return nil, errors.New("unsupported user identity")
	}
	pub, err := base64.RawStdEncoding.DecodeString(p.SigningKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return nil, errors.New("invalid user signing key")
	}
	if idFromPublic(pub) != p.ID {
		return nil, errors.New("user id does not match signing key")
	}
	sig, err := base64.RawStdEncoding.DecodeString(p.Signature)
	if err != nil || !ed25519.Verify(ed25519.PublicKey(pub), profileMessage(p), sig) {
		return nil, errors.New("invalid user profile signature")
	}
	return ed25519.PublicKey(pub), nil
}

func idFromPublic(pub []byte) string {
	sum := sha256.Sum256(pub)
	return userPrefix + strings.ToLower(b32.EncodeToString(sum[:]))
}

func profileMessage(p PublicIdentity) []byte {
	return []byte(strings.Join([]string{
		"knotroute/user-profile/v1", p.ID, p.SigningKey, p.DisplayName, p.Bio, p.AvatarHash,
		formatInt(p.UpdatedUnix),
	}, "\n"))
}
