# Store a note's Taskwarrior link in a column instead of the note body

## Context

A task note is linked to its Taskwarrior task by a `taskwarrior: <uuid>` frontmatter line. The note body
also holds an `[Open task in Taskchamp](taskchampdev://task/<uuid>)` line. Both were chosen because they
needed no schema change and travelled with the existing sync. But they are machine data sitting in
user-editable text:
- They push the real content down.
- Deleting or mistyping the line unlinks the note without any warning.
- A conflict copy duplicates the link.
- Every lookup is a `LIKE` scan over all bodies plus a frontmatter parse.

Goal: put the uuid in a `task.taskwarrior_uuid` column, synced like any other column, and have the body
hold only user content. Both the frontmatter line and the "Open task" line go away. The app UI takes over
the "open task" role.

Taskchamp and taskwarrior_macos keep sending `vimango://task-note?uuid=…`, and Taskchamp keeps its
`vimango` annotation. Only the vimango side changes how it stores and finds the link.

## Handoff notes (read first in a new session)

- **Status (2026-10-16):** design approved in discussion and reviewed against the code (the review's changes
  are folded in below: pull-side link rule, backfill placement, fresh schemas, ALTER race, hybrid's title
  match). **Step 1 done**: the production column and index were added 2026-10-16, after backup
  `vimango-20261016-0752.dump`. **Step 2 done** (Go sync + desktop SQLite, with integration tests). Next is
  step 3. Last related commits:
  vimango_ios `7bbcdeb` (dropped `project:` from new task notes' frontmatter) and `64303b8` (hyphen follow-up).
  vimango_hybrid's port (`TaskNote.swift` `body`) still writes `project:`. That doesn't matter, since this plan
  removes the body entirely.
- **Repos and branches:** `~/vimango` (Go CLI + `vimango-sync`), `~/vimango_hybrid`, `~/vimango_ios`, `~/taskchamp`
  (docs only), and `~/taskwarrior_macos` (doc comment only). All are on `dev`. Commit locally in each, with the
  Co-Authored-By line; **never push** (the user pushes).
- **Read each repo's CLAUDE.md first.** vimango_ios's "Local database and sync" section explains the sync
  contract this must follow. `~/vimango/BACKUP_RESTORE.md` covers backup and restore.
- **Line numbers are hints as of 2026-10-16.** Find code by symbol name.
- **Hard ordering constraint:** a new client build SELECTs `taskwarrior_uuid` from Postgres and fails if the column
  isn't there. Step 1 must be done before any new build syncs. The user runs the production `ALTER TABLE`;
  confirm with them first.
- **Do it one step at a time.** Each step is a separate commit, tested and confirmed with the user before the next.

## Who is affected

| Codebase | Today | Change |
|---|---|---|
| Postgres (`task` table) | no link field | add a nullable `taskwarrior_uuid TEXT` column and an index |
| `~/vimango` Go: `internal/sync/sync.go` and the desktop `vimango.db` | doesn't know about links | carry the column in both directions; add it to the local SQLite |
| `~/vimango_hybrid` (reads the desktop `vimango.db`, synced by `vimango-sync`) | `Repository.entry(forTaskUUID:)` does a LIKE scan and parses frontmatter; `TaskNote.swift` writes the body | look up by the column; set the column when it creates a note; add an "Open task" command |
| `~/vimango_ios` | `Note.taskUUID` is parsed from the body; `LocalStore.fetchNote(taskUUID:)` does a LIKE scan | stored column, pulled and pushed |
| Go terminal client | sees the frontmatter as text | sees nothing (accepted loss) |

The Go client uses explicit column lists everywhere (pull `SELECT` at sync.go:451, push `INSERT`/`UPDATE`
at 1517/1547, conflict copy at 1234). So a client that hasn't been updated ignores the column and never
overwrites it. That makes the rollout order flexible.

## Design decisions

- **Column:** `taskwarrior_uuid TEXT NULL`, lowercased, on `task` in Postgres, the desktop `vimango.db`, and
  iOS `note`. Add a partial index `WHERE taskwarrior_uuid IS NOT NULL AND deleted = false`.
- **Not unique.** Two devices that are both offline could each create a note for the same task. A unique
  index would make the second push fail forever. Lookups keep "newest `modified` wins", as today.
- **Conflict copies don't inherit the link** (Go `saveConflictCopy`, iOS conflict copy in `applyPulled`). The
  copy is a side note; the original stays linked.
- **The link is written once, and a push never clears it.** It is set when a note is created for a task, or by
  `linkTask`, and never changed. Push UPDATEs write `taskwarrior_uuid = COALESCE($n, taskwarrior_uuid)`. The
  `modified = server_modified` guard is not enough on its own. A machine upgraded after the migration
  pulled the cleaned body while it was still running the old build, so its row matches `server_modified` but
  has no link locally. Without COALESCE, its next edit would push NULL over the server's link.
- **A pull never clears a local link either.** The case: a client links an existing note (iOS `linkTask`,
  hybrid's title match) while another device edits the same note. The pull takes the server row, which has no
  link, and the conflict copy doesn't inherit one, so the link is lost. So pull UPDATEs also write
  `COALESCE(server, local)`, and when the local row has a link the server lacks, the row stays dirty so the
  link goes up:
  - iOS: `dirty = 1`.
  - Go: stamp `modified = datetime('now')` instead of `r.nextClient`. The row then pushes on the *next* run.
    It isn't pushed in this one, because the copy fetched before the pull is stale and its push guard would
    refuse it.
- **No unlink, ever.** Because a NULL never overwrites a link, nothing can remove one. To relink, delete the note
  or edit the database by hand. Accepted.
- **Lowercase at every write site.** Not enforced by a CHECK constraint: a client that wrote an uppercase uuid
  would then fail its push forever, which is the same problem as the unique index.
- **`sameContent`** (Go) leaves the link *out*. This reverses the original plan. The pull merges links
  (COALESCE plus keep-dirty) instead of picking a winner, so a link-only difference is already handled without
  treating the row as a conflict. Including the link would only produce a conflict copy with identical text.
  iOS `applyPulled` has no `sameContent`: a dirty row the server changed is always a conflict there.
- **A new task note's body starts empty.** It no longer gets a frontmatter block or the "Open task" line. This
  also removes `TaskNoteRequest.body`, `body(_:linkedTo:)`, `frontmatterLine` and `frontmatter(of:linksTo:)` in
  both Swift ports. `taskUUID(inFrontmatterOf:)` survives only in the migration.

## Every database gets the column: Postgres plus each local SQLite

One Postgres database and about half a dozen local SQLite databases need the column. They are:
- **Postgres:** migrated by hand once (step 1).
- **The desktop `vimango.db` on every machine running the vimango CLI and/or vimango_hybrid** (Macs and Linux).
  The local `fts5_vimango.db` doesn't need it.
- **The iOS VimNotes `vimango.sqlite`** on each device.

The local migration is built into the apps, not a script you run per machine. That's how `server_modified`
and the `uuid` columns were added before (sync.go:855, app.go:426/494/533). Each machine is migrated the first
time a new build starts there:
- **Go vimango and `vimango-sync`:** at startup, add the column and index if missing (idempotent).
- **vimango_hybrid:** the same check when it opens `vimango.db`, in case it starts before `vimango-sync` has.
- **iOS:** a GRDB migration `"v2"`, which runs once per device.

**Backfill.** A newly added column is NULL on every row. Sync only pulls rows whose `modified` is past the
watermark, so links already on the server would never come down. So every sync, in both Go and iOS, starts with
`SELECT tid, taskwarrior_uuid FROM task WHERE taskwarrior_uuid IS NOT NULL AND deleted = false` and runs
`UPDATE … SET taskwarrior_uuid = ? WHERE tid = ? AND taskwarrior_uuid IS NULL` locally, without touching
`modified` or dirty state.
- Linked notes number in the dozens at most, so running it on every sync is cheap.
- It needs no "backfill pending" flag, and it repairs a machine whenever it was upgraded.
- It makes the local data correct whether a machine is upgraded before or after the step 5 data migration.
  It does not relax the hard ordering above: a new build still can't sync until step 1 is done.

**Fresh databases.** `ensureSyncColumns` only runs at sync time, on existing databases. So the column also goes
into `init.go`'s `CREATE TABLE task` (`--init`). The integration test's `clientSchema` deliberately leaves it out,
as it already does `server_modified`, so every test also exercises `ensureSyncColumns`.

**Concurrent ALTER.** hybrid and `vimango-sync` may both find the column missing and add it at the same moment.
Treat SQLite's "duplicate column name" error as success, and create the index with `IF NOT EXISTS`.

**What each machine has to do** is rebuild and run the new vimango (and hybrid, on the Macs) and install the
new VimNotes build. Machines that are never upgraded stay harmless: an old build neither reads nor writes
the column. The exception is task notes created from such a machine, which don't happen on the Linux CLI
machines anyway. Upgrade the Macs and the phone before running step 5.

## Steps (each lands and is tested before the next)

1. **Back up the server** (`~/vimango/BACKUP_RESTORE.md`), then on Postgres:
   `ALTER TABLE task ADD COLUMN taskwarrior_uuid TEXT;` plus the partial index. Add both to
   `cmd/create_dbs/postgres_init4.sql`. The integration tests load that file, so step 2's tests depend on it.
2. **Go sync + desktop SQLite:**
   - Add the column and index in `Syncer.ensureSyncColumns()` (sync.go:846, which already adds
     `server_modified`). Both the CLI's sync and `cmd/vimango-sync` go through `internal/sync`. The CLI itself
     never reads the column.
   - Add it to `init.go`'s `CREATE TABLE task` (see "Fresh databases").
   - Add the backfill to `Synchronize`, after the `reportOnly` return and before the pull. That keeps
     `--report-only` nearly read-only and leaves `Probe` untouched.
   - Extend `internal/sync/sync_integration_test.go` (see how it's run in `~/vimango/CLAUDE.md`). Include the
     lost-link case: link a row locally while the server changes it, then sync twice; the server must end up linked.
   - Add the column to the pull SELECT, `localTask`/`localTaskByTid`, local INSERT/UPDATE (~1360/1385, UPDATE
     with COALESCE plus the keep-dirty rule), the client-changes SELECT (~518), push INSERT (1517), push UPDATE
     (1547, with COALESCE). `sameContent` leaves it out (see design decisions).
   - `saveConflictCopy` stays as it is, because its explicit list already leaves the column out.
   - Update `DATABASE_SCHEMA.md` (it documents the local SQLite only, and is already missing `uuid`/`server_modified`).
3. **vimango_ios:**
   - `LocalStore` migration `"v2"`: add the column and index. Today there is only `"v1"` (LocalStore.swift:50).
   - `SyncEngine` runs the backfill each sync, and the push UPDATE uses COALESCE.
   - `noteColumns` and `Note` get a stored `taskUUID`, replacing the computed property in `Models/Note.swift:28`.
   - `RemoteStore` pull (`DatabaseService.swift` ~120) and push (~145/164) include the column.
   - `applyPulled` sets the column on its own rows (COALESCE plus keep-dirty, see design decisions) and leaves it
     off conflict copies. The FTS trigger is `AFTER UPDATE OF title, body`, so neither this nor the backfill fires it.
   - `saveEdit` carries the column through rather than rewriting it, and `dirtyNotes()`/`DirtyNote` include it for
     the push. `insertNote`/`updateNote` in `DatabaseService.swift` send it (update with COALESCE).
   - `fetchNote(taskUUID:)` becomes a plain equality query.
   - `openTaskNote` creates the note with an empty body and the column set.
   - `linkTask` sets the column and moves the folder, with no body edit.
   - `NoteDetailView` and `NoteRowView` are unchanged, since they already read `note.taskUUID`.
4. **vimango_hybrid:**
   - `Repository.entry(forTaskUUID:)` (~313) becomes an equality query.
   - Note creation (`OrganizerViewModel` ~779, which inserts `request.body`; see also the comment at
     `Repository.swift` ~554) sets the column with an empty body.
   - `taskNoteEntry`'s title-match fallback (`entryID(forTitle:)`) now links the note it finds: it sets the column
     and bumps `modified` so the link syncs. Leaving that note unlinked was only to avoid a content write behind
     the editor. The editor never writes this column, so that reason no longer applies. Update the doc comment
     above `taskNoteEntry` to match.
   - Add an "Open Task" menu item and ex command that opens `taskchampdev://task/<uuid>` (taskwarrior_macos claims
     the scheme), replacing the body link. Follow hybrid's existing menu/ex-command pattern. It's disabled when
     the note has no link.
   - Optionally mark linked notes in the organizer.
   - Since hybrid opens `vimango.db` directly, it adds the column if missing, in case it starts before `vimango-sync` has.
5. **One-time migration**, a small Go command under `~/vimango/cmd/`, like `fix_uuid_sync`:
   - Sync every client first, so no linked note has an unsynced edit.
   - "Leading frontmatter" means the same thing it does in the clients. The note's first line is `---`, the block
     ends at the next `---` line, and CRLF is accepted. Mirror `splitFrontmatter` in `~/vimango/frontmatter.go`,
     which the Swift `FrontmatterParser.splitBlock` and hybrid's `frontmatterBlock` agree with.
   - For each live note whose leading frontmatter holds `taskwarrior: <uuid>`:
     - set `taskwarrior_uuid`;
     - remove that line, plus any leftover `project:` line Taskchamp wrote, and the whole block if it's then empty;
     - remove the `[Open task in Taskchamp](taskchampdev://task/<uuid>)` line;
     - bump `modified = now()`.
   - `splitFrontmatter` lives in the root `package main` and can't be imported from `cmd/`. Copy it into the
     command (it's a one-off), with a comment pointing at the original.
   - Don't leave a stray blank line where the "Open task" line was, or at the top once the block is gone.
   - Use `--dry-run` to print the diffs first.
   - Every client then pulls the cleaned rows on its next sync. There are probably only a handful.
6. **Docs:**
   - The "Taskchamp task notes" sections in vimango_ios and vimango_hybrid `CLAUDE.md`.
   - The Taskchamp `CLAUDE.md` bullet that says VimNotes "finds the note by a `taskwarrior: <uuid>` frontmatter line".
   - The doc comment in `taskwarrior_macos/Sources/TaskwarriorCore/VimangoLink.swift`.
   - The vimango_ios follow-up about the checkbox button in `NoteRowView` still applies, but it now reads the column.

## Risks / open points

- **Clients updated at different times.** Old builds ignore the column (explicit column lists). A note an old
  build creates for a task won't be found by uuid, so a new client would create a second one. Update all three
  before running the migration.
- **The Go terminal client loses all sign of the link.** It never had a working action for it.

## Verification

- Go (done for step 2): integration tests `TestLinkRoundTrips`, `TestPushNeverClearsLink`,
  `TestBackfillFillsLinkWithoutPushing`, `TestPullKeepsLocalLink` and `TestLinkOnlyDifferenceIsNotAConflict`.
  Each was checked to fail when its fix is removed. Check the migration
  command's `--dry-run` against a restored copy of the dump.
- iOS: `/verify`, with `TaskNoteLinkTests` rewritten around the column, plus a migration test (v1 database becomes v2).
- End to end:
  - In Taskchamp, tap a task's note button. VimNotes should create a note with an empty body and the column set.
  - Sync, then open the same task from taskwarrior_macos. Hybrid should find that note by uuid.
  - In VimNotes, use "Create task" on a note. The task should be created, and the note's row should show the link marker.
  - Edit the body freely; the link should remain.
