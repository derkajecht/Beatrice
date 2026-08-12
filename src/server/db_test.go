package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewDatabaseCreatesFileNotDir(t *testing.T) {
	location := t.TempDir()
	dbInfo := TestDB(t, "beatrice.db", location)
	defer dbInfo.DB.Close()

	p := filepath.Join(location, "beatrice.db")
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("database file not found at %s: %v", p, err)
	}
	if info.IsDir() {
		t.Fatalf("expected file, got directory at %s", p)
	}
}

func TestNewDatabaseTildeExpansion(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	dbInfo := TestDB(t, "x.db", "~/sub")
	defer dbInfo.DB.Close()

	p := filepath.Join(home, "sub", "x.db")
	info, err := os.Stat(p)
	if err != nil {
		t.Fatalf("database file not found at %s: %v", p, err)
	}
	if info.IsDir() {
		t.Fatalf("expected file, got directory at %s", p)
	}
}

func TestStoreUserRoundTrip(t *testing.T) {
	dbInfo := TestDB(t, "beatrice.db", t.TempDir())
	defer dbInfo.DB.Close()

	// First store should succeed
	if err := dbInfo.StoreUser("alice", []byte("pubkey")); err != nil {
		t.Fatalf("first StoreUser failed: %v", err)
	}

	// Duplicate nickname must fail with UNIQUE constraint
	err := dbInfo.StoreUser("alice", []byte("other"))
	if err == nil {
		t.Fatal("expected error on duplicate nickname, got nil")
	}
	if !strings.Contains(err.Error(), "store user") {
		t.Fatalf("expected error to contain 'store user', got: %v", err)
	}
}
