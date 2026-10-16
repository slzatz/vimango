package main

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
)

// Runs against a throwaway Postgres, like internal/sync's integration tests,
// and is skipped unless VIMANGO_TEST_PG is set (see ~/vimango/CLAUDE.md).
func testServer(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("VIMANGO_TEST_PG")
	if dsn == "" {
		t.Skip("VIMANGO_TEST_PG not set")
	}
	admin, err := sql.Open("postgres", dsn+" dbname=postgres")
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	name := fmt.Sprintf("vimango_migrate_test_%d", rand.Int63())
	if _, err := admin.Exec("CREATE DATABASE " + name); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if admin, err := sql.Open("postgres", dsn+" dbname=postgres"); err == nil {
			admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)")
			admin.Close()
		}
	})
	pg, err := sql.Open("postgres", dsn+" dbname="+name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pg.Close() })
	schema, err := os.ReadFile(filepath.Join("..", "create_dbs", "postgres_init4.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pg.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	return pg
}

func linkedNote(uuid, text string) string {
	return "---\ntaskwarrior: " + uuid + "\n---\n\n[Open task in Taskchamp](taskchampdev://task/" + uuid + ")\n\n" + text
}

// A note edited between reading and applying rolls the whole run back, so
// rerunning after a sync is always safe.
func TestApplyRollsBackWhenANoteChanged(t *testing.T) {
	pg := testServer(t)
	a, b := "11111111-2222-3333-4444-555555555555", "66666666-7777-8888-9999-000000000000"
	if _, err := pg.Exec("INSERT INTO task (title, note, added) VALUES ('a', $1, now()), ('b', $2, now());",
		linkedNote(a, "first\n"), linkedNote(b, "second\n")); err != nil {
		t.Fatal(err)
	}
	cs, _, err := findCandidates(pg)
	if err != nil || len(cs) != 2 {
		t.Fatalf("candidates = %d, err %v", len(cs), err)
	}
	if _, err := pg.Exec("UPDATE task SET note = note || 'edited elsewhere', modified = now() + interval '1 second' WHERE title = 'b';"); err != nil {
		t.Fatal(err)
	}

	if err := apply(pg, cs); err == nil {
		t.Fatal("apply succeeded over a changed note")
	}
	var linked int
	if err := pg.QueryRow("SELECT COUNT(*) FROM task WHERE taskwarrior_uuid IS NOT NULL OR note NOT LIKE '---%';").Scan(&linked); err != nil {
		t.Fatal(err)
	}
	if linked != 0 {
		t.Errorf("%d rows changed despite the rollback", linked)
	}

	cs, _, err = findCandidates(pg)
	if err != nil {
		t.Fatal(err)
	}
	if err := apply(pg, cs); err != nil {
		t.Fatalf("rerun failed: %v", err)
	}
	var note string
	if err := pg.QueryRow("SELECT note FROM task WHERE title = 'b' AND taskwarrior_uuid = $1;", b).Scan(&note); err != nil {
		t.Fatal(err)
	}
	if note != "second\nedited elsewhere" {
		t.Errorf("b = %q, want the other edit kept", note)
	}
}
