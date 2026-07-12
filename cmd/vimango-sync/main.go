// vimango-sync is a standalone CLI that runs the vimango synchronization
// engine against a SQLite client database and a Postgres server. It is
// designed to be spawned as a subprocess by the macOS app (~/vimango_macos)
// but can also be run from the shell for diagnostics.
//
// Usage:
//
//	echo '{"host":"...","port":"...","user":"...","password":"...","db":"...","ssl_mode":"require"}' \
//	  | vimango-sync --db /path/to/main.db [--fts-db /path/to/fts.db] [--report-only]
//
// stdout receives the formatted markdown sync log on completion. stderr
// receives a one-line error message on fatal failure. Exit code 0 means
// success, non-zero means fatal error.
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	syncpkg "github.com/slzatz/vimango/internal/sync"
)

type postgresConfig struct {
	Host      string `json:"host"`
	Port      string `json:"port"`
	User      string `json:"user"`
	Password  string `json:"password"`
	DB        string `json:"db"`
	SSLMode   string `json:"ssl_mode"`
	SSLCACert string `json:"ssl_ca_cert"`
}

func main() {
	dbPath := flag.String("db", "", "path to SQLite main database (required)")
	ftsDBPath := flag.String("fts-db", "", "path to SQLite FTS database (defaults to --db)")
	reportOnly := flag.Bool("report-only", false, "only report changes; do not apply them")
	flag.Parse()

	if *dbPath == "" {
		fatal("--db is required")
	}
	if *ftsDBPath == "" {
		*ftsDBPath = *dbPath
	}
	if _, err := os.Stat(*dbPath); err != nil {
		fatal("--db path not accessible: %v", err)
	}

	pg, err := readPostgresConfig(os.Stdin)
	if err != nil {
		fatal("reading postgres config from stdin: %v", err)
	}
	applyEnvOverrides(&pg)
	if pg.Host == "" {
		fatal("postgres config missing 'host'")
	}

	mainDB, err := sql.Open("sqlite3", *dbPath)
	if err != nil {
		fatal("opening main SQLite db: %v", err)
	}
	defer mainDB.Close()
	if _, err := mainDB.Exec("PRAGMA foreign_keys=ON;"); err != nil {
		fatal("enabling foreign keys on main db: %v", err)
	}

	ftsDB, err := sql.Open("sqlite3", *ftsDBPath)
	if err != nil {
		fatal("opening FTS SQLite db: %v", err)
	}
	defer ftsDB.Close()

	pgDB, err := sql.Open("postgres", buildPGConnString(pg))
	if err != nil {
		fatal("opening postgres: %v", err)
	}
	defer pgDB.Close()
	if err := pgDB.Ping(); err != nil {
		fatal("postgres ping failed: %v", err)
	}

	syncer := syncpkg.New(mainDB, ftsDB, pgDB, pg.Host, pg.DB)
	log, syncErr := syncer.Synchronize(*reportOnly)

	// Always emit the log (it contains diagnostic detail even on failure).
	fmt.Print(log)
	if !endsWithNewline(log) {
		fmt.Println()
	}

	if syncErr != nil {
		fmt.Fprintf(os.Stderr, "sync failed: %v\n", syncErr)
		os.Exit(1)
	}
}

func readPostgresConfig(r io.Reader) (postgresConfig, error) {
	var pg postgresConfig
	if err := json.NewDecoder(r).Decode(&pg); err != nil {
		return pg, err
	}
	if pg.SSLMode == "" {
		pg.SSLMode = "disable"
	}
	return pg, nil
}

// applyEnvOverrides matches the terminal app's behavior in
// ~/vimango/common.go: env vars take precedence over config.json so
// secrets don't have to live in the file.
func applyEnvOverrides(pg *postgresConfig) {
	if v := os.Getenv("VIMANGO_PG_PASSWORD"); v != "" {
		pg.Password = v
	}
	if v := os.Getenv("VIMANGO_PG_SSL_MODE"); v != "" {
		pg.SSLMode = v
	}
	if v := os.Getenv("VIMANGO_PG_SSL_CA_CERT"); v != "" {
		pg.SSLCACert = v
	}
}

func buildPGConnString(pg postgresConfig) string {
	conn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		pg.Host, pg.Port, pg.User, pg.Password, pg.DB, pg.SSLMode)
	if pg.SSLCACert != "" {
		conn += fmt.Sprintf(" sslrootcert=%s", pg.SSLCACert)
	}
	return conn
}

func endsWithNewline(s string) bool {
	return len(s) > 0 && s[len(s)-1] == '\n'
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "vimango-sync: "+format+"\n", args...)
	os.Exit(1)
}
