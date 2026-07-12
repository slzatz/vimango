//go:build !fts5

package main

// This tool creates the FTS5 virtual table, and mattn/go-sqlite3 compiles
// FTS5 support only under the `fts5` build tag. Fail at compile time
// instead of at runtime:
//
//	CGO_ENABLED=1 go build --tags fts5 ./cmd/...
var _ = thisBinaryRequiresBuildTagFts5
