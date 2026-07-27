// Package validation contains utility functions for the project such as validation, listener,
// responses and router
package sharedvalidation

import (
	"slices"
)

// IsEmpty returns true if the given string is empty
func IsEmpty(s string) bool {
	return s == ""
}

// HasEmptyArgs returns false if any of the inputs are empty
func HasEmptyArgs(args ...string) bool {
	return slices.Contains(args, "")
}
