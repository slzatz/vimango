//go:build cgo && !windows

package main

import (
	// Import CGO sqlite driver (only on non-Windows platforms with CGO)
	_ "github.com/mattn/go-sqlite3"
)

// cgoSQLiteAvailable returns true when CGO SQLite driver is available
func cgoSQLiteAvailable() bool {
	return true
}