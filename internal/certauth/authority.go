package certauth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Subject struct {
	CommonName         string
	Organization       []string
	OrganizationalUnit []string
	Country            []string
	Province           []string
	Locality           []string
	StreetAddress      []string
	PostalCode         []string
}

type Profile struct {
	Subject      Subject
	ValidityDays int
}

type Info struct {
	Subject     string
	Issuer      string
	CommonName  string
	Fingerprint string
	Serial      string
	NotBefore   time.Time
	NotAfter    time.Time
	RootPath    string
}

type Authority struct {
	cert    *x509.Certificate
	key     *ecdsa.PrivateKey
	certPEM []byte
	dir     string
	mu      sync.Mutex
	cache   map[string]tls.Certificate
}

func DefaultProfile() Profile {
	return Profile{Subject: Subject{CommonName: "KnotRoute Local Root CA", Organization: []string{"KnotRoute"}}, ValidityDays: 3650}
}

func normalizeProfile(profile Profile) (Profile, error) {
	if strings.TrimSpace(profile.Subject.CommonName) == "" {
		profile.Subject.CommonName = "KnotRoute Local Root CA"
	}
	if len(profile.Subject.Organization) == 0 {
		profile.Subject.Organization = []string{"KnotRoute"}
	}
	if profile.ValidityDays == 0 {
		profile.ValidityDays = 3650
	}
	if profile.ValidityDays < 30 || profile.ValidityDays > 7300 {
		return Profile{}, errors.New("CA validity must be between 30 and 7300 days")
	}
	return profile, nil
}

func LoadOrCreate(dir string) (*Authority, error) {
	return LoadOrCreateWithProfile(dir, DefaultProfile())
}

func LoadOrCreateWithProfile(dir string, profile Profile) (*Authority, error) {
	if dir == "" {
		return nil, errors.New("CA directory is empty")
	}
	profile, err := normalizeProfile(profile)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	certPath := filepath.Join(dir, "root-ca.pem")
	keyPath := filepath.Join(dir, "root-ca-key.pem")
	if certRaw, err := os.ReadFile(certPath); err == nil {
		keyRaw, err := os.ReadFile(keyPath)
		if err != nil {
			return nil, err
		}
		return load(dir, certRaw, keyRaw)
	}
	return generate(dir, certPath, keyPath, profile)
}

// Regenerate replaces the local CA keypair. Existing certificates issued by
// the previous CA immediately stop validating, so callers should reinstall the
// new root on every client after this operation.
func Regenerate(dir string, profile Profile) (*Authority, error) {
	if dir == "" {
		return nil, errors.New("CA directory is empty")
	}
	profile, err := normalizeProfile(profile)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	return generate(dir, filepath.Join(dir, "root-ca.pem"), filepath.Join(dir, "root-ca-key.pem"), profile)
}

func generate(dir, certPath, keyPath string, profile Profile) (*Authority, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 159))
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:         profile.Subject.CommonName,
			Organization:       cloneStrings(profile.Subject.Organization),
			OrganizationalUnit: cloneStrings(profile.Subject.OrganizationalUnit),
			Country:            cloneStrings(profile.Subject.Country),
			Province:           cloneStrings(profile.Subject.Province),
			Locality:           cloneStrings(profile.Subject.Locality),
			StreetAddress:      cloneStrings(profile.Subject.StreetAddress),
			PostalCode:         cloneStrings(profile.Subject.PostalCode),
		},
		NotBefore:             now.Add(-time.Hour),
		NotAfter:              now.AddDate(0, 0, profile.ValidityDays),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		MaxPathLen:            0,
		MaxPathLenZero:        true,
		SignatureAlgorithm:    x509.ECDSAWithSHA256,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if err := writePairAtomic(certPath, keyPath, certPEM, keyPEM); err != nil {
		return nil, err
	}
	return load(dir, certPEM, keyPEM)
}

func writePairAtomic(certPath, keyPath string, certPEM, keyPEM []byte) error {
	dir := filepath.Dir(certPath)
	certTmp, err := os.CreateTemp(dir, ".root-ca-*.pem")
	if err != nil {
		return err
	}
	certTmpPath := certTmp.Name()
	defer os.Remove(certTmpPath)
	keyTmp, err := os.CreateTemp(dir, ".root-ca-key-*.pem")
	if err != nil {
		_ = certTmp.Close()
		return err
	}
	keyTmpPath := keyTmp.Name()
	defer os.Remove(keyTmpPath)
	if err := certTmp.Chmod(0o644); err != nil {
		return err
	}
	if err := keyTmp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := certTmp.Write(certPEM); err != nil {
		return err
	}
	if _, err := keyTmp.Write(keyPEM); err != nil {
		return err
	}
	if err := certTmp.Sync(); err != nil {
		return err
	}
	if err := keyTmp.Sync(); err != nil {
		return err
	}
	if err := certTmp.Close(); err != nil {
		return err
	}
	if err := keyTmp.Close(); err != nil {
		return err
	}
	// Replace the key first and certificate second. Load validates that both
	// belong to the same pair, so an interrupted rotation fails closed.
	if err := os.Rename(keyTmpPath, keyPath); err != nil {
		return err
	}
	if err := os.Rename(certTmpPath, certPath); err != nil {
		return err
	}
	return nil
}

func load(dir string, certPEM, keyPEM []byte) (*Authority, error) {
	cb, _ := pem.Decode(certPEM)
	if cb == nil {
		return nil, errors.New("invalid CA certificate PEM")
	}
	cert, err := x509.ParseCertificate(cb.Bytes)
	if err != nil {
		return nil, err
	}
	if !cert.IsCA || !cert.BasicConstraintsValid {
		return nil, errors.New("root certificate is not a CA")
	}
	kb, _ := pem.Decode(keyPEM)
	if kb == nil {
		return nil, errors.New("invalid CA key PEM")
	}
	raw, err := x509.ParsePKCS8PrivateKey(kb.Bytes)
	if err != nil {
		return nil, err
	}
	key, ok := raw.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("CA key is not ECDSA")
	}
	certKey, ok := cert.PublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, errors.New("CA certificate public key is not ECDSA")
	}
	if key.PublicKey.X.Cmp(certKey.X) != 0 || key.PublicKey.Y.Cmp(certKey.Y) != 0 {
		return nil, errors.New("CA certificate does not match the private key")
	}
	return &Authority{cert: cert, key: key, certPEM: append([]byte(nil), certPEM...), dir: dir, cache: map[string]tls.Certificate{}}, nil
}

func (a *Authority) RootPath() string { return filepath.Join(a.dir, "root-ca.pem") }
func (a *Authority) RootPEM() []byte  { return append([]byte(nil), a.certPEM...) }
func (a *Authority) Fingerprint() string {
	sum := sha256.Sum256(a.cert.Raw)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}
func (a *Authority) Info() Info {
	return Info{
		Subject: a.cert.Subject.String(), Issuer: a.cert.Issuer.String(), CommonName: a.cert.Subject.CommonName,
		Fingerprint: a.Fingerprint(), Serial: strings.ToUpper(a.cert.SerialNumber.Text(16)),
		NotBefore: a.cert.NotBefore, NotAfter: a.cert.NotAfter, RootPath: a.RootPath(),
	}
}
func (a *Authority) Certificate(host string) (tls.Certificate, error) {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if !strings.HasSuffix(host, ".knot") {
		return tls.Certificate{}, errors.New("refusing certificate for non-.knot name")
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if c, ok := a.cache[host]; ok {
		return c, nil
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 159))
	if err != nil {
		return tls.Certificate{}, err
	}
	now := time.Now().UTC()
	tmpl := &x509.Certificate{
		SerialNumber: serial, Subject: pkix.Name{CommonName: host, Organization: []string{"KnotRoute Local Service"}},
		DNSNames: []string{host}, NotBefore: now.Add(-5 * time.Minute), NotAfter: now.Add(24 * time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true,
		SignatureAlgorithm: x509.ECDSAWithSHA256,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, a.cert, &key.PublicKey, a.key)
	if err != nil {
		return tls.Certificate{}, err
	}
	c := tls.Certificate{Certificate: [][]byte{der, a.cert.Raw}, PrivateKey: key}
	a.cache[host] = c
	return c, nil
}

func cloneStrings(in []string) []string { return append([]string(nil), in...) }

func ProfileDescription(profile Profile) string {
	profile, _ = normalizeProfile(profile)
	return fmt.Sprintf("CN=%s, O=%s, validity=%d days", profile.Subject.CommonName, strings.Join(profile.Subject.Organization, ","), profile.ValidityDays)
}
