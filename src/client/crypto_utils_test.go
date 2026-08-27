package client

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/derkajecht/Beatrice/src/shared"
)

// newTestUser builds a User with a fresh ephemeral HPKE key pair.
func newTestUser(t *testing.T, nickname string) *User {
	t.Helper()
	cp, err := NewUserSession(true)
	if err != nil {
		t.Fatalf("NewUserSession(%s) failed: %v", nickname, err)
	}
	u := NewUser()
	u.Nickname = nickname
	u.Crypto = *cp
	return u
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	alice := newTestUser(t, "alice")
	bob := newTestUser(t, "bob")

	alice.ConnectedUsers["bob"] = bob.Crypto.PubKey

	targets, err := EncryptMessage(alice, []string{"bob"}, "hello bob")
	if err != nil {
		t.Fatalf("EncryptMessage failed: %v", err)
	}
	if len(targets) != 1 {
		t.Fatalf("expected 1 encrypted target, got %d", len(targets))
	}
	if targets[0].Nickname != "bob" || len(targets[0].Enc) == 0 || len(targets[0].CT) == 0 {
		t.Fatalf("unexpected target record: %+v", targets[0])
	}

	got, err := DecryptMessage(bob, targets[0].Enc, targets[0].CT, "alice", "bob")
	if err != nil {
		t.Fatalf("DecryptMessage failed: %v", err)
	}
	if got != "hello bob" {
		t.Fatalf("expected %q, got %q", "hello bob", got)
	}
}

// TestDirectoryFromHandshakeRoundTrip proves that a directory populated the
// way the real handshake flow populates it (HandshakePacket.HPKEPubKey ->
// ServerClient -> DirPacket -> client ConnectedUsers) yields working
// encrypt/decrypt between peers.
func TestDirectoryFromHandshakeRoundTrip(t *testing.T) {
	alice := newTestUser(t, "alice")
	bob := newTestUser(t, "bob")

	// Client side of the handshake: HPKE key travels in HandshakePacket.
	hsBytes, err := json.Marshal(shared.HandshakePacket{
		Nickname:   "bob",
		PubKey:     []byte("identity-key"),
		HPKEPubKey: bob.Crypto.PubKey,
	})
	if err != nil {
		t.Fatalf("marshal HandshakePacket failed: %v", err)
	}
	var hs shared.HandshakePacket
	if err := json.Unmarshal(hsBytes, &hs); err != nil {
		t.Fatalf("unmarshal HandshakePacket failed: %v", err)
	}

	// Server side (as completeHandshake does): HPKE key lands in the
	// directory snapshot sent back as a DirPacket.
	dir := shared.DirPacket{CurrentUsers: map[string][]byte{"bob": hs.HPKEPubKey}}

	// Client applies the authoritative directory exactly as readLoop does.
	alice.ApplyDirPacket(dir.CurrentUsers)

	targets, err := EncryptMessage(alice, []string{"bob"}, "via directory")
	if err != nil {
		t.Fatalf("EncryptMessage failed: %v", err)
	}
	got, err := DecryptMessage(bob, targets[0].Enc, targets[0].CT, "alice", "bob")
	if err != nil {
		t.Fatalf("DecryptMessage failed: %v", err)
	}
	if got != "via directory" {
		t.Fatalf("expected %q, got %q", "via directory", got)
	}
}

// TestDecryptRejectsAADTampering verifies that routing metadata (sender or
// recipient) is bound into the ciphertext: any mismatch must fail to open.
func TestDecryptRejectsAADTampering(t *testing.T) {
	alice := newTestUser(t, "alice")
	bob := newTestUser(t, "bob")
	alice.ConnectedUsers["bob"] = bob.Crypto.PubKey

	targets, err := EncryptMessage(alice, []string{"bob"}, "sealed metadata")
	if err != nil {
		t.Fatalf("EncryptMessage failed: %v", err)
	}
	tgt := targets[0]

	cases := []struct {
		name, sender, recipient string
	}{
		{"wrong sender", "mallory", "bob"},
		{"wrong recipient", "alice", "carol"},
	}
	for _, tc := range cases {
		if _, err := DecryptMessage(bob, tgt.Enc, tgt.CT, tc.sender, tc.recipient); err == nil {
			t.Fatalf("%s: expected decryption failure on AAD mismatch, got nil", tc.name)
		}
	}
}

func TestEncryptMessageSkipsSelfAndUnknownFails(t *testing.T) {
	alice := newTestUser(t, "alice")

	// Unknown target: no results and an error (never silently ignored).
	targets, err := EncryptMessage(alice, []string{"ghost"}, "hi")
	if err == nil {
		t.Fatal("expected error for unknown target, got nil")
	}
	if len(targets) != 0 {
		t.Fatalf("expected no targets for unknown recipient, got %d", len(targets))
	}

	// Encrypting only to self yields nothing without an error.
	targets, err = EncryptMessage(alice, []string{"alice"}, "note to self")
	if err != nil {
		t.Fatalf("self-only encryption should not error: %v", err)
	}
	if len(targets) != 0 {
		t.Fatalf("expected no targets when encrypting to self, got %d", len(targets))
	}
}

func TestDecryptMessageRejectsGarbage(t *testing.T) {
	bob := newTestUser(t, "bob")

	if _, err := DecryptMessage(bob, nil, []byte("ct"), "alice", "bob"); err == nil {
		t.Fatal("expected error for missing enc, got nil")
	}
	// NOTE: double check this implementation, I don't think it's doing what it should be doing. on paper, this looks like it would return true, but []byte("garbage") can be deciphered.
	if _, err := DecryptMessage(bob, []byte("enc"), []byte("garbage"), "alice", "bob"); err == nil {
		t.Fatal("expected error for undecryptable ciphertext, got nil")
	}
}

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
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("expected identity key file mode 0600, got %o", perm)
	}
	if filepath.Dir(path) != filepath.Join(home, ".beatrice") {
		t.Fatalf("unexpected identity key location: %s", path)
	}
}
