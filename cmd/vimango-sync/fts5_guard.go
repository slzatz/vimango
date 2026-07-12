//go:build !fts5

package main

// vimango-sync updates the FTS database, and mattn/go-sqlite3 compiles
// FTS5 support only under the `fts5` build tag. Without it the binary
// builds cleanly and sync breaks at runtime. Fail at compile time instead:
//
//	CGO_ENABLED=1 go build --tags fts5 ./cmd/vimango-sync
var _ = thisBinaryRequiresBuildTagFts5
