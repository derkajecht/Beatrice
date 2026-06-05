package main

import (
	"flag"
	"log"

	"github.com/derkajecht/Beatrice/internal/api"
	"github.com/derkajecht/Beatrice/internal/storage"
)

// main function to start the server.
// It takes in the host and port as arguments and creates a new server instance.
// It also takes in the database name as an argument and creates a new database instance.
func RunServer(host string, port string, dbName string) {

	// create a new server instance and pass the database connection
	err := api.NewServer(host, port)
	if err != nil {
		log.Fatalf("Could not set up server: %v\n", err)
	}
	// log that the server was started
	log.Println("Server started")

	// create a new database instance and pass the database connection
	// NewDatabase checks if the database is accessible and returns an error if not
	// if the database is accessible, it returns a pointer to the database connection
	db, err, _ := storage.NewDatabase(dbName)
	if err != nil {
		log.Fatalf("Could not set up database: %v\n", err)
	}
	// log that the database connection was established
	log.Println("Database connection established")

	// Defer the closing of the database connection to ensure that it is properly closed
	defer db.Close()
}

func main() {

	host := flag.String("host", "localhost", "Host of the server")
	port := flag.String("port", "8080", "Port of the server")
	dbName := flag.String("db", "beatrice.db", "Name of the database")

	flag.Parse()

	RunServer(*host, *port, *dbName)

	// example usage:
	// go run main.go -host localhost -port 8080 -db beatrice.db
}
