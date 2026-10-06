# Sync issues (internal/sync/sync.go)

Found 2026-10-04 while porting the sync protocol to the iOS client
(`~/vimango_ios`, `Services/SyncEngine.swift`). Each one was checked against
the code; line numbers are as of `d334247`. None is fixed yet. The TUI and
vimango_hybrid both run this code — hybrid through the `vimango-sync` CLI — so
fixing it here fixes both.

Ordered roughly by how likely each is to lose data.

Two more found on review (2026-10-06), to be fixed together with 1-3:

- **The client watermark has the same race as issue 1.** `client` is set to
  `datetime('now')` at the end of the run, so a local write that lands
  mid-sync is skipped for good. Examples: a hybrid GRDB star or title change,
  or an `--editor` `:w`. Timestamps have one-second precision and the
  comparison is a strict `>`, so an edit in the same second the sync ends is
  lost too.
- **Issue 2 cannot be fixed alone.** A watermark that doesn't advance re-pulls
  the client's own pushes. Under the current rule those count as server
  edits and discard newer local ones. A single permanently failing row would
  also stop the watermark forever, so failures need to be tracked per row.

Issue 8 is the likely source of the dangling uuids in the data finding. When a
container push fails, its tasks are still pushed carrying its uuid.

## 1. Server watermark is taken at the end of the run

`sync.go:1222-1229` stores `SELECT now()` as the `server` watermark *after*
everything has been pulled and pushed. A row another client commits while the
sync is running has `modified` earlier than that `now()`, but it was not there
when the pull ran. The next sync asks for `modified > watermark` and skips it
for good, unless the row is edited again.

This matters more now that the iOS client writes to Postgres on its own
schedule: an iOS push that lands during a desktop sync is never pulled by that
desktop.

**Fix:** read the watermark before the pull, and pull with a small overlap
such as `localtimestamp - interval '2 minutes'`. The overlap covers
transactions that began before `now()` and commit after the pull has read past
them. Rows pulled twice are harmless, because the upsert is idempotent.

Two things have to change with it:

- **Use `localtimestamp`, not `now()`.** `task.modified` is
  `timestamp without time zone`, while `now()` is zone-aware. lib/pq hands it
  back as `…Z` and the iOS driver as `…+00`, and the offset is dropped when it
  is cast back to `timestamp`. That only works while the session zone is UTC,
  which it is today.
- **The conflict rule (issue 3) has to change at the same time.** Once the
  watermark comes before the push, the client's own pushes come back on the
  next pull. Under the current rule ("the server updated this tid in this
  run") those echoes would count as server edits and wipe the client's newer
  local edits. iOS avoids this by storing, per row, the server `modified` it
  last saw (`server_modified`), and ignoring a pulled row whose `modified`
  equals it.

## 2. Watermarks advance even when rows failed

Every step logs its errors and carries on: `syncEntriesToServer`,
`syncContainersToServer`, the delete loops and others return nothing.
`sync.go:1222-1240` then moves both watermarks forward regardless. A row whose
push or pull failed, from a network blip or a constraint error, is never
retried unless it is modified again.

**Fix:** collect errors, and only advance the watermarks when the run
succeeded. Re-pulling is idempotent, so a failed run simply repeats next time.
Pushing per row and stopping at the first failure also leaves the remaining
local rows dirty for the retry.

## 3. "Server wins" only covers edits from the same run; the losing edit is dropped

`sync.go:823-826` skips a client push if the same tid was pulled earlier in
*this* run, and logs `Server won:`. Two consequences:

- **The losing local edit is silently discarded.** It is logged, but nothing
  keeps the text.
- **Stale edits can still win.** A conflict counts only if the server change
  falls inside this run's pull window. `~/vimango_hybrid/TODO.md:46-75`
  already describes stale editor buffers reverting server changes.

**Fix (what iOS does):**

- Keep `server_modified` per row.
- Guard each push with `UPDATE … WHERE tid=$ AND modified=$server_modified`.
  If the guard rejects it, leave the row dirty; the next pull turns it into a
  conflict copy.
- When a dirty local row really was changed on the server, take the server's
  version and save the local one as a new note titled
  `"<title> (conflict YYYY-MM-DD)"`.

## 4. Keyword deletes pushed to the server don't bump `modified`

**Fixed 2026-10-06.**

`deleteKeywordFromBoth`, at `sync.go:1063`:

```go
s.PG.Exec("UPDATE keyword SET deleted=true WHERE tid=$1", c.tid)
```

Other clients only pull rows with `modified > watermark`, so they never see
the delete. Every other tombstone sets `modified=now()`.

**Fix:** `UPDATE keyword SET deleted=true, modified=now() WHERE tid=$1`.

## 5. Keywords added to a task that has never synced are orphaned

**Guarded 2026-10-06:** `addTaskKeywordByUUID` now refuses, with a message, when the task or keyword has no tid yet. Keying `task_keyword` on uuid is still open.

`entryTidFromId` (`dbfunc.go:21-25`) ignores its `Scan` error, so a task with
a NULL tid returns 0. `addTaskKeywordByUUID` (`dbfunc.go:690-708`) then
inserts `task_keyword (task_tid=0, …)`.

- **On push:** once the task gets a real tid, the push reads keywords by the
  new tid (`sync.go:899`). It finds nothing, so the association never reaches
  the server, and the `task_tid=0` row stays behind locally.
- **Keyword side:** a keyword that has never synced has a NULL tid too, so
  the keyword-tid lookup at `dbfunc.go:695` fails. The UI doesn't prevent
  either case.

The live database has no such rows today, but nothing prevents them.

**Fix:** key local `task_keyword` on the local task id (or a task uuid), or
refuse to add keywords until the task has synced.

## 6. Deletes never clean the FTS index

**Fixed 2026-10-06** for future deletes. The existing orphans (74 rows, plus one tid with a duplicate row) still need the one-off cleanup.

The local delete paths (`sync.go:907-962`) remove `task_keyword` and `task`
rows but never touch `fts5_vimango.db`. Orphaned FTS rows build up: about
5,923 FTS rows against 5,849 tasks as of 2026-10-04. FTS rows are also keyed by
tid, so a task has no FTS row until its first sync (`~/vimango_hybrid/HISTORY.md:267, 523-528`).

**Fix:** `DELETE FROM fts WHERE tid=?` alongside each local task delete, plus
a one-off cleanup of the existing orphans.

## 7. The `--init` schema breaks pushing new tasks

**Fixed 2026-10-06:** `DEFAULT 1` restored, NULL tids scan as 0 and resolve from the uuids, and `fetchAllChanges` fails on any scan error.

In `init.go`, the canonical schema gives `folder_tid` and `context_tid` no
DEFAULT. Production databases were created earlier and have `DEFAULT 1`.

On a freshly `--init`'d database:

1. Locally created tasks have NULL in those columns.
2. The client fetch (`sync.go:500`) scans them into `int` and ignores the
   error. `database/sql` stops at the first failing column, so `context_uuid`
   and `folder_uuid` are left empty as well.
3. The server insert then fails its foreign key.

**Fix:**

- Restore `DEFAULT 1`, or scan into `sql.NullInt64`.
- Check the `rows.Scan` errors throughout `fetchAllChanges`.

## 8. Containers match by tid only; title collisions fail

Pull (`sync.go:675-703`) and push (`706-741`) match contexts, folders and
keywords by tid alone, but titles are UNIQUE on both sides. Two clients that
both create a container with the same title before syncing get a constraint
error on one side. Because of issue 2 the error is not retried.

**Fix:** match by uuid first. On a title clash, adopt the server's uuid and
re-point local rows to it.

## Data finding (server, not code)

46 live tasks reference a `context_uuid` that has **no row at all** in
`context`, not even a deleted one:

| context_uuid | live notes | last modified |
|---|---|---|
| `e88ed6c2-1e45-4277-82f9-27158d91e5d1` | 43 | 2025-04-23 |
| `ab86c52a-f212-48cf-bd9a-d0893e5e689b` | 1 | 2026-08-14 |
| `c6eba010-7b60-47b8-a6ce-d9e0cef22256` | 1 | 2026-01-18 |
| `3eff18d3-59a3-4637-8a03-488b6cb16ee7` | 1 | 2026-08-14 |

Any query that inner-joins `context` hides these notes; the old iOS list did.
The two 2026-08-14 rows suggest something still writes uuids that were never
pushed, possibly a context created locally and referenced before it synced.
That is worth tracing alongside issue 8.

**Cleanup:**

```sql
UPDATE task SET context_uuid='00000000-0000-0000-0000-000000000001', modified=now()
WHERE context_uuid NOT IN (SELECT uuid FROM context WHERE uuid IS NOT NULL);
```

Back up with `pg_dump` first.
