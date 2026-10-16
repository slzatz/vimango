// migrate_taskwarrior_links is the one-time step 5 of
// docs/taskwarrior-link-column.md: it moves each task note's Taskwarrior
// link out of its text and into the task.taskwarrior_uuid column on the
// server.
//
// For every live note whose leading frontmatter holds `taskwarrior: <uuid>`,
// it sets the column, takes the link lines out of the body (cleanTaskNote)
// and bumps modified, so every client pulls the cleaned row on its next
// sync. A conflict copy ("<title> (conflict YYYY-MM-DD)") whose task another
// note also carries is cleaned but not linked: conflict copies don't inherit
// a link.
//
// Sync every client first, so that no linked note has a local edit waiting:
// that edit would lose to the cleaned row and come back as a conflict copy
// with the old text. Each update is guarded by the modified the command
// read, and all of them run in one transaction, so a note that changed
// underneath it rolls everything back and the command can just be rerun.
//
// Usage:
//
//	migrate_taskwarrior_links --dry-run   # print what would change
//	migrate_taskwarrior_links             # apply it
//
// Connection values come from the postgres section of --config (default
// config.json in the current directory).
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	_ "github.com/lib/pq"
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

// A note to migrate.
type candidate struct {
	tid      int
	title    string
	modified string
	existing sql.NullString // taskwarrior_uuid already on the row
	cleaned
	link bool // set the column (false for a conflict copy)
}

func main() {
	configPath := flag.String("config", "config.json", "config.json holding the postgres section")
	dryRun := flag.Bool("dry-run", false, "print what would change without changing anything")
	flag.Parse()

	pg, err := openPostgres(*configPath)
	if err != nil {
		fatal("%v", err)
	}
	defer pg.Close()

	candidates, ignored, err := findCandidates(pg)
	if err != nil {
		fatal("%v", err)
	}
	report(candidates, ignored)
	if len(candidates) == 0 {
		fmt.Println("Nothing to migrate.")
		return
	}
	if *dryRun {
		fmt.Println("Dry run: nothing was changed.")
		return
	}
	if err := apply(pg, candidates); err != nil {
		fatal("%v\nNothing was changed. Sync, then run it again.", err)
	}
	fmt.Printf("Migrated %d notes. Every client picks them up on its next sync.\n", len(candidates))
}

func openPostgres(path string) (*sql.DB, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %v", path, err)
	}
	var cfg struct {
		Postgres postgresConfig `json:"postgres"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %v", path, err)
	}
	p := cfg.Postgres
	// connect_timeout as in vimango-sync: a blackholed route would otherwise
	// hang for over a minute.
	// Values quoted, so an empty or spaced one can't swallow the next key.
	q := func(v string) string {
		return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(v) + "'"
	}
	conn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s connect_timeout=10",
		q(p.Host), q(p.Port), q(p.User), q(p.Password), q(p.DB), q(p.SSLMode))
	if p.SSLCACert != "" {
		conn += " sslrootcert=" + q(p.SSLCACert)
	}
	db, err := sql.Open("postgres", conn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connecting to %s: %v", p.Host, err)
	}
	return db, nil
}

// findCandidates returns the notes to migrate, and the titles of notes that
// mention a taskwarrior line without being linked by it (left alone).
func findCandidates(pg *sql.DB) ([]candidate, []string, error) {
	rows, err := pg.Query("SELECT tid, title, note, modified::text, taskwarrior_uuid FROM task " +
		"WHERE deleted = false AND note LIKE '%taskwarrior:%' ORDER BY tid;")
	if err != nil {
		return nil, nil, fmt.Errorf("reading notes: %v", err)
	}
	defer rows.Close()

	var cs []candidate
	var ignored []string
	for rows.Next() {
		var c candidate
		var note string
		if err := rows.Scan(&c.tid, &c.title, &note, &c.modified, &c.existing); err != nil {
			return nil, nil, fmt.Errorf("reading notes: %v", err)
		}
		cl, ok := cleanTaskNote(note)
		if !ok {
			ignored = append(ignored, fmt.Sprintf("%d %q", c.tid, c.title))
			continue
		}
		c.cleaned = cl
		cs = append(cs, c)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("reading notes: %v", err)
	}

	// A conflict copy keeps its text cleaned but gets no link when another
	// note carries the same task; otherwise it is the task's only note.
	original := map[string]bool{}
	for _, c := range cs {
		if !isConflictCopy(c.title) {
			original[c.uuid] = true
		}
	}
	for i := range cs {
		cs[i].link = !(isConflictCopy(cs[i].title) && original[cs[i].uuid])
	}
	return cs, ignored, nil
}

func report(cs []candidate, ignored []string) {
	for _, c := range cs {
		fmt.Printf("tid %d %q\n", c.tid, c.title)
		switch {
		case !c.link:
			fmt.Printf("  conflict copy: text cleaned, not linked (task %s)\n", c.uuid)
		case c.existing.Valid && c.existing.String != c.uuid:
			fmt.Printf("  already linked to %s; keeps that link, text cleaned\n", c.existing.String)
		default:
			fmt.Printf("  link -> %s\n", c.uuid)
		}
		for _, l := range c.removed {
			fmt.Printf("  - %s\n", l)
		}
		fmt.Printf("  now begins: %s\n", preview(c.note))
	}
	for _, s := range ignored {
		fmt.Printf("left alone (mentions taskwarrior: but isn't linked by it): %s\n", s)
	}
	fmt.Printf("\n%d notes to migrate, %d left alone.\n", len(cs), len(ignored))
}

// preview is the first non-blank line of a note, cut short.
func preview(note string) string {
	for _, l := range strings.Split(note, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			if r := []rune(l); len(r) > 70 {
				return string(r[:70]) + "…"
			}
			return l
		}
	}
	return "(empty)"
}

func apply(pg *sql.DB, cs []candidate) error {
	tx, err := pg.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, c := range cs {
		var link, note sql.NullString
		if c.link {
			link = sql.NullString{String: c.uuid, Valid: true}
		}
		// vimango stores an empty note as NULL.
		if c.note != "" {
			note = sql.NullString{String: c.note, Valid: true}
		}
		// COALESCE: a link is written once, so one already there stays.
		res, err := tx.Exec("UPDATE task SET note=$1, taskwarrior_uuid=COALESCE(taskwarrior_uuid, $2), modified=now() "+
			"WHERE tid=$3 AND deleted=false AND modified::text=$4;", note, link, c.tid, c.modified)
		if err != nil {
			return fmt.Errorf("updating tid %d: %v", c.tid, err)
		}
		if n, err := res.RowsAffected(); err != nil || n != 1 {
			return fmt.Errorf("tid %d %q changed while this ran", c.tid, c.title)
		}
	}
	return tx.Commit()
}

func fatal(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "migrate_taskwarrior_links: "+format+"\n", args...)
	os.Exit(1)
}
