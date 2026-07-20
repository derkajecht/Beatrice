package config

type ConnectionInfo struct {
	Host    string
	Port    string
	ConType string
}

type DatabaseInfo struct {
	Name     string
	Location string
}

// NewConnectionInfo returns a new ConnectionInfo struct with default values
// for the host, port, and connection type ("localhost", "8080", "tcp")
// these values can be overridden by the user inputted values through the cli
func NewConnectionInfo() ConnectionInfo {
	return ConnectionInfo{
		Host:    "localhost",
		Port:    "8080",
		ConType: "tcp",
	}
}

// NewDatabaseInfo returns a new DatabaseInfo struct with default values
// for the database name and location
// these values can be overridden by the user inputted values through the cli
func NewDatabaseInfo() DatabaseInfo {
	return DatabaseInfo{
		Name:     "beatrice.db",
		Location: "./beatrice",
	}
}
