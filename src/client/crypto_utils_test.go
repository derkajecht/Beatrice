package client

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreateIdentityKeyRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	pub1, priv1, err := loadOrCreateIdentityKey()
	if err != nil {
		t.Fatalf("first load failed: %v", err)
	}

	// Second call must load the same persisted key, not generate a new one.
	pub2, priv2, err := loadOrCreateIdentityKey()
	if err != nil {
		t.Fatalf("second load failed: %v", err)
	}

	if !bytes.Equal(priv1, priv2) {
		t.Fatal("identity private key changed between sessions")
	}
	if !bytes.Equal(pub1, pub2) {
		t.Fatal("identity public key changed between sessions")
	}

	// Key file must exist with owner-only permissions.
	path, err := identityKeyPath()
	if err != nil {
		t.Fatalf("identityKeyPath failed: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("identity key file not found at %s: %v", path, err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("expected identity key file mode 0600, got %o", perm)
	}
	if filepath.Dir(path) != filepath.Join(home, ".beatrice") {
		t.Fatalf("unexpected identity key location: %s", path)
	}
}
