package config

import "fmt"

// DBBackend determines which supported database backend Podman should use.
type DBBackend int

const (
	// Unsupported database backend.  Used as a sane base value for the type.
	DBBackendUnsupported DBBackend = iota
	// BoltDB backend.
	DBBackendBoltDB
	// SQLite backend.
	DBBackendSQLite

	stringBoltDB = "boltdb"
	stringSQLite = "sqlite"
)

// String returns the DBBackend's string representation.
func (d DBBackend) String() string {
	switch d {
	case DBBackendBoltDB:
		return stringBoltDB
	case DBBackendSQLite:
		return stringSQLite
	default:
		return fmt.Sprintf("unsupported database backend: %d", d)
	}
}

// Validate returns whether the DBBackend is supported.
func (d DBBackend) Validate() error {
	switch d {
	case DBBackendBoltDB, DBBackendSQLite:
		return nil
	default:
		return fmt.Errorf("unsupported database backend: %d", d)
	}
}

// ParseDBBackend parses the specified string into a DBBackend.
// An error is return for unsupported backends.
func ParseDBBackend(raw string) (DBBackend, error) {
	// NOTE: this function should be used for parsing the user-specified
	// values on Podman's CLI.
	switch raw {
	case stringBoltDB:
		return DBBackendBoltDB, nil
	case stringSQLite:
		return DBBackendSQLite, nil
	default:
		return DBBackendUnsupported, fmt.Errorf("unsupported database backend: %q", raw)
	}
}

// DBBackend returns the configured database backend.
func (c *Config) DBBackend() (DBBackend, error) {
	return ParseDBBackend(c.Engine.DBBackend)
}
