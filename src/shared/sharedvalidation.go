// Package validation contains utility functions for the project such as validation, listener,
// responses and router
package shared

import (
	"slices"
)

// HasEmptyArgs returns false if any of the inputs are empty
func HasEmptyArgs(args ...string) bool {
	return slices.Contains(args, "")
}

// ValidPresenceStatus reports whether s is a recognized presence status.
// The server uses this to reject malformed presence updates before fan-out.
func ValidPresenceStatus(s string) bool {
	return s == PresenceActive || s == PresenceAway
}
