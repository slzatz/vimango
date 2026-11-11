# Updating a Mac to the November 2026 vimango

For a Mac that already runs vimango and vimango_hybrid and has not been
updated since before 2026-11-10. (A Mac with nothing on it: start with
[MACOS_SETUP.md](../MACOS_SETUP.md) instead.)

Three things changed together, and all three have to be brought up on each
machine:

| What | Where | Why it needs a step |
|---|---|---|
| vimango | `~/vimango`, branch `dev` | the code |
| libvim | `~/libvim`, the **slzatz/libvim** fork, branch `main` | `libvim.a` is built per machine and is not in git; the new vimango does not link against an old one |
| the local database | `~/vimango/vimango.db` | each machine has its own file; sync carries notes, not the file, so each machine converts its own |
| vimango_hybrid | `~/vimango_hybrid`, branch `dev` | it searches the new index, and bundles a `vimango-sync` built from `~/vimango` |

Nothing changes on the PostgreSQL server or in the iOS app. The Macs can be
done one at a time, in any order, days apart.

Allow about fifteen minutes, most of it compiling. Run every command from the
directory shown; vimango finds `config.json` and the database relative to
where it is started.

**Order matters in two places:** vimango before vimango_hybrid (hybrid's build
compiles `vimango-sync` out of `~/vimango`), and both on the same day (an old
hybrid searches the old index file, which the new vimango no longer updates).

---

## 0. Before you start

**Quit everything that has the database open**: vimango_hybrid (Cmd-Q, not
just closing the window), any vimango in a terminal, any `sqlite3` shell on
`vimango.db`. The database conversion in step 5 needs to be alone with the
file.

```bash
pgrep -lf 'vimango|VimangoHybrid'     # should print nothing
```

See where this machine is:

```bash
cd ~/vimango
git status -sb | head -5              # branch, and anything modified locally
git log --oneline -1
sqlite3 vimango.db "PRAGMA journal_mode;"   # delete = not converted yet; wal = already done
```

If `git status` lists modified tracked files (`vimango.vim`, a style file),
decide now: `git stash` keeps them aside, `git checkout -- <file>` drops them.
A pull stops on a file changed on both sides.

**Back up the database.** It is the one thing here that is not reproducible
from GitHub:

```bash
cd ~/vimango
sqlite3 vimango.db ".backup vimango-before-update.db.bak"
ls -la vimango-before-update.db.bak   # roughly the size of vimango.db
```

(`*.bak` is gitignored. Use `.backup`, not `cp` — after step 5 `cp` is no
longer a safe way to copy this file; see "Afterwards".)

---

## 1. Pull vimango

The changes are on `dev`. `main` was fast-forwarded to it on 2026-11-10 (at
`0556725`), so it has everything this guide was first written for; what came
later that day (libvim writing no files, the organizer's `:e <id>`) is on
`dev` until the next merge. The check below says whether `main` has the
first part.

```bash
cd ~/vimango
git fetch origin
git merge-base --is-ancestor 8d1e246 origin/main && echo "main has it" || echo "use dev"
```

Then, for `dev`:

```bash
git switch dev
git pull --ff-only
git log --oneline -1                  # 8d1e246 or later
```

`--ff-only` refuses rather than merges if this machine has local commits; if
it refuses, look at `git log origin/dev..dev` before doing anything else.

---

## 2. Remove the Go workspace file

```bash
cd ~/vimango
rm -f go.work go.work.sum
go env GOWORK                         # must print an empty line
```

Why: vimango uses a fork of the glamour markdown library. It used to come from
a checkout at `~/glamour` through a `go.work` file; now `go.mod` pins an exact
commit and `go build` downloads it. A leftover `go.work` wins over that pin,
silently, and builds against whatever `~/glamour` happens to hold.

Check which copy a build will use:

```bash
go list -m -f '{{.Dir}}' github.com/charmbracelet/glamour
# want a path under ~/go/pkg/mod/github.com/slzatz/glamour@...  — not ~/glamour
```

`~/glamour` itself is no longer needed by vimango.

---

## 3. Build libvim.a

```bash
cd ~/vimango
./scripts/build-libvim.sh --clean
```

`--clean` rebuilds every object. Use it for this update on every Mac: one
earlier fork change left stale objects behind on incremental builds, and the
result links, mostly works, and crashes on `zg`. A clean build takes a couple
of minutes and rules that out. (Later updates don't need it.)

The script looks for the fork at `~/libvim`, clones it if it is not there,
pulls `main`, builds, and copies `libvim.a` and the spell dictionary into
`~/vimango`. The last lines should be:

```
Done. /Users/<you>/vimango/libvim.a is libvim d93698bc8 fix(mappings): ...
Now build vimango:
```

The commit must be **`d93698bc8` or later** (`31301666c` is enough for a
vimango at `108a96e` or earlier, `77fc72a6d` for one at `0556725` or earlier).
To check afterwards:

```bash
git -C ~/libvim merge-base --is-ancestor d93698bc8 HEAD && echo "libvim is new enough"
```

The script refuses to build from a checkout that would give the wrong library,
and says what to do. What each refusal means:

| It says | What happened | Do |
|---|---|---|
| `is not the slzatz/libvim fork (origin: …onivim…)` | `~/libvim` is a clone of upstream, from the old setup instructions | `mv ~/libvim ~/libvim.upstream`, run the script again; it clones the fork |
| `The fork's master is now main; switching …` | not an error — the fork's branch was renamed, and the script moves the checkout across | nothing |
| `master has commits that are not on origin/main` | local commits on this machine's checkout | look at them: `git -C ~/libvim log origin/main..master`; then `git -C ~/libvim switch main` |
| `is on branch 'spelling_restore', not main` (or another branch) | the checkout was left on a work branch, since merged | `git -C ~/libvim fetch origin && git -C ~/libvim switch main` |
| `has uncommitted changes to tracked files` | edits in `~/libvim` | `git -C ~/libvim stash`, or commit them |
| `Homebrew not found` | — | install from https://brew.sh |

If the compile itself fails, see "Building by hand" in the fork's `CLAUDE.md`
(`~/libvim/CLAUDE.md`) for the configure flags current clang needs; the fork's
`build-macos.sh` already passes them.

---

## 4. Build vimango

```bash
cd ~/vimango
CGO_ENABLED=1 go build --tags=fts5
ls -la vimango                        # timestamp: just now
```

`ld: warning: ignoring duplicate libraries: '-lm'` is printed every time and
means nothing. The first build also downloads the glamour fork, so it needs
the network.

If it stops with `undefined:` and a name starting `vim`, `libvim.a` is older
than this vimango needs — step 3 did not finish, or built from the wrong
checkout:

| Error | `libvim.a` is older than |
|---|---|
| `undefined: vimSetMappingsEnabled` | fork `ed0f2e241` |
| `undefined: vimSpellGetErrors` | the fork's spell API |
| `undefined: vimSearchGetHighlightEnabled` | fork `a036ecafe` |
| `undefined: vimSetFileWritesEnabled` (or `_vimSetFileWritesEnabled` from the linker) | fork `31301666c` |
| `undefined: vimInputPending` | fork `d93698bc8` |

A `libvim.a` between `a036ecafe` and `77fc72a6d` links without complaint and
is still too old (vim's errors show, its reports — "5 lines yanked" — do
not). That is why step 3 checks the commit rather than trusting the build.

`undefined: thisBinaryRequiresBuildTagFts5` means `--tags=fts5` was left off.

`heic_worker` (HEIC image decoding) did not change. If `ls ~/vimango/heic_worker`
shows it, leave it.

Optional, about ten seconds — the test suite, which exercises the new
`libvim.a`:

```bash
CGO_ENABLED=1 go test --tags=fts5 ./...
```

Every line should start `ok` (or say `no test files`).

---

## 5. Convert the database

```bash
cd ~/vimango
./vimango --migrate
```

On a machine that has not been converted it prints two lines:

```
vimango.db: journal mode: delete -> wal (write-ahead log; readers and the writer no longer block each other)
vimango.db: search index: built task_fts from 5907 notes, with triggers that keep it current (fts5_vimango.db is no longer used)
```

(the note count is this machine's). Run it again:

```
vimango.db: already up to date
```

What it did:

- **Write-ahead logging.** A sync and a preview render no longer wait on each
  other. From now on two more files sit beside the open database,
  `vimango.db-wal` and `vimango.db-shm`. They are part of it.
- **The search index moved into `vimango.db`**, as the table `task_fts`, kept
  current by triggers. The separate `fts5_vimango.db` is no longer read or
  written by anything.

Check it:

```bash
sqlite3 vimango.db "PRAGMA journal_mode;
  SELECT count(*) FROM task;
  SELECT count(*) FROM task_fts;
  SELECT count(*) FROM sqlite_master WHERE type='trigger' AND name LIKE 'task_fts_%';
  INSERT INTO task_fts(task_fts) VALUES ('integrity-check');"
```

Want: `wal`, the same number twice, `8`, and no error from the last line.

If `--migrate` prints `Error:` instead, another program still has the
database open (step 0). Nothing is left half-done; quit it and run
`--migrate` again.

This step is optional in one sense: any start of the new vimango or
`vimango-sync` does the same conversion without saying so. Running it by hand
shows that it happened.

---

## 6. Update vimango_hybrid

```bash
cd ~/vimango_hybrid
git status -sb | head -3
git switch dev
git pull --ff-only
git log --oneline -1                  # 20c8176 or later
./scripts/run.sh
```

`run.sh` builds the app, builds `vimango-sync` from `~/vimango` into the app
bundle (which is why vimango was pulled first), signs it and launches it. You
should see `→ swift build`, `→ go build vimango-sync`, `→ codesign`,
`→ launching`.

---

## 7. Check that it worked

In vimango_hybrid:

- **Sync** (it will bring this machine up to date with the others; the first
  one after a long gap can be large). It should finish without an error.
- **Search** for a word you know is in a note. Then change a note's *title*
  to include a made-up word, and search for that word without syncing first:
  it should be found. (The old index only learned of a note at sync.)
- **Open a note in the editor** and try:
  - `/` and a word: every match is highlighted; `:noh` clears them.
  - `5yy`: the message line says `5 lines yanked`.
  - `:r !date`: `E145: Shell commands and some functionality not allowed`,
    and nothing is inserted.
  - `:spell`, then `z=` on a misspelled word: a numbered list.
  - Hold a key down at the bottom of a long note: no lag.

In a terminal, `cd ~/vimango && ./vimango` should start as before.

**If the sync says `password authentication failed for user … (28P01)`** and
the password in `~/vimango/config.json` is right, an environment variable is
overriding it: `VIMANGO_PG_PASSWORD`, when set, wins over the file in vimango,
`vimango-sync` and hybrid alike. `run.sh` starts hybrid from a terminal, and
hybrid started that way carries that terminal's variables — a stale line in a
shell profile that the Dock-launched app never saw. (This happened on the
first Mac updated: an old password in `~/.secrets`.) Builds from `0c7acc5` on
end the error with a note naming the variable. To find it:

```bash
echo "${VIMANGO_PG_PASSWORD:+set}"        # in the terminal run.sh was started from
grep -l VIMANGO_PG_PASSWORD ~/.zshenv ~/.zprofile ~/.zshrc ~/.zlogin ~/.bash_profile ~/.bashrc ~/.profile ~/.secrets 2>/dev/null
```

Delete the line, open a new terminal, quit hybrid and run `./scripts/run.sh`
again.

---

## Afterwards

**Copying or backing up the database.** Recent changes now live in
`vimango.db-wal` until SQLite folds them into the main file, so `cp
vimango.db somewhere` can produce a copy that is missing them. Always:

```bash
sqlite3 vimango.db ".backup copy.db"
```

**Things that can be deleted**, once you are satisfied (nothing reads them):

```bash
cd ~/vimango
ls fts5_vimango.db* webview_worker .venv go.work* 2>/dev/null
```

- `fts5_vimango.db` — the old search index
- `webview_worker`, `.venv` — helpers for features removed from the TUI
- `vimango-before-update.db.bak` — the backup from step 0, after a few days
- in `config.json`, the `"fts_db"` and `"claude"` entries are ignored; they
  can stay or go
- `~/glamour` and `~/libvim.upstream`, if nothing else of yours uses them

**Going back**, if something is badly wrong: check out the previous commits
in `~/vimango` and `~/vimango_hybrid` and rebuild both. The converted database
should not have to be undone for that: write-ahead logging is something any
SQLite program handles, and the extra index table is one older builds never
look at. (That is reasoning, not something that was tried.) The backup from
step 0 is there for the case where the database itself is in doubt: with
everything quit, `rm vimango.db-wal vimango.db-shm`, then
`cp vimango-before-update.db.bak vimango.db`.

---

## The short version

For the next Mac, once you have done one:

```bash
# quit vimango_hybrid and any vimango first
cd ~/vimango
sqlite3 vimango.db ".backup vimango-before-update.db.bak"
git switch dev && git pull --ff-only
rm -f go.work go.work.sum
./scripts/build-libvim.sh --clean          # must end: libvim d93698bc8 or later
CGO_ENABLED=1 go build --tags=fts5
./vimango --migrate                        # two lines; a second run: already up to date
cd ~/vimango_hybrid
git switch dev && git pull --ff-only
./scripts/run.sh
```
