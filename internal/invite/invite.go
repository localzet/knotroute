package invite

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/localzet/knotroute/internal/config"
	"github.com/localzet/knotroute/internal/identity"
	"github.com/localzet/knotroute/internal/networkid"
	"github.com/localzet/knotroute/internal/nodeid"
)

const Version = 1

type Body struct {
	Version    int           `json:"version"`
	NetworkID  string        `json:"network_id"`
	Beacons    []string      `json:"beacons,omitempty"`
	Peers      []config.Peer `json:"peers,omitempty"`
	SignerNode string        `json:"signer_node"`
	PublicKey  string        `json:"public_key"`
	IssuedUnix int64         `json:"issued_unix"`
}

type Invite struct {
	Body
	Signature string `json:"signature"`
}

func New(id *identity.Identity, network networkid.ID, beacons []string, peers []config.Peer, now time.Time) (Invite, error) {
	beacons = append([]string(nil), beacons...)
	sort.Strings(beacons)
	peers = append([]config.Peer(nil), peers...)
	sort.Slice(peers, func(i, j int) bool { return peers[i].Address < peers[j].Address })
	body := Body{Version: Version, NetworkID: network.String(), Beacons: beacons, Peers: peers, SignerNode: id.ID.String(), PublicKey: base64.StdEncoding.EncodeToString(id.PublicKey), IssuedUnix: now.UTC().Unix()}
	raw, err := json.Marshal(body)
	if err != nil {
		return Invite{}, err
	}
	return Invite{Body: body, Signature: base64.StdEncoding.EncodeToString(id.Sign(raw))}, nil
}

func (i Invite) Verify(now time.Time) (networkid.ID, error) {
	if i.Version != Version {
		return networkid.ID{}, errors.New("unsupported invite version")
	}
	network, err := networkid.Parse(i.NetworkID)
	if err != nil {
		return networkid.ID{}, err
	}
	nid, err := nodeid.Parse(i.SignerNode)
	if err != nil {
		return networkid.ID{}, err
	}
	pub, err := base64.StdEncoding.DecodeString(i.PublicKey)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return networkid.ID{}, errors.New("invalid invite public key")
	}
	if nodeid.FromPublicKey(pub) != nid {
		return networkid.ID{}, errors.New("invite signer identity mismatch")
	}
	if i.IssuedUnix > now.Add(5*time.Minute).Unix() {
		return networkid.ID{}, errors.New("invite timestamp is in the future")
	}
	sig, err := base64.StdEncoding.DecodeString(i.Signature)
	if err != nil {
		return networkid.ID{}, errors.New("invalid invite signature encoding")
	}
	raw, err := json.Marshal(i.Body)
	if err != nil {
		return networkid.ID{}, err
	}
	if !ed25519.Verify(ed25519.PublicKey(pub), raw, sig) {
		return networkid.ID{}, errors.New("invalid invite signature")
	}
	return network, nil
}
