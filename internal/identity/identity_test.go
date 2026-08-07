package identity

import (
	"os"
	"path/filepath"
	"runtime"
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
	// Windows does not expose POSIX permission bits through os.FileMode.
	// Files created with os.WriteFile(..., 0o600) commonly stat as 0666 there;
	// access control is provided by the inherited NTFS ACL instead.
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0o077 != 0 {
			t.Fatalf("identity permissions are too broad: %o", info.Mode().Perm())
		}
	}
}
