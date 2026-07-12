//go:build !fts5

package main

// mattn/go-sqlite3 (the only SQLite driver since the modernc removal)
// compiles FTS5 support only under the `fts5` build tag. Without it this
// binary builds cleanly and then breaks at runtime — note-saving fails
// when the FTS index is updated. Fail at compile time instead:
//
//	CGO_ENABLED=1 go build --tags="fts5,cgo"
var _ = thisBinaryRequiresBuildTagFts5
