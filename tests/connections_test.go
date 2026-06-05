package tests_test

import (
	"testing"

	"github.com/derkajecht/Beatrice/internal/api"
)

func TestNewServer_EmptyPort(t *testing.T) {
	err := api.NewServer("", "localhost")
	expected := "Cannot start server: port cannot be empty"

	if err == nil {
		t.Errorf("Expected error %s, got nil", expected)
	}

	if err.Error() != expected {
		t.Errorf("Expected error %s, got %s", expected, err.Error())
	}
}
