//go:build !cgo || windows

package main

// cgoSQLiteAvailable returns false when CGO SQLite driver is not available
func cgoSQLiteAvailable() bool {
	return false
}