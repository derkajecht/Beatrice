package main

import (
	"flag"
	"log"

	"github.com/derkajecht/Beatrice/src/client"
)

func main() {
	// Command line args
	host := flag.String("host", "localhost", "Host of the server")
	port := flag.String("port", "8080", "Port of the server")
	nick := flag.String("nick", "anon", "Nickname to use")
	ephemeral := flag.Bool("ephemeral", false, "Create key just for this session.")
	configPath := flag.String("config", "", "Path to config file (default: $BEATRICE_CONFIG or the user config directory)")
	caPath := flag.String("ca", "", "PEM file of the CA or self-signed server certificate")
	flag.Parse()

	// Load configuration before starting the TUI so a bad config fails fast.
	cfg, err := client.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// start the client with the provided args
	if err := client.StartClient(*host, *port, *nick, *caPath, *ephemeral, cfg); err != nil {
		log.Fatal(err)
	}
}
