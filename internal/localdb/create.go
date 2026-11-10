package localdb

import (
	"database/sql"
	"fmt"
)

// The two containers every database has, both titled "none": what a note is
// in until it is put somewhere. Their uuids are the same on every machine and
// on the server, and are the defaults of task.context_uuid and
// task.folder_uuid in the schema.
const (
	DefaultContextUUID = "00000000-0000-0000-0000-000000000001"
	DefaultFolderUUID  = "00000000-0000-0000-0000-000000000002"
)

// Schema is a new vimango.db: the tables as `vimango --init` creates them.
// The search index and its triggers are not here; Migrate adds them, as it
// does for a database of any age.
const Schema = `
CREATE TABLE context (
	id INTEGER NOT NULL,
	tid INTEGER,
	uuid TEXT NOT NULL UNIQUE,
	title TEXT NOT NULL,
	star BOOLEAN DEFAULT FALSE,
	deleted BOOLEAN DEFAULT FALSE,
	modified TEXT DEFAULT CURRENT_TIMESTAMP,
	server_modified TEXT,
	PRIMARY KEY (id),
	UNIQUE (tid),
	UNIQUE (title),
	CHECK (star IN (0, 1)),
	CHECK (deleted IN (0, 1))
);
CREATE TABLE folder (
	id INTEGER NOT NULL,
	tid INTEGER,
	uuid TEXT NOT NULL UNIQUE,
	title TEXT NOT NULL,
	star BOOLEAN DEFAULT FALSE,
	deleted BOOLEAN DEFAULT FALSE,
	modified TEXT DEFAULT CURRENT_TIMESTAMP,
	server_modified TEXT,
	PRIMARY KEY (id),
	UNIQUE (tid),
	UNIQUE (title),
	CHECK (star IN (0, 1)),
	CHECK (deleted IN (0, 1))
);
CREATE TABLE keyword (
	id INTEGER NOT NULL,
	tid INTEGER,
	uuid TEXT NOT NULL UNIQUE,
	title TEXT NOT NULL,
	star BOOLEAN DEFAULT FALSE,
	deleted BOOLEAN DEFAULT FALSE,
	modified TEXT DEFAULT CURRENT_TIMESTAMP,
	server_modified TEXT,
	PRIMARY KEY (id),
	UNIQUE (tid),
	UNIQUE (title),
	CHECK (star IN (0, 1)),
	CHECK (deleted IN (0, 1))
);
CREATE TABLE task (
	id INTEGER NOT NULL,
	tid INTEGER,
	star BOOLEAN DEFAULT FALSE,
	title TEXT NOT NULL,
	folder_tid INTEGER DEFAULT 1,
	context_tid INTEGER DEFAULT 1,
	folder_uuid TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000002',
	context_uuid TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000001',
	note TEXT,
	archived BOOLEAN DEFAULT FALSE,
	deleted BOOLEAN DEFAULT FALSE,
	added TEXT NOT NULL,
	modified TEXT DEFAULT CURRENT_TIMESTAMP,
	server_modified TEXT, -- the server's modified as last seen; see internal/sync
	taskwarrior_uuid TEXT, -- linked Taskwarrior task; see docs/taskwarrior-link-column.md
	PRIMARY KEY (id),
	FOREIGN KEY(folder_uuid) REFERENCES folder (uuid),
	FOREIGN KEY(context_uuid) REFERENCES context (uuid),
	UNIQUE (tid),
	CHECK (star IN (0, 1)),
	CHECK (archived IN (0, 1)),
	CHECK (deleted IN (0, 1))
);
CREATE INDEX task_taskwarrior_uuid_idx ON task (taskwarrior_uuid)
	WHERE taskwarrior_uuid IS NOT NULL AND deleted = false;
CREATE TABLE sync (
	id INTEGER NOT NULL,
	machine TEXT NOT NULL,
	timestamp TEXT DEFAULT CURRENT_TIMESTAMP,
	PRIMARY KEY (id),
	UNIQUE (machine)
);
CREATE TABLE task_keyword (
	task_tid INTEGER NOT NULL,
	keyword_tid INTEGER,
	keyword_uuid TEXT NOT NULL,
	PRIMARY KEY (task_tid, keyword_uuid),
	FOREIGN KEY(task_tid) REFERENCES task (tid),
	FOREIGN KEY(keyword_uuid) REFERENCES keyword (uuid)
);
CREATE TABLE sync_log (
	id INTEGER NOT NULL,
	title TEXT,
	modified TEXT,
	note TEXT,
	PRIMARY KEY (id)
);
`

// Create makes an empty database into a vimango.db: the schema, the two
// default containers, the sync watermarks, and then everything Migrate
// ensures. It is what `vimango --init` runs, and what a test that wants a
// database runs.
func Create(db *sql.DB) error {
	if _, err := db.Exec(Schema); err != nil {
		return fmt.Errorf("creating the schema: %w", err)
	}
	for _, stmt := range []struct {
		sql  string
		args []any
	}{
		{"INSERT INTO context (title, tid, uuid) VALUES ('none', 1, ?);", []any{DefaultContextUUID}},
		{"INSERT INTO folder (title, tid, uuid) VALUES ('none', 1, ?);", []any{DefaultFolderUUID}},
		{"INSERT INTO sync (machine) VALUES ('server');", nil},
		{"INSERT INTO sync (machine) VALUES ('client');", nil},
	} {
		if _, err := db.Exec(stmt.sql, stmt.args...); err != nil {
			return fmt.Errorf("%s %w", stmt.sql, err)
		}
	}
	if _, err := Migrate(db); err != nil {
		return fmt.Errorf("preparing the database: %w", err)
	}
	return nil
}
