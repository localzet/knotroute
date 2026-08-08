package certauth

import (
	"crypto/x509"
	"testing"
)

func TestIssue(t *testing.T) {
	a, err := LoadOrCreate(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c, err := a.Certificate("example.knot")
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(c.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(a.RootPEM())
	if _, err := leaf.Verify(x509.VerifyOptions{DNSName: "example.knot", Roots: roots}); err != nil {
		t.Fatal(err)
	}
}

func TestCustomRootProfileAndRotation(t *testing.T) {
	dir := t.TempDir()
	profile := Profile{ValidityDays: 825, Subject: Subject{CommonName: "Localzet KnotRoute Root", Organization: []string{"Localzet"}, OrganizationalUnit: []string{"Private Network"}, Country: []string{"RU"}, Province: []string{"Moscow"}, Locality: []string{"Moscow"}, StreetAddress: []string{"Example street"}, PostalCode: []string{"101000"}}}
	a, err := LoadOrCreateWithProfile(dir, profile)
	if err != nil {
		t.Fatal(err)
	}
	info := a.Info()
	if info.CommonName != "Localzet KnotRoute Root" {
		t.Fatalf("CN=%q", info.CommonName)
	}
	if info.Subject != info.Issuer {
		t.Fatalf("self-signed root issuer must equal subject: %q != %q", info.Subject, info.Issuer)
	}
	old := info.Fingerprint
	rotated, err := Regenerate(dir, profile)
	if err != nil {
		t.Fatal(err)
	}
	if rotated.Fingerprint() == old {
		t.Fatal("rotation must create a new root keypair/certificate")
	}
	leaf, err := rotated.Certificate("service.knot")
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(leaf.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if cert.Issuer.String() != rotated.Info().Subject {
		t.Fatalf("leaf issuer=%q root subject=%q", cert.Issuer.String(), rotated.Info().Subject)
	}
}
