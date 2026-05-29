package server

import (
	"log"

	"github.com/derkajecht/Beatrice/internal/storage"

	"github.com/derkajecht/Beatrice/internal/api"
)

func main(host int, port int) {
	// Call the NewDatabase function to initialize a new database connection
	// and check if the connection is successful
	db, err := storage.NewDatabase("chat_history")
	if err != nil {
		log.Fatalf("Could not set up DB: %v\n", err)
	}
	// Defer the closing of the database connection
	// to ensure that it is properly closed
	defer db.Close()

	server := api.NewServer(db)

	server.Start(host, port)
}
