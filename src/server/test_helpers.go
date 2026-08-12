package server

import "testing"

// Sets up db for testing
func TestDB(t *testing.T, dbName, dbLoc string) *DatabaseInfo {
	dbInfo, err := NewDatabase(dbName, dbLoc)
	if err != nil {
		t.Fatalf("NewDatabase failed: %v", err)
	}
	return dbInfo
}
