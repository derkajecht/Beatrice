package storage

import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite"
)

// NewDatabase initializes a new database connection
// and checks if the connection is successful
// if not, it will log the error and exit the program
func NewDatabase(dbName string) (*sql.DB, error) {

	db, err := sql.Open("sqlite", dbName)
	if err != nil {
		log.Fatal(err)
	}

	if err := db.Ping(); err != nil {
		log.Fatal(err)
	}

	log.Println("Database initialized successfully")

	return db, nil
}
