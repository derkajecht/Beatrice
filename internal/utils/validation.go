// Package utils contains utility functions for the project such as validation, listener,
// responses and router
package utils

import (
	"database/sql"
	"errors"
)

// IsEmpty returns true if the given string is empty
func IsEmpty(s string) bool {
	return s == ""
}

// IsValidUsername returns true if the given string is not already taken
// queries the database
func IsValidUsername(db *sql.DB, username string) bool {

	// declare a variable to store the number of rows
	var exists int

	// check if the username exists in the database
	err := db.QueryRow("SELECT id FROM users WHERE username = ?", username).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return true // username is available
		}
		return false // an error occurred
	}

	return false // username is already taken
}
