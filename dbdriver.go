package main

import (
	"database/sql"
	"fmt"
	"runtime"

	// Import postgres driver (available on all platforms)
	_ "github.com/lib/pq"
	// Import pure Go sqlite driver (available on all platforms)
	_ "modernc.org/sqlite"
)

// SQLiteDriver represents the available SQLite driver options
type SQLiteDriver int

const (
	SQLiteDriverModernC SQLiteDriver = iota // Pure Go implementation (modernc.org/sqlite)
	SQLiteDriverMattn                       // CGO implementation (mattn/go-sqlite3)
)

// SQLiteConfig holds the configuration for SQLite driver selection
type SQLiteConfig struct {
	Driver SQLiteDriver
}

// GetSQLiteDriverName returns the driver name for sql.Open based on the selected driver
func (cfg *SQLiteConfig) GetSQLiteDriverName() string {
	switch cfg.Driver {
	case SQLiteDriverMattn:
		return "sqlite3"
	case SQLiteDriverModernC:
		return "sqlite"
	default:
		return "sqlite" // Default to pure Go implementation
	}
}

// GetSQLiteDriverDisplayName returns a human-readable name for the driver
func (cfg *SQLiteConfig) GetSQLiteDriverDisplayName() string {
	switch cfg.Driver {
	case SQLiteDriverMattn:
		return "mattn/go-sqlite3 (CGO)"
	case SQLiteDriverModernC:
		return "modernc.org/sqlite (Pure Go)"
	default:
		return "modernc.org/sqlite (Pure Go)"
	}
}

// OpenSQLiteDB opens a SQLite database using the configured driver
func (cfg *SQLiteConfig) OpenSQLiteDB(dataSourceName string) (*sql.DB, error) {
	driverName := cfg.GetSQLiteDriverName()
	return sql.Open(driverName, dataSourceName)
}

// IsCGOSQLiteAvailable checks if the CGO SQLite driver is available
func IsCGOSQLiteAvailable() bool {
	// On Windows, CGO SQLite is never available
	if runtime.GOOS == "windows" {
		return false
	}
	
	// On other platforms, it depends on build tags
	// This will be true only if the cgo build tag is set and we're not on Windows
	return cgoSQLiteAvailable()
}

// DetermineSQLiteDriver determines which SQLite driver to use based on:
// 1. Command line arguments
// 2. Platform constraints (Windows only supports pure Go)
// 3. Build constraints (CGO availability)
func DetermineSQLiteDriver(args []string) *SQLiteConfig {
	cfg := &SQLiteConfig{
		Driver: SQLiteDriverModernC, // Default to pure Go
	}

	// On Windows, force pure Go implementation
	if runtime.GOOS == "windows" {
		cfg.Driver = SQLiteDriverModernC
		return cfg
	}

	// Check for --go-sqlite flag (forces pure Go)
	// Check for --cgo-sqlite flag (forces CGO) - only if CGO is available
	for _, arg := range args {
		if arg == "--go-sqlite" {
			cfg.Driver = SQLiteDriverModernC
			return cfg
		}
		if arg == "--cgo-sqlite" && IsCGOSQLiteAvailable() {
			cfg.Driver = SQLiteDriverMattn
			return cfg
		}
	}

	// Default behavior: use pure Go implementation
	return cfg
}

// LogSQLiteDriverChoice logs which SQLite driver is being used
func LogSQLiteDriverChoice(cfg *SQLiteConfig) {
	fmt.Printf("Using SQLite driver: %s\n", cfg.GetSQLiteDriverDisplayName())
}