package naming

import (
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/localzet/knotroute/internal/nodeid"
	"github.com/localzet/knotroute/internal/serviceid"
)

const (
	Suffix                     = ".knot"
	NodeAddressVersion    byte = 1
	ServiceAddressVersion byte = 2
	AddressVersion        byte = NodeAddressVersion
	DefaultService             = "http"
)

var (
	b32         = base32.StdEncoding.WithPadding(base32.NoPadding)
	aliasName   = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)
	serviceName = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}$`)
)

type AddressKind byte

const (
	AddressNode    AddressKind = AddressKind(NodeAddressVersion)
	AddressService AddressKind = AddressKind(ServiceAddressVersion)
)

func encodeLabel(kind byte, raw []byte) string {
	payload := make([]byte, 1+len(raw)+2)
	payload[0] = kind
	copy(payload[1:], raw)
	sum := checksum(payload[:1+len(raw)])
	copy(payload[1+len(raw):], sum[:2])
	return strings.ToLower(b32.EncodeToString(payload))
}
func CanonicalLabel(id nodeid.ID) string            { return encodeLabel(NodeAddressVersion, id[:]) }
func CanonicalDomain(id nodeid.ID) string           { return CanonicalLabel(id) + Suffix }
func ServiceCanonicalLabel(id serviceid.ID) string  { return encodeLabel(ServiceAddressVersion, id[:]) }
func ServiceCanonicalDomain(id serviceid.ID) string { return ServiceCanonicalLabel(id) + Suffix }
func ServiceDomain(service string, id nodeid.ID) (string, error) {
	service = normalizeService(service)
	if !serviceName.MatchString(service) {
		return "", fmt.Errorf("invalid service name %q", service)
	}
	return service + "." + CanonicalDomain(id), nil
}

type ParsedLabel struct {
	Kind    AddressKind
	Node    nodeid.ID
	Service serviceid.ID
}

func ParseLabel(label string) (ParsedLabel, error) {
	label = strings.TrimSpace(strings.ToLower(label))
	raw, err := b32.DecodeString(strings.ToUpper(label))
	if err != nil {
		return ParsedLabel{}, fmt.Errorf("decode .knot address: %w", err)
	}
	if len(raw) != 35 {
		return ParsedLabel{}, fmt.Errorf("invalid .knot address length: got %d bytes", len(raw))
	}
	expected := checksum(raw[:33])
	if raw[33] != expected[0] || raw[34] != expected[1] {
		return ParsedLabel{}, errors.New("invalid .knot address checksum")
	}
	switch raw[0] {
	case NodeAddressVersion:
		id, err := nodeid.FromBytes(raw[1:33])
		return ParsedLabel{Kind: AddressNode, Node: id}, err
	case ServiceAddressVersion:
		id, err := serviceid.FromBytes(raw[1:33])
		return ParsedLabel{Kind: AddressService, Service: id}, err
	default:
		return ParsedLabel{}, fmt.Errorf("unsupported .knot address version %d", raw[0])
	}
}
func ParseCanonicalLabel(label string) (nodeid.ID, error) {
	p, err := ParseLabel(label)
	if err != nil {
		return nodeid.ID{}, err
	}
	if p.Kind != AddressNode {
		return nodeid.ID{}, errors.New("address is a service identity, not a node identity")
	}
	return p.Node, nil
}
func ParseServiceCanonicalLabel(label string) (serviceid.ID, error) {
	p, err := ParseLabel(label)
	if err != nil {
		return serviceid.ID{}, err
	}
	if p.Kind != AddressService {
		return serviceid.ID{}, errors.New("address is a node identity, not a service identity")
	}
	return p.Service, nil
}

func checksum(payload []byte) [32]byte {
	data := append([]byte("knotroute-address-checksum-v2"), payload...)
	return sha256.Sum256(data)
}

type Alias struct {
	Name        string `json:"name"`
	Node        string `json:"node,omitempty"`
	ServiceID   string `json:"service_id,omitempty"`
	Description string `json:"description,omitempty"`
}
type Resolved struct {
	Kind      AddressKind
	Node      nodeid.ID
	ServiceID serviceid.ID
	Service   string
	Canonical bool
	Alias     string
	Host      string
}

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
	if parsed, err := ParseLabel(identityLabel); err == nil {
		if parsed.Kind == AddressService {
			if len(labels) != 1 {
				return Resolved{}, errors.New("service-identity .knot addresses do not accept a service prefix")
			}
			return Resolved{Kind: AddressService, ServiceID: parsed.Service, Canonical: true, Host: host}, nil
		}
		service := DefaultService
		if len(labels) > 1 {
			service = normalizeService(strings.Join(labels[:len(labels)-1], "."))
		}
		if !serviceName.MatchString(service) {
			return Resolved{}, fmt.Errorf("invalid service name %q", service)
		}
		return Resolved{Kind: AddressNode, Node: parsed.Node, Service: service, Canonical: true, Host: host}, nil
	}
	if len(labels) != 1 && len(labels) != 2 {
		return Resolved{}, errors.New("invalid alias address")
	}
	alias := labels[len(labels)-1]
	if !aliasName.MatchString(alias) {
		return Resolved{}, fmt.Errorf("invalid alias %q", alias)
	}
	for _, a := range aliases {
		if strings.EqualFold(a.Name, alias) {
			if a.ServiceID != "" {
				if len(labels) != 1 {
					return Resolved{}, errors.New("service alias does not accept a service prefix")
				}
				id, err := ParseServiceReference(a.ServiceID)
				if err != nil {
					return Resolved{}, fmt.Errorf("alias %q: %w", a.Name, err)
				}
				return Resolved{Kind: AddressService, ServiceID: id, Alias: alias, Host: host}, nil
			}
			id, err := ParseNodeReference(a.Node)
			if err != nil {
				return Resolved{}, fmt.Errorf("alias %q: %w", a.Name, err)
			}
			service := DefaultService
			if len(labels) > 1 {
				service = normalizeService(labels[0])
			}
			return Resolved{Kind: AddressNode, Node: id, Service: service, Alias: alias, Host: host}, nil
		}
	}
	return Resolved{}, fmt.Errorf("unknown local alias %q", alias)
}

func ValidateAlias(a Alias) error {
	a.Name = strings.ToLower(strings.TrimSpace(a.Name))
	if !aliasName.MatchString(a.Name) {
		return fmt.Errorf("invalid alias name %q", a.Name)
	}
	if (a.Node == "") == (a.ServiceID == "") {
		return errors.New("alias must set exactly one of node or service_id")
	}
	if a.Node != "" {
		_, err := ParseNodeReference(a.Node)
		return err
	}
	_, err := ParseServiceReference(a.ServiceID)
	return err
}
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
func ParseServiceReference(value string) (serviceid.ID, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if strings.HasSuffix(value, Suffix) {
		labels := strings.Split(strings.TrimSuffix(value, Suffix), ".")
		if len(labels) != 1 {
			return serviceid.ID{}, errors.New("service target must be a bare service address")
		}
		return ParseServiceCanonicalLabel(labels[0])
	}
	return serviceid.Parse(value)
}
func ResolveNodeReference(value string, aliases []Alias) (nodeid.ID, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if id, err := ParseNodeReference(value); err == nil {
		return id, nil
	}
	if strings.HasSuffix(value, Suffix) {
		r, err := ResolveHost(value, aliases)
		if err != nil {
			return nodeid.ID{}, err
		}
		if r.Kind != AddressNode {
			return nodeid.ID{}, errors.New("reference resolves to a service, not a node")
		}
		return r.Node, nil
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
