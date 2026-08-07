package identity

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateSaveLoadAndSign(t *testing.T) {
	id, err := Generate()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "identity.json")
	if err := id.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != id.ID {
		t.Fatalf("id mismatch: %s != %s", loaded.ID, id.ID)
	}
	msg := []byte("knotroute")
	sig := loaded.Sign(msg)
	if !Verify(loaded.PublicKey, msg, sig) {
		t.Fatal("signature did not verify")
	}
	if info, err := os.Stat(path); err == nil && info.Mode().Perm()&0o077 != 0 {
		t.Fatalf("identity permissions are too broad: %o", info.Mode().Perm())
	}
}
