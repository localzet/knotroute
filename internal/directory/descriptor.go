package directory

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/localzet/knotroute/internal/networkid"
	"github.com/localzet/knotroute/internal/nodeid"
	"github.com/localzet/knotroute/internal/serviceid"
	"github.com/localzet/knotroute/internal/serviceidentity"
	"sort"
	"time"
)

const DescriptorVersion = 1

type DescriptorBody struct {
	Version            int               `json:"version"`
	NetworkID          string            `json:"network_id"`
	ServiceID          string            `json:"service_id"`
	PublicKey          string            `json:"public_key"`
	IntroductionPoints []string          `json:"introduction_points"`
	Revision           uint64            `json:"revision"`
	PublishedUnix      int64             `json:"published_unix"`
	ExpiresUnix        int64             `json:"expires_unix"`
	Metadata           map[string]string `json:"metadata,omitempty"`
}
type Descriptor struct {
	DescriptorBody
	Signature string `json:"signature"`
}

func New(id *serviceidentity.Identity, network networkid.ID, intros []nodeid.ID, revision uint64, ttl time.Duration, metadata map[string]string) (Descriptor, error) {
	s := make([]string, 0, len(intros))
	for _, x := range intros {
		s = append(s, x.String())
	}
	sort.Strings(s)
	now := time.Now().UTC()
	body := DescriptorBody{Version: DescriptorVersion, NetworkID: network.String(), ServiceID: id.ID.String(), PublicKey: base64.StdEncoding.EncodeToString(id.PublicKey), IntroductionPoints: s, Revision: revision, PublishedUnix: now.Unix(), ExpiresUnix: now.Add(ttl).Unix(), Metadata: metadata}
	raw, err := json.Marshal(body)
	if err != nil {
		return Descriptor{}, err
	}
	return Descriptor{DescriptorBody: body, Signature: base64.StdEncoding.EncodeToString(id.Sign(raw))}, nil
}
func (d Descriptor) Verify(now time.Time, network networkid.ID) (serviceid.ID, error) {
	if d.Version != DescriptorVersion {
		return serviceid.ID{}, fmt.Errorf("unsupported descriptor version %d", d.Version)
	}
	if d.NetworkID != network.String() {
		return serviceid.ID{}, errors.New("descriptor network mismatch")
	}
	id, err := serviceid.Parse(d.ServiceID)
	if err != nil {
		return serviceid.ID{}, err
	}
	pub, err := base64.StdEncoding.DecodeString(d.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return serviceid.ID{}, errors.New("invalid descriptor public key")
	}
	if serviceid.FromPublicKey(pub) != id {
		return serviceid.ID{}, errors.New("service id does not match public key")
	}
	if d.ExpiresUnix <= now.Unix() {
		return serviceid.ID{}, errors.New("descriptor expired")
	}
	if d.PublishedUnix > now.Add(5*time.Minute).Unix() {
		return serviceid.ID{}, errors.New("descriptor is from the future")
	}
	if len(d.IntroductionPoints) == 0 || len(d.IntroductionPoints) > 8 {
		return serviceid.ID{}, errors.New("invalid introduction point count")
	}
	for _, s := range d.IntroductionPoints {
		if _, err := nodeid.Parse(s); err != nil {
			return serviceid.ID{}, errors.New("invalid introduction point")
		}
	}
	sig, err := base64.StdEncoding.DecodeString(d.Signature)
	if err != nil {
		return serviceid.ID{}, errors.New("invalid descriptor signature")
	}
	raw, _ := json.Marshal(d.DescriptorBody)
	if !ed25519.Verify(ed25519.PublicKey(pub), raw, sig) {
		return serviceid.ID{}, errors.New("invalid descriptor signature")
	}
	return id, nil
}
