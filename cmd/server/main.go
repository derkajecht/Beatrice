package main

import (
	"flag"

	"github.com/derkajecht/Beatrice/internal/server/websocket"
)

func main() {

	// set flags for the cli args
	host := flag.String("host", "0.0.0.0", "Host of the server - default is 0.0.0.0")
	port := flag.String("port", "8080", "Port of the server - default is 8080")
	dbName := flag.String("db", "beatrice.db", "Name of the database")
	dbLocation := flag.String("dbLocation", "./beatrice", "Location of the database")
	flag.Parse()

	// run the server - starts the websocket server, starts the hub and listens for incoming connections
	websocket.StartServer(*host, *port, *dbName, *dbLocation)

	// example usage:
	// go run main.go -host localhost -port 8080 -contype tcp -db beatrice.db -dbloc ./beatrice
}
