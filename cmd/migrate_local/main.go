// migrate_local is a standalone utility to migrate existing local SQLite databases
// from the old tid-only schema to the new UUID-based schema.
//
// Usage: migrate_local /path/to/listmanager.db
//
// This utility is idempotent - it can be run multiple times safely.
package main

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

// Default UUIDs for the "none" containers (must match init.go)
const (
	DefaultContextUUID = "00000000-0000-0000-0000-000000000001"
	DefaultFolderUUID  = "00000000-0000-0000-0000-000000000002"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: migrate_local /path/to/listmanager.db")
		fmt.Println("\nThis utility migrates an existing vimango SQLite database")
		fmt.Println("from the old tid-only schema to the new UUID-based schema.")
		os.Exit(1)
	}

	dbPath := os.Args[1]

	// Check if file exists
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		fmt.Printf("Error: Database file not found: %s\n", dbPath)
		os.Exit(1)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		fmt.Printf("Error opening database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Check connection
	if err := db.Ping(); err != nil {
		fmt.Printf("Error connecting to database: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Connected to database: %s\n\n", dbPath)

	// Run migration
	if err := migrateToUUID(db); err != nil {
		fmt.Printf("Migration failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\nMigration completed successfully!")
}

func migrateToUUID(db *sql.DB) error {
	// Check if migration is already complete
	if migrationComplete(db) {
		fmt.Println("Database already migrated to UUID schema - no changes needed.")
		return nil
	}

	fmt.Println("Starting UUID migration...")

	// Step 1: Add uuid columns to container tables
	if err := addContainerUUIDColumns(db); err != nil {
		return fmt.Errorf("adding container UUID columns: %w", err)
	}

	// Step 2: Generate UUIDs for existing containers
	if err := generateContainerUUIDs(db); err != nil {
		return fmt.Errorf("generating container UUIDs: %w", err)
	}

	// Step 3: Add uuid columns to task table
	if err := addTaskUUIDColumns(db); err != nil {
		return fmt.Errorf("adding task UUID columns: %w", err)
	}

	// Step 4: Populate task uuid references from tid references
	if err := migrateTaskUUIDReferences(db); err != nil {
		return fmt.Errorf("migrating task UUID references: %w", err)
	}

	// Step 5: Add keyword_uuid column to task_keyword table
	if err := addTaskKeywordUUIDColumn(db); err != nil {
		return fmt.Errorf("adding task_keyword UUID column: %w", err)
	}

	// Step 6: Populate task_keyword uuid references
	if err := migrateTaskKeywordUUIDReferences(db); err != nil {
		return fmt.Errorf("migrating task_keyword UUID references: %w", err)
	}

	return nil
}

func migrationComplete(db *sql.DB) bool {
	// Check if uuid column exists in context table
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('context') WHERE name='uuid'").Scan(&count)
	if err != nil || count == 0 {
		return false
	}

	// Check if all contexts have UUIDs
	err = db.QueryRow("SELECT COUNT(*) FROM context WHERE uuid IS NULL OR uuid = ''").Scan(&count)
	if err != nil || count > 0 {
		return false
	}

	// Check if task has uuid columns
	err = db.QueryRow("SELECT COUNT(*) FROM pragma_table_info('task') WHERE name='context_uuid'").Scan(&count)
	if err != nil || count == 0 {
		return false
	}

	// Check if all tasks have context_uuid populated
	err = db.QueryRow("SELECT COUNT(*) FROM task WHERE context_uuid IS NULL OR context_uuid = ''").Scan(&count)
	if err != nil || count > 0 {
		return false
	}

	return true
}

func columnExists(db *sql.DB, table, column string) bool {
	var count int
	query := fmt.Sprintf("SELECT COUNT(*) FROM pragma_table_info('%s') WHERE name='%s'", table, column)
	err := db.QueryRow(query).Scan(&count)
	return err == nil && count > 0
}

func addContainerUUIDColumns(db *sql.DB) error {
	tables := []string{"context", "folder", "keyword"}

	for _, table := range tables {
		if columnExists(db, table, "uuid") {
			fmt.Printf("  Column 'uuid' already exists in %s table\n", table)
			continue
		}

		query := fmt.Sprintf("ALTER TABLE %s ADD COLUMN uuid TEXT", table)
		_, err := db.Exec(query)
		if err != nil {
			return fmt.Errorf("adding uuid column to %s: %w", table, err)
		}
		fmt.Printf("  Added 'uuid' column to %s table\n", table)
	}

	return nil
}

func generateContainerUUIDs(db *sql.DB) error {
	tables := []struct {
		name       string
		defaultTid int
		defaultUUID string
	}{
		{"context", 1, DefaultContextUUID},
		{"folder", 1, DefaultFolderUUID},
		{"keyword", 0, ""}, // keywords don't have a default
	}

	for _, t := range tables {
		// Get containers without UUIDs
		rows, err := db.Query(fmt.Sprintf("SELECT id, tid, title FROM %s WHERE uuid IS NULL OR uuid = ''", t.name))
		if err != nil {
			return fmt.Errorf("querying %s: %w", t.name, err)
		}

		var updates []struct {
			id    int
			uuid  string
			title string
		}

		for rows.Next() {
			var id int
			var tid sql.NullInt64
			var title string
			if err := rows.Scan(&id, &tid, &title); err != nil {
				rows.Close()
				return fmt.Errorf("scanning %s row: %w", t.name, err)
			}

			var newUUID string
			// Use default UUID for the "none" container (tid=1 for context/folder)
			if t.defaultUUID != "" && tid.Valid && int(tid.Int64) == t.defaultTid {
				newUUID = t.defaultUUID
			} else {
				newUUID = uuid.New().String()
			}

			updates = append(updates, struct {
				id    int
				uuid  string
				title string
			}{id, newUUID, title})
		}
		rows.Close()

		// Apply updates
		for _, u := range updates {
			_, err := db.Exec(fmt.Sprintf("UPDATE %s SET uuid = ? WHERE id = ?", t.name), u.uuid, u.id)
			if err != nil {
				return fmt.Errorf("updating %s uuid for id %d: %w", t.name, u.id, err)
			}
			fmt.Printf("  Generated UUID for %s '%s': %s\n", t.name, u.title, u.uuid)
		}

		if len(updates) == 0 {
			fmt.Printf("  All %s entries already have UUIDs\n", t.name)
		}
	}

	return nil
}

func addTaskUUIDColumns(db *sql.DB) error {
	columns := []struct {
		name         string
		defaultValue string
	}{
		{"context_uuid", DefaultContextUUID},
		{"folder_uuid", DefaultFolderUUID},
	}

	for _, col := range columns {
		if columnExists(db, "task", col.name) {
			fmt.Printf("  Column '%s' already exists in task table\n", col.name)
			continue
		}

		query := fmt.Sprintf("ALTER TABLE task ADD COLUMN %s TEXT DEFAULT '%s'", col.name, col.defaultValue)
		_, err := db.Exec(query)
		if err != nil {
			return fmt.Errorf("adding %s column to task: %w", col.name, err)
		}
		fmt.Printf("  Added '%s' column to task table (default: %s)\n", col.name, col.defaultValue)
	}

	return nil
}

func migrateTaskUUIDReferences(db *sql.DB) error {
	// Migrate context_uuid from context_tid
	result, err := db.Exec(`
		UPDATE task SET context_uuid = (
			SELECT uuid FROM context WHERE context.tid = task.context_tid
		) WHERE (context_uuid IS NULL OR context_uuid = '' OR context_uuid = ?)
		  AND context_tid IS NOT NULL
		  AND EXISTS (SELECT 1 FROM context WHERE context.tid = task.context_tid)
	`, DefaultContextUUID)
	if err != nil {
		return fmt.Errorf("migrating context_uuid: %w", err)
	}
	rows, _ := result.RowsAffected()
	fmt.Printf("  Migrated %d task context_uuid references from context_tid\n", rows)

	// Migrate folder_uuid from folder_tid
	result, err = db.Exec(`
		UPDATE task SET folder_uuid = (
			SELECT uuid FROM folder WHERE folder.tid = task.folder_tid
		) WHERE (folder_uuid IS NULL OR folder_uuid = '' OR folder_uuid = ?)
		  AND folder_tid IS NOT NULL
		  AND EXISTS (SELECT 1 FROM folder WHERE folder.tid = task.folder_tid)
	`, DefaultFolderUUID)
	if err != nil {
		return fmt.Errorf("migrating folder_uuid: %w", err)
	}
	rows, _ = result.RowsAffected()
	fmt.Printf("  Migrated %d task folder_uuid references from folder_tid\n", rows)

	return nil
}

func addTaskKeywordUUIDColumn(db *sql.DB) error {
	if columnExists(db, "task_keyword", "keyword_uuid") {
		fmt.Println("  Column 'keyword_uuid' already exists in task_keyword table")
		return nil
	}

	_, err := db.Exec("ALTER TABLE task_keyword ADD COLUMN keyword_uuid TEXT")
	if err != nil {
		return fmt.Errorf("adding keyword_uuid column: %w", err)
	}
	fmt.Println("  Added 'keyword_uuid' column to task_keyword table")

	return nil
}

func migrateTaskKeywordUUIDReferences(db *sql.DB) error {
	result, err := db.Exec(`
		UPDATE task_keyword SET keyword_uuid = (
			SELECT uuid FROM keyword WHERE keyword.tid = task_keyword.keyword_tid
		) WHERE (keyword_uuid IS NULL OR keyword_uuid = '')
		  AND keyword_tid IS NOT NULL
		  AND EXISTS (SELECT 1 FROM keyword WHERE keyword.tid = task_keyword.keyword_tid)
	`)
	if err != nil {
		return fmt.Errorf("migrating keyword_uuid: %w", err)
	}
	rows, _ := result.RowsAffected()
	fmt.Printf("  Migrated %d task_keyword keyword_uuid references from keyword_tid\n", rows)

	return nil
}
