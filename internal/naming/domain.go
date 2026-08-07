package naming

import (
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/localzet/knotroute/internal/nodeid"
)

const (
	Suffix              = ".knot"
	AddressVersion byte = 1
	DefaultService      = "http"
)

var (
	b32         = base32.StdEncoding.WithPadding(base32.NoPadding)
	aliasName   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	serviceName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)
)

// CanonicalLabel encodes a versioned, checksummed node identity into a DNS-safe label.
// The address is self-authenticating because the embedded node ID is a SHA-256 digest
// of the node's Ed25519 public key and is verified during every overlay handshake.
func CanonicalLabel(id nodeid.ID) string {
	payload := make([]byte, 1+len(id)+2)
	payload[0] = AddressVersion
	copy(payload[1:], id[:])
	checksum := checksum(payload[:1+len(id)])
	copy(payload[1+len(id):], checksum[:2])
	return strings.ToLower(b32.EncodeToString(payload))
}

func CanonicalDomain(id nodeid.ID) string { return CanonicalLabel(id) + Suffix }

func ServiceDomain(service string, id nodeid.ID) (string, error) {
	service = normalizeService(service)
	if !serviceName.MatchString(service) {
		return "", fmt.Errorf("invalid service name %q", service)
	}
	return service + "." + CanonicalDomain(id), nil
}

func ParseCanonicalLabel(label string) (nodeid.ID, error) {
	var id nodeid.ID
	label = strings.TrimSpace(strings.ToLower(label))
	raw, err := b32.DecodeString(strings.ToUpper(label))
	if err != nil {
		return id, fmt.Errorf("decode .knot address: %w", err)
	}
	if len(raw) != 35 {
		return id, fmt.Errorf("invalid .knot address length: got %d bytes", len(raw))
	}
	if raw[0] != AddressVersion {
		return id, fmt.Errorf("unsupported .knot address version %d", raw[0])
	}
	copy(id[:], raw[1:33])
	expected := checksum(raw[:33])
	if raw[33] != expected[0] || raw[34] != expected[1] {
		return nodeid.ID{}, errors.New("invalid .knot address checksum")
	}
	return id, nil
}

func checksum(payload []byte) [32]byte {
	const domain = "knotroute-address-checksum-v1"
	data := make([]byte, 0, len(domain)+len(payload))
	data = append(data, domain...)
	data = append(data, payload...)
	return sha256.Sum256(data)
}

type Alias struct {
	Name        string `json:"name"`
	Node        string `json:"node"`
	Description string `json:"description,omitempty"`
}

type Resolved struct {
	Node      nodeid.ID
	Service   string
	Canonical bool
	Alias     string
	Host      string
}

// ResolveHost accepts service.<canonical>.knot, <canonical>.knot,
// service.<alias>.knot, or <alias>.knot. Aliases are local and intentionally
// do not pretend to be globally unique.
func ResolveHost(host string, aliases []Alias) (Resolved, error) {
	host = strings.TrimSpace(strings.ToLower(host))
	host = strings.TrimSuffix(host, ".")
	if strings.Contains(host, ":") {
		return Resolved{}, errors.New("ResolveHost expects a hostname without a port")
	}
	if !strings.HasSuffix(host, Suffix) {
		return Resolved{}, fmt.Errorf("host %q is not a .knot name", host)
	}
	labels := strings.Split(strings.TrimSuffix(host, Suffix), ".")
	if len(labels) == 0 || labels[len(labels)-1] == "" {
		return Resolved{}, errors.New("empty .knot name")
	}
	identityLabel := labels[len(labels)-1]
	service := DefaultService
	if len(labels) > 1 {
		service = normalizeService(strings.Join(labels[:len(labels)-1], "."))
	}
	if !serviceName.MatchString(service) {
		return Resolved{}, fmt.Errorf("invalid service name %q", service)
	}
	if id, err := ParseCanonicalLabel(identityLabel); err == nil {
		return Resolved{Node: id, Service: service, Canonical: true, Host: host}, nil
	}
	if !aliasName.MatchString(identityLabel) {
		return Resolved{}, fmt.Errorf("invalid alias %q", identityLabel)
	}
	for _, a := range aliases {
		if strings.EqualFold(a.Name, identityLabel) {
			id, err := ParseNodeReference(a.Node)
			if err != nil {
				return Resolved{}, fmt.Errorf("alias %q: %w", a.Name, err)
			}
			return Resolved{Node: id, Service: service, Alias: identityLabel, Host: host}, nil
		}
	}
	return Resolved{}, fmt.Errorf("unknown local alias %q", identityLabel)
}

func ValidateAlias(a Alias) error {
	a.Name = strings.ToLower(strings.TrimSpace(a.Name))
	if !aliasName.MatchString(a.Name) {
		return fmt.Errorf("invalid alias name %q", a.Name)
	}
	_, err := ParseNodeReference(a.Node)
	return err
}

// ParseNodeReference accepts an internal kr_ node ID or a bare canonical
// <label>.knot node address.
func ParseNodeReference(value string) (nodeid.ID, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if strings.HasSuffix(value, Suffix) {
		labels := strings.Split(strings.TrimSuffix(value, Suffix), ".")
		if len(labels) != 1 {
			return nodeid.ID{}, errors.New("alias target must be a node address, not a service address")
		}
		return ParseCanonicalLabel(labels[0])
	}
	return nodeid.Parse(value)
}

// ResolveNodeReference additionally accepts a local alias as alias.knot.
func ResolveNodeReference(value string, aliases []Alias) (nodeid.ID, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if id, err := ParseNodeReference(value); err == nil {
		return id, nil
	}
	if strings.HasSuffix(value, Suffix) {
		resolved, err := ResolveHost(value, aliases)
		if err != nil {
			return nodeid.ID{}, err
		}
		return resolved.Node, nil
	}
	return nodeid.ID{}, fmt.Errorf("invalid node reference %q", value)
}

func normalizeService(service string) string {
	service = strings.ToLower(strings.TrimSpace(service))
	if service == "" {
		return DefaultService
	}
	return service
}
