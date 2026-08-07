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
