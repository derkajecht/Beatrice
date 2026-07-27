package main

import (
	"flag"
	"log"

	"github.com/derkajecht/Beatrice/internal/api"
	"github.com/derkajecht/Beatrice/internal/server/storage"
	"github.com/derkajecht/Beatrice/internal/shared/types"
)

func ServerStruct(dbLocation string) *types.Server {
	return &types.Server{
		ActiveConnections:  make(map[string]map[string]string),
		PendingConnections: make(map[string]string),
		DatabasePath:       dbLocation,
	}
}

// RunServer starts a new server instance and a new database connection
// It takes the host, port, and database name as arguments
// It returns an error if the host, port, or database name is empty
func RunServer(host, port, conType, dbName, dbLocation string) {

	// create a new server instance and pass the database connection
	go api.NewServer(host, port, conType)

	// log that the server was started
	log.Println("Server started")

	// create a new database instance and pass the database connection
	// NewDatabase checks if the database is accessible and returns an error if not
	// if the database is accessible, it returns a pointer to the database connection
	db, dbLocation, err := storage.NewDatabase(dbName, dbLocation)
	if err != nil {
		log.Fatalf("Could not set up database: %v\n", err)
	}
	// log that the database connection was established
	log.Println("Database connection established")

	// store the database path in the server struct
	ServerStruct(dbLocation)

	// Defer the closing of the database connection to ensure that it is properly closed
	defer db.Close()
}

func main() {

	// set flags for the cli args
	host := flag.String("host", "localhost", "Host of the server")
	port := flag.String("port", "8080", "Port of the server")
	conType := flag.String("conType", "tcp", "Connection type (tcp, udp, etc.)")
	dbName := flag.String("db", "beatrice.db", "Name of the database")
	dbLocation := flag.String("dbLocation", "./beatrice", "Location of the database")
	// dont need to define help flag because it is already defined in the flag package
	// based on the decsriptions of the flags defined above

	flag.Parse()

	RunServer(*host, *port, *conType, *dbName, *dbLocation)

	// example usage:
	// go run main.go -host localhost -port 8080 -contype tcp -db beatrice.db -dbloc ./beatrice
}
