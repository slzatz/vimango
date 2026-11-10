# Vimango Database Schema

This document describes the schema of a machine's vimango SQLite database. The definitions in code are `sqliteSchema` in `init.go` (what `--init` creates) and `internal/localdb` (the search index, and write-ahead-log mode); databases older than a given column were brought up to date by migrations, so column order and some constraints differ between machines.

## Database Files

- **vimango.db** - The one database: tasks/notes, contexts, folders, keywords, sync data, and the full-text search index (`task_fts`). It is in write-ahead-log mode, so `vimango.db-wal` and `vimango.db-shm` sit beside it while it is open; copy it with `sqlite3 vimango.db ".backup copy.db"`, not `cp`.
- **fts5_vimango.db** - No longer used. It held the search index until 2026-11-10.

## Main Database: vimango.db

### Table: task

The core table storing notes/tasks.

```sql
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
    server_modified TEXT,
    taskwarrior_uuid TEXT,
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
```

This is `sqliteSchema` in `init.go`. Older databases get `server_modified`,
`taskwarrior_uuid` and the index added by `ensureSyncColumns` in
`internal/sync` on their first sync.

**Columns:**
- `id` - Internal auto-increment primary key
- `tid` - Task ID used for synchronization (**UNIQUE** constraint); NULL until the row first reaches the server
- `star` - Boolean flag for starred/favorite items
- `title` - Note/task title (required)
- `folder_tid`, `context_tid` - Deprecated server ids of the folder and context; the uuids are authoritative
- `folder_uuid`, `context_uuid` - The folder and context (default to the "none" containers)
- `note` - Main content of the note (markdown text)
- `archived` - Boolean flag for archived items
- `deleted` - Soft delete flag (items are marked deleted, not removed)
- `added` - Timestamp when task was created
- `modified` - Timestamp of last modification
- `server_modified` - The server's `modified` as this client last saw it; how sync recognizes its own pushes and detects conflicts
- `taskwarrior_uuid` - The Taskwarrior task the note is linked to, lowercased, or NULL. Set once and never cleared by sync; see `docs/taskwarrior-link-column.md`

**Important Notes:**
- The `tid` column is **UNIQUE** (not shown in some init SQL files)
- Missing columns that appear in some init SQL files: `tag`, `duetime`, `completed`, `duedate`, `created`, `startdate`

### Table: context

Organizational container for grouping tasks by context.

```sql
CREATE TABLE context (
    id INTEGER NOT NULL,
    tid INTEGER,
    title TEXT NOT NULL,
    star BOOLEAN DEFAULT FALSE,
    deleted BOOLEAN DEFAULT FALSE,
    modified TEXT DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE (tid),
    UNIQUE (title),
    CHECK (star IN (0, 1)),
    CHECK (deleted IN (0, 1))
);
```

**Columns:**
- `id` - Internal auto-increment primary key
- `tid` - Context ID for synchronization (UNIQUE)
- `title` - Context name (UNIQUE, required)
- `star` - Boolean flag for starred contexts
- `deleted` - Soft delete flag
- `modified` - Timestamp of last modification

### Table: folder

Organizational container for grouping tasks by folder.

```sql
CREATE TABLE folder (
    id INTEGER NOT NULL,
    tid INTEGER,
    title TEXT NOT NULL,
    star BOOLEAN DEFAULT FALSE,
    deleted BOOLEAN DEFAULT FALSE,
    modified TEXT DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE (tid),
    UNIQUE (title),
    CHECK (star IN (0, 1)),
    CHECK (deleted IN (0, 1))
);
```

**Columns:**
- `id` - Internal auto-increment primary key
- `tid` - Folder ID for synchronization (UNIQUE)
- `title` - Folder name (UNIQUE, required)
- `star` - Boolean flag for starred folders
- `deleted` - Soft delete flag
- `modified` - Timestamp of last modification

### Table: keyword

Keywords/tags that can be associated with tasks.

```sql
CREATE TABLE keyword (
    id INTEGER NOT NULL,
    tid INTEGER,
    title TEXT NOT NULL,
    star BOOLEAN DEFAULT FALSE,
    deleted BOOLEAN DEFAULT FALSE,
    modified TEXT DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE (tid),
    UNIQUE (title),
    CHECK (star IN (0, 1)),
    CHECK (deleted IN (0, 1))
);
```

**Columns:**
- `id` - Internal auto-increment primary key
- `tid` - Keyword ID for synchronization (UNIQUE)
- `title` - Keyword/tag name (UNIQUE, required)
- `star` - Boolean flag for starred keywords
- `deleted` - Soft delete flag
- `modified` - Timestamp of last modification

**Note:** Uses `title` column (not `name` as in some init SQL files)

### Table: task_keyword

Junction table for many-to-many relationship between tasks and keywords.

```sql
CREATE TABLE task_keyword (
    task_tid INTEGER NOT NULL,
    keyword_tid INTEGER NOT NULL,
    PRIMARY KEY (task_tid, keyword_tid),
    FOREIGN KEY(task_tid) REFERENCES task (tid),
    FOREIGN KEY(keyword_tid) REFERENCES keyword (tid)
);
```

**Columns:**
- `task_tid` - References task.tid (composite primary key)
- `keyword_tid` - References keyword.tid (composite primary key)

**Critical Note:** This junction table references the `tid` columns of task and keyword tables, NOT the `id` columns. This is different from typical junction table patterns.

### Table: sync

Tracks synchronization timestamps for different machines/endpoints.

```sql
CREATE TABLE sync (
    id INTEGER NOT NULL,
    machine TEXT NOT NULL,
    timestamp TEXT DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE (machine)
);
```

**Columns:**
- `id` - Internal auto-increment primary key
- `machine` - Machine/endpoint identifier (UNIQUE, required)
- `timestamp` - Last sync timestamp

**Typical values:** 'server', 'client'

### Table: sync_log

Logging table for synchronization operations.

```sql
CREATE TABLE sync_log (
    id INTEGER NOT NULL,
    title TEXT,
    modified TEXT,
    note TEXT,
    PRIMARY KEY (id)
);
```

**Columns:**
- `id` - Log entry ID (primary key)
- `title` - Log entry title
- `modified` - Timestamp
- `note` - Log details/content

## Search index: task_fts

```sql
CREATE VIRTUAL TABLE task_fts USING fts5(title, note, tag);
```

One row per note, with `rowid` = `task.id`.

- `title` - the note's title
- `note` - the note's text
- `tag` - the note's keywords, joined with commas

Eight triggers keep it current, named `task_fts_*`: on `task` (after insert, after delete, after update of `title`, `note` or `tid`), on `task_keyword` (after insert, update, delete) and on `keyword` (after a rename, after delete). Nothing writes the table directly. Search it with:

```sql
SELECT task.* FROM task_fts JOIN task ON task.id = task_fts.rowid
WHERE task_fts MATCH ? ORDER BY bm25(task_fts, 2.0, 1.0, 5.0);
```

FTS5 creates its own internal tables: `task_fts_data`, `task_fts_idx`, `task_fts_content`, `task_fts_docsize`, `task_fts_config`. The definitions and the migration that builds the index are in `internal/localdb/searchindex.go`; see "Search index" in `CLAUDE.md`.

## Important Schema Characteristics

### All tid Columns Are UNIQUE

Every table with a `tid` column has a UNIQUE constraint:
- task.tid (UNIQUE)
- context.tid (UNIQUE)
- folder.tid (UNIQUE)
- keyword.tid (UNIQUE)

The `tid` columns serve as synchronization identifiers and must be unique across the database.

### Junction Table Uses tid References

Unlike typical junction tables that reference primary key `id` columns, the `task_keyword` table references the `tid` columns:
- `task_keyword.task_tid` → `task.tid`
- `task_keyword.keyword_tid` → `keyword.tid`

This design aligns with the synchronization architecture where `tid` values are the stable identifiers.

### Soft Deletes

All major tables use soft deletion via the `deleted` boolean flag:
- task.deleted
- context.deleted
- folder.deleted
- keyword.deleted

Items are marked as deleted but not removed from the database, enabling synchronization and potential recovery.

### Default "none" Containers

Both context and folder have default entries with tid=1 titled "none":
- context (tid=1, title="none")
- folder (tid=1, title="none")

These serve as default containers when no specific context or folder is assigned (see task.context_tid and task.folder_tid defaults).

## Schema Initialization

The schemas in code:
- `init.go` (`sqliteSchema`, `runInit`) - what `vimango --init` creates
- `internal/localdb` - what every start then ensures: write-ahead-log mode, and the `task_fts` index with its triggers
- `schema/postgres.sql` - the PostgreSQL server schema; the sync integration tests build their databases from it

## Build Requirements

Every binary that writes `vimango.db` is built with the `fts5` tag, because
the search index's triggers run inside the writer:

```bash
CGO_ENABLED=1 go build --tags=fts5
```

`fts5` is the only tag. cgo is required (`CGO_ENABLED=0` builds are
unsupported), and `cgo` does not belong in the tag list: the toolchain sets
that constraint itself.

## Database Driver Selection

The application uses a single SQLite driver: **mattn/go-sqlite3** (CGO).
Binaries must be built with the `fts5` tag (enforced at compile time by
`fts5_guard.go` files); the pure-Go modernc.org/sqlite driver was removed
2026-07-12.
