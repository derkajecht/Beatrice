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
