# Vimango Database Schema

This document describes the **actual** schema of the vimango SQLite databases as they exist in production. Note that this may differ from the initialization SQL files in `cmd/create_sqlite_db/` and `cmd/create_dbs/`.

## Database Files

- **vimango.db** - Main application database containing tasks/notes, contexts, folders, keywords, and sync data
- **fts5_vimango.db** - Separate database containing the FTS5 virtual table for full-text search

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
    note TEXT,
    archived BOOLEAN DEFAULT FALSE,
    deleted BOOLEAN DEFAULT FALSE,
    added TEXT NOT NULL,
    modified TEXT DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    FOREIGN KEY(folder_tid) REFERENCES folder (tid),
    FOREIGN KEY(context_tid) REFERENCES context (tid),
    UNIQUE (tid),
    CHECK (star IN (0, 1)),
    CHECK (archived IN (0, 1)),
    CHECK (deleted IN (0, 1))
);
```

**Columns:**
- `id` - Internal auto-increment primary key
- `tid` - Task ID used for synchronization (**UNIQUE** constraint)
- `star` - Boolean flag for starred/favorite items
- `title` - Note/task title (required)
- `folder_tid` - Foreign key to folder.tid (defaults to 1 = "none")
- `context_tid` - Foreign key to context.tid (defaults to 1 = "none")
- `note` - Main content of the note (markdown text)
- `archived` - Boolean flag for archived items
- `deleted` - Soft delete flag (items are marked deleted, not removed)
- `added` - Timestamp when task was created
- `modified` - Timestamp of last modification

**Important Notes:**
- The `tid` column is **UNIQUE** (not shown in some init SQL files)
- Has an `archived` column (not present in some init SQL files)
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

## FTS Database: fts5_vimango.db

### Virtual Table: fts

Full-text search index using SQLite's FTS5 extension.

```sql
CREATE VIRTUAL TABLE fts USING fts5 (
    title,
    note,
    tag,
    tid UNINDEXED
);
```

**Columns:**
- `title` - Task title (indexed for full-text search)
- `note` - Task note content (indexed for full-text search)
- `tag` - Task tags (indexed for full-text search)
- `tid` - Task ID (UNINDEXED, used for linking back to task.tid)

**Notes:**
- The `tid` column is not indexed for search but allows linking FTS results back to the main task table
- FTS5 automatically creates internal tables: `fts_data`, `fts_idx`, `fts_content`, `fts_docsize`, `fts_config`
- Uses `tid` (not `lm_id` as shown in some init SQL files)

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

While the actual schema is documented above, initialization code can be found at:
- `cmd/create_sqlite_db/sqlite_init.sql` - SQL initialization script
- `cmd/create_sqlite_db/main.go` - Database creation code including FTS table
- `cmd/create_dbs/sqlite_init.sql` - Alternative initialization script

**Warning:** These initialization files may not exactly match the production schema documented above. The production database has evolved beyond the initial schema definitions.

## Build Requirements

To enable FTS5 full-text search support, build with the `fts5` tag:

```bash
# Pure Go build
CGO_ENABLED=0 go build --tags=fts5

# CGO build
CGO_ENABLED=1 go build --tags="fts5,cgo"
```

## Database Driver Selection

The application supports two SQLite drivers:
- **modernc.org/sqlite** (Pure Go, default) - Works on all platforms
- **mattn/go-sqlite3** (CGO-based) - Only available on Linux/Unix with CGO enabled

Use `--go-sqlite` or `--cgo-sqlite` runtime flags to select the driver.
