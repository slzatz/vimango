//go:build !fts5

package localdb

// The search index is an FTS5 table, and mattn/go-sqlite3 compiles FTS5 in
// only under the `fts5` build tag. Without it this package builds and then
// fails at runtime with "no such module: fts5". Fail at compile time instead:
//
//	CGO_ENABLED=1 go build --tags=fts5
var _ = thisBinaryRequiresBuildTagFts5
