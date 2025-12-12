// fix_uuid_sync is a one-time utility to align UUIDs between PostgreSQL and local SQLite.
//
// Problem: During initial migration, each database independently generated UUIDs
// for containers. But for sync to work, the same container must have the same UUID
// everywhere.
//
// Solution: Use PostgreSQL as the source of truth. Copy UUIDs from PostgreSQL to
// local SQLite by matching on tid (which is already synchronized).
//
// Usage:
//   fix_uuid_sync --pg-host=HOST --pg-port=PORT --pg-user=USER --pg-password=PASS --pg-db=DB --local-db=/path/to/listmanager.db
//
// Or with config file:
//   fix_uuid_sync --config=/path/to/config.json --local-db=/path/to/listmanager.db
package main

import (
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

type Config struct {
	Postgres struct {
		Host     string `json:"host"`
		Port     string `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		DB       string `json:"db"`
	} `json:"postgres"`
}

type UUIDMapping struct {
	tid  int
	uuid string
}

func main() {
	// Command line flags
	pgHost := flag.String("pg-host", "", "PostgreSQL host")
	pgPort := flag.String("pg-port", "5432", "PostgreSQL port")
	pgUser := flag.String("pg-user", "", "PostgreSQL user")
	pgPassword := flag.String("pg-password", "", "PostgreSQL password")
	pgDB := flag.String("pg-db", "", "PostgreSQL database name")
	configFile := flag.String("config", "", "Path to config.json (alternative to individual pg flags)")
	localDB := flag.String("local-db", "", "Path to local SQLite database")
	dryRun := flag.Bool("dry-run", false, "Show what would be changed without making changes")

	flag.Parse()

	if *localDB == "" {
		fmt.Println("Error: --local-db is required")
		flag.Usage()
		os.Exit(1)
	}

	// Load PostgreSQL config
	var config Config
	if *configFile != "" {
		data, err := os.ReadFile(*configFile)
		if err != nil {
			fmt.Printf("Error reading config file: %v\n", err)
			os.Exit(1)
		}
		if err := json.Unmarshal(data, &config); err != nil {
			fmt.Printf("Error parsing config file: %v\n", err)
			os.Exit(1)
		}
	} else {
		if *pgHost == "" || *pgUser == "" || *pgDB == "" {
			fmt.Println("Error: Either --config or PostgreSQL connection flags are required")
			flag.Usage()
			os.Exit(1)
		}
		config.Postgres.Host = *pgHost
		config.Postgres.Port = *pgPort
		config.Postgres.User = *pgUser
		config.Postgres.Password = *pgPassword
		config.Postgres.DB = *pgDB
	}

	// Connect to PostgreSQL
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		config.Postgres.Host, config.Postgres.Port, config.Postgres.User,
		config.Postgres.Password, config.Postgres.DB)

	pgConn, err := sql.Open("postgres", connStr)
	if err != nil {
		fmt.Printf("Error connecting to PostgreSQL: %v\n", err)
		os.Exit(1)
	}
	defer pgConn.Close()

	if err := pgConn.Ping(); err != nil {
		fmt.Printf("Error pinging PostgreSQL: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Connected to PostgreSQL: %s/%s\n", config.Postgres.Host, config.Postgres.DB)

	// Connect to SQLite
	sqliteConn, err := sql.Open("sqlite", *localDB)
	if err != nil {
		fmt.Printf("Error connecting to SQLite: %v\n", err)
		os.Exit(1)
	}
	defer sqliteConn.Close()

	if err := sqliteConn.Ping(); err != nil {
		fmt.Printf("Error pinging SQLite: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Connected to SQLite: %s\n\n", *localDB)

	if *dryRun {
		fmt.Println("=== DRY RUN MODE - No changes will be made ===\n")
	}

	// Sync UUIDs for each container type
	for _, containerType := range []string{"context", "folder", "keyword"} {
		if err := syncContainerUUIDs(pgConn, sqliteConn, containerType, *dryRun); err != nil {
			fmt.Printf("Error syncing %s UUIDs: %v\n", containerType, err)
			os.Exit(1)
		}
	}

	// Update task references
	if err := updateTaskReferences(sqliteConn, *dryRun); err != nil {
		fmt.Printf("Error updating task references: %v\n", err)
		os.Exit(1)
	}

	// Update task_keyword references
	if err := updateTaskKeywordReferences(sqliteConn, *dryRun); err != nil {
		fmt.Printf("Error updating task_keyword references: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n=== UUID sync complete ===")
}

func syncContainerUUIDs(pgConn, sqliteConn *sql.DB, containerType string, dryRun bool) error {
	fmt.Printf("Syncing %s UUIDs from PostgreSQL to SQLite...\n", containerType)

	// Fetch UUID mappings from PostgreSQL
	query := fmt.Sprintf("SELECT tid, uuid FROM %s WHERE uuid IS NOT NULL", containerType)
	rows, err := pgConn.Query(query)
	if err != nil {
		return fmt.Errorf("error fetching from PostgreSQL: %w", err)
	}
	defer rows.Close()

	mappings := []UUIDMapping{}
	for rows.Next() {
		var m UUIDMapping
		var uuid sql.NullString
		if err := rows.Scan(&m.tid, &uuid); err != nil {
			return fmt.Errorf("error scanning row: %w", err)
		}
		if uuid.Valid {
			m.uuid = uuid.String
			mappings = append(mappings, m)
		}
	}

	fmt.Printf("  Found %d %s entries in PostgreSQL with UUIDs\n", len(mappings), containerType)

	// Update SQLite with PostgreSQL UUIDs
	updateCount := 0
	for _, m := range mappings {
		// Check current UUID in SQLite
		var currentUUID sql.NullString
		checkQuery := fmt.Sprintf("SELECT uuid FROM %s WHERE tid = ?", containerType)
		err := sqliteConn.QueryRow(checkQuery, m.tid).Scan(&currentUUID)
		if err == sql.ErrNoRows {
			fmt.Printf("  WARNING: tid %d exists in PostgreSQL but not in SQLite\n", m.tid)
			continue
		} else if err != nil {
			return fmt.Errorf("error checking SQLite: %w", err)
		}

		if currentUUID.String == m.uuid {
			continue // Already matches
		}

		if dryRun {
			fmt.Printf("  Would update %s tid=%d: %s -> %s\n", containerType, m.tid, currentUUID.String, m.uuid)
		} else {
			updateQuery := fmt.Sprintf("UPDATE %s SET uuid = ? WHERE tid = ?", containerType)
			_, err := sqliteConn.Exec(updateQuery, m.uuid, m.tid)
			if err != nil {
				return fmt.Errorf("error updating SQLite: %w", err)
			}
		}
		updateCount++
	}

	if dryRun {
		fmt.Printf("  Would update %d %s UUIDs\n", updateCount, containerType)
	} else {
		fmt.Printf("  Updated %d %s UUIDs\n", updateCount, containerType)
	}

	return nil
}

func updateTaskReferences(sqliteConn *sql.DB, dryRun bool) error {
	fmt.Println("\nUpdating task context_uuid and folder_uuid references...")

	// Update context_uuid based on context_tid
	if dryRun {
		var count int
		err := sqliteConn.QueryRow(`
			SELECT COUNT(*) FROM task t
			JOIN context c ON c.tid = t.context_tid
			WHERE t.context_uuid != c.uuid OR t.context_uuid IS NULL
		`).Scan(&count)
		if err != nil {
			return fmt.Errorf("error counting context_uuid mismatches: %w", err)
		}
		fmt.Printf("  Would update %d task context_uuid references\n", count)
	} else {
		result, err := sqliteConn.Exec(`
			UPDATE task SET context_uuid = (
				SELECT uuid FROM context WHERE context.tid = task.context_tid
			) WHERE context_tid IS NOT NULL AND EXISTS (
				SELECT 1 FROM context WHERE context.tid = task.context_tid
			)
		`)
		if err != nil {
			return fmt.Errorf("error updating context_uuid: %w", err)
		}
		rows, _ := result.RowsAffected()
		fmt.Printf("  Updated %d task context_uuid references\n", rows)
	}

	// Update folder_uuid based on folder_tid
	if dryRun {
		var count int
		err := sqliteConn.QueryRow(`
			SELECT COUNT(*) FROM task t
			JOIN folder f ON f.tid = t.folder_tid
			WHERE t.folder_uuid != f.uuid OR t.folder_uuid IS NULL
		`).Scan(&count)
		if err != nil {
			return fmt.Errorf("error counting folder_uuid mismatches: %w", err)
		}
		fmt.Printf("  Would update %d task folder_uuid references\n", count)
	} else {
		result, err := sqliteConn.Exec(`
			UPDATE task SET folder_uuid = (
				SELECT uuid FROM folder WHERE folder.tid = task.folder_tid
			) WHERE folder_tid IS NOT NULL AND EXISTS (
				SELECT 1 FROM folder WHERE folder.tid = task.folder_tid
			)
		`)
		if err != nil {
			return fmt.Errorf("error updating folder_uuid: %w", err)
		}
		rows, _ := result.RowsAffected()
		fmt.Printf("  Updated %d task folder_uuid references\n", rows)
	}

	return nil
}

func updateTaskKeywordReferences(sqliteConn *sql.DB, dryRun bool) error {
	fmt.Println("\nUpdating task_keyword keyword_uuid references...")

	if dryRun {
		var count int
		err := sqliteConn.QueryRow(`
			SELECT COUNT(*) FROM task_keyword tk
			JOIN keyword k ON k.tid = tk.keyword_tid
			WHERE tk.keyword_uuid != k.uuid OR tk.keyword_uuid IS NULL
		`).Scan(&count)
		if err != nil {
			return fmt.Errorf("error counting keyword_uuid mismatches: %w", err)
		}
		fmt.Printf("  Would update %d task_keyword keyword_uuid references\n", count)
	} else {
		result, err := sqliteConn.Exec(`
			UPDATE task_keyword SET keyword_uuid = (
				SELECT uuid FROM keyword WHERE keyword.tid = task_keyword.keyword_tid
			) WHERE keyword_tid IS NOT NULL AND EXISTS (
				SELECT 1 FROM keyword WHERE keyword.tid = task_keyword.keyword_tid
			)
		`)
		if err != nil {
			return fmt.Errorf("error updating keyword_uuid: %w", err)
		}
		rows, _ := result.RowsAffected()
		fmt.Printf("  Updated %d task_keyword keyword_uuid references\n", rows)
	}

	return nil
}
