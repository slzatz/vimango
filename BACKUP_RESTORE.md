# Backing up and restoring the sync server

Every client — the TUI, vimango_hybrid (through `vimango-sync`) and the iOS
app — syncs against one Postgres database. These commands run from a Mac over
the same TLS connection the clients use, so no SSH access to the server is
needed.

Connection values are in the `postgres` section of `config.json`. This repo is
public, so they are placeholders here: `<host>`, `<db user>` and the database
name (`vimango`).

## Setup (once)

```bash
brew install libpq    # pg_dump / pg_restore / psql; must be >= the server's major version (18)
```

Homebrew does not put libpq on `PATH`. Set up the shell for each session:

```bash
export PATH="/opt/homebrew/opt/libpq/bin:$PATH"
export PGHOST=<host> PGPORT=5432 PGDATABASE=vimango PGUSER=<db user>
export PGSSLMODE=verify-ca PGSSLROOTCERT="$HOME/vimango_ios/VimNotes/VimNotes/ca.crt"
```

Each command then prompts for the database password. To skip the prompt, add
it to `~/.pgpass` with `chmod 600`.

## Back up

```bash
mkdir -p ~/backups
pg_dump -Fc -f ~/backups/vimango-$(date +%Y%m%d-%H%M).dump
```

The result is about 2 MB, compressed from about 20 MB. Check that the dump is
complete by counting each table's rows:

```bash
F=~/backups/vimango-YYYYMMDD-HHMM.dump
for t in task context folder keyword task_keyword; do
  printf '%s: ' $t
  pg_restore -a -t $t -f - $F | awk '/^COPY /{on=1;next} /^\\\.$/{on=0} on' | wc -l
done
```

`task` includes tombstones (`deleted = true`), so it counts more rows than any
client shows.

Backups so far:

| File | Taken | task | context | folder | keyword | task_keyword |
|---|---|---|---|---|---|---|
| `vimango-20261004-1825.dump` | 2026-10-04, before the iOS app's first two-way sync | 5,935 | 32 | 18 | 139 | 852 |

## Restore

A restore **replaces** the five vimango tables with the dump's contents.
Everything written to the server after the dump was taken is lost, so read
"After a restore" before you start.

**1. Back up the current state first**, even a broken one. It costs a few
seconds and makes the restore reversible.

```bash
pg_dump -Fc -f ~/backups/vimango-before-restore-$(date +%Y%m%d-%H%M).dump
```

**2. Restore only the vimango tables, as one transaction.**

```bash
F=~/backups/vimango-YYYYMMDD-HHMM.dump
pg_restore --list $F | grep -vE ' (EXTENSION|SCHEMA) ' > /tmp/vimango-restore.list
pg_restore --clean --if-exists --no-owner --single-transaction \
  -L /tmp/vimango-restore.list -d vimango $F
```

Why it is filtered rather than a plain `pg_restore --clean`:

- **The dump holds entries the database user does not own.** These are the
  `uuid-ossp` extension and `ALTER SCHEMA public OWNER TO postgres`, and
  dropping or re-owning them needs a superuser.
- **Under `--single-transaction` that error aborts everything.** The database
  would be left untouched, but nothing would be restored.
- **The filtered list keeps only the needed objects:** DROP/CREATE of the five
  tables and four `tid` sequences, their data, constraints and sequence
  values.
- **The extension stays in place.** Leaving it out is safe because nothing
  drops it.
- **A failure changes nothing.** Any error rolls the whole restore back.

This was checked by rendering the SQL offline (`pg_restore … -f -`). As of
2026-10-04 it has not been run against the live server.

**3. Move the `tid` sequences past anything a client might still hold.**

The restore sets each sequence back to its value at dump time. A desktop that
synced after the dump holds tids issued since then, and the server would now
hand those numbers to *different* new notes. The desktop's push is
`UPDATE task … WHERE tid=$1` with no version check, so it would then overwrite
the wrong note. Jump the sequences well clear:

```bash
psql <<'SQL'
SELECT setval('task_tid_seq',    (SELECT max(tid) FROM task)    + 100000);
SELECT setval('context_tid_seq', (SELECT max(tid) FROM context) + 1000);
SELECT setval('folder_tid_seq',  (SELECT max(tid) FROM folder)  + 1000);
SELECT setval('keyword_tid_seq', (SELECT max(tid) FROM keyword) + 1000);
SQL
```

**4. Spot-check:** `psql -c 'SELECT count(*) FROM task'` should match the
dump's `task` row count.

## After a restore: the clients

Each client keeps its own copy and its own sync watermark. A restore rolls the
server back but leaves every client where it was.

- **iOS:** Settings → Reset Local Data. This erases the phone's copy and
  re-downloads it. Edits the phone has not synced are lost, and the dialog
  says how many there are.
  - Even without a reset, the iOS push only updates a row whose `modified`
    still equals the version it last saw. Its stale edits are therefore
    refused rather than written over restored notes.
- **TUI / vimango_hybrid:** there is **no tested rebuild procedure yet**.
  - A desktop that has not synced since the dump was taken is unaffected.
  - A desktop that has synced since then has two problems:
    1. Its watermark is newer than the restored rows, so it will not pull them.
    2. Notes it created after the dump carry tids the server no longer has.
       Their pushes match no row and are silently dropped; step 3 is what
       keeps them from hitting other notes instead.
  - `./vimango --init` is not a safe rebuild path. It refuses to run while
    `config.json` exists, and its schema has the NULL `*_tid` problem in
    `SYNC_ISSUES.md` §7, which breaks pushing new notes.
  - Until there is a procedure, stop using such a desktop and work out its
    rebuild deliberately.
    - Setting its watermark back makes the next sync re-pull everything:
      `UPDATE sync SET timestamp='1970-01-01 00:00:00' WHERE machine='server'`
      in its `vimango.db`. Do not delete the row, because `sync.go:1150` fails
      on a missing row.
    - Any notes it alone holds would need to be copied out first.
