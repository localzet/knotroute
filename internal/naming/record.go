package naming

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/localzet/knotroute/internal/identity"
	"github.com/localzet/knotroute/internal/nodeid"
)

const AliasRecordVersion = 1

type AliasRecordBody struct {
	Version     int    `json:"version"`
	Name        string `json:"name"`
	Node        string `json:"node"`
	PublicKey   string `json:"public_key"`
	Description string `json:"description,omitempty"`
	IssuedUnix  int64  `json:"issued_unix"`
	ExpiresUnix int64  `json:"expires_unix,omitempty"`
}

type AliasRecord struct {
	AliasRecordBody
	Signature string `json:"signature"`
}

func SignAliasRecord(id *identity.Identity, name, description string, now time.Time, validity time.Duration) (AliasRecord, error) {
	alias := Alias{Name: strings.ToLower(strings.TrimSpace(name)), Node: id.ID.String(), Description: strings.TrimSpace(description)}
	if err := ValidateAlias(alias); err != nil {
		return AliasRecord{}, err
	}
	body := AliasRecordBody{
		Version: AliasRecordVersion, Name: alias.Name, Node: id.ID.String(),
		PublicKey: base64.StdEncoding.EncodeToString(id.PublicKey), Description: alias.Description,
		IssuedUnix: now.UTC().Unix(),
	}
	if validity > 0 {
		body.ExpiresUnix = now.UTC().Add(validity).Unix()
	}
	canonical, err := json.Marshal(body)
	if err != nil {
		return AliasRecord{}, err
	}
	return AliasRecord{AliasRecordBody: body, Signature: base64.StdEncoding.EncodeToString(id.Sign(canonical))}, nil
}

func (r AliasRecord) Verify(now time.Time) (Alias, error) {
	if r.Version != AliasRecordVersion {
		return Alias{}, fmt.Errorf("unsupported alias record version %d", r.Version)
	}
	id, err := nodeid.Parse(r.Node)
	if err != nil {
		return Alias{}, err
	}
	pub, err := base64.StdEncoding.DecodeString(r.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return Alias{}, errors.New("invalid alias public key")
	}
	if nodeid.FromPublicKey(pub) != id {
		return Alias{}, errors.New("alias node does not match public key")
	}
	if r.IssuedUnix > now.Add(5*time.Minute).Unix() {
		return Alias{}, errors.New("alias record is from the future")
	}
	if r.ExpiresUnix != 0 && r.ExpiresUnix <= now.Unix() {
		return Alias{}, errors.New("alias record expired")
	}
	sig, err := base64.StdEncoding.DecodeString(r.Signature)
	if err != nil {
		return Alias{}, errors.New("invalid alias signature encoding")
	}
	canonical, err := json.Marshal(r.AliasRecordBody)
	if err != nil {
		return Alias{}, err
	}
	if !identity.Verify(pub, canonical, sig) {
		return Alias{}, errors.New("invalid alias signature")
	}
	alias := Alias{Name: strings.ToLower(strings.TrimSpace(r.Name)), Node: id.String(), Description: strings.TrimSpace(r.Description)}
	if err := ValidateAlias(alias); err != nil {
		return Alias{}, err
	}
	return alias, nil
}
