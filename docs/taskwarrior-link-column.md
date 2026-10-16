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

- **Status (2026-10-16):** design approved in discussion; nothing implemented. Last related commits:
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
- **`sameContent`** (Go, sync.go:1216) and its iOS counterpart include the column, so a link-only change
  isn't treated as "no change".
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
- It makes it safe to upgrade a machine before or after the server migration.

**What each machine has to do** is rebuild and run the new vimango (and hybrid, on the Macs) and install the
new VimNotes build. Machines that are never upgraded stay harmless: an old build neither reads nor writes
the column. The exception is task notes created from such a machine, which don't happen on the Linux CLI
machines anyway. Upgrade the Macs and the phone before running step 5.

## Steps (each lands and is tested before the next)

1. **Back up the server** (`~/vimango/BACKUP_RESTORE.md`), then on Postgres:
   `ALTER TABLE task ADD COLUMN taskwarrior_uuid TEXT;` plus the partial index. Add it to
   `cmd/create_dbs/postgres_init4.sql` and `DATABASE_SCHEMA.md`.
2. **Go sync + desktop SQLite:**
   - Add the column and index in `Syncer.ensureSyncColumns()` (sync.go:846, which already adds
     `server_modified`). Both the CLI's sync and `cmd/vimango-sync` go through `internal/sync`. The CLI itself
     never reads the column.
   - Add the backfill at the start of a sync run, as described above.
   - Extend `internal/sync/sync_integration_test.go` (see how it's run in `~/vimango/CLAUDE.md`).
   - Add the column to the pull SELECT, local INSERT/UPDATE (~1360/1385), push INSERT (1517), push UPDATE
     (1547, with COALESCE) and `sameContent`.
   - `saveConflictCopy` stays as it is, because its explicit list already leaves the column out.
3. **vimango_ios:**
   - `LocalStore` migration `"v2"`: add the column and index. Today there is only `"v1"` (LocalStore.swift:50).
   - `SyncEngine` runs the backfill each sync, and the push UPDATE uses COALESCE.
   - `noteColumns` and `Note` get a stored `taskUUID`, replacing the computed property in `Models/Note.swift:28`.
   - `RemoteStore` pull (`DatabaseService.swift` ~120) and push (~145/164) include the column.
   - `applyPulled` sets the column on its own rows and leaves it off conflict copies.
   - `fetchNote(taskUUID:)` becomes a plain equality query.
   - `openTaskNote` creates the note with an empty body and the column set.
   - `linkTask` sets the column and moves the folder, with no body edit.
   - `NoteDetailView` and `NoteRowView` are unchanged, since they already read `note.taskUUID`.
4. **vimango_hybrid:**
   - `Repository.entry(forTaskUUID:)` (~313) becomes an equality query.
   - Note creation (`OrganizerViewModel` ~779, which inserts `request.body`; see also the comment at
     `Repository.swift` ~554) sets the column with an empty body.
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
- **The vimango MCP connector** (if it writes notes) is another Postgres client to check; it would only ignore the column.
- **The Go terminal client loses all sign of the link.** It never had a working action for it.

## Verification

- Go: unit tests for the sync's pull/push carrying the column and for `sameContent`. Check the migration
  command's `--dry-run` against a restored copy of the dump.
- iOS: `/verify`, with `TaskNoteLinkTests` rewritten around the column, plus a migration test (v1 database becomes v2).
- End to end:
  - In Taskchamp, tap a task's note button. VimNotes should create a note with an empty body and the column set.
  - Sync, then open the same task from taskwarrior_macos. Hybrid should find that note by uuid.
  - In VimNotes, use "Create task" on a note. The task should be created, and the note's row should show the link marker.
  - Edit the body freely; the link should remain.
