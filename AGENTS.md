# Repository Guidelines

## Project Structure & Module Organization
- Root contains the main app (`main.go`, core files) and Go module (`go.mod`, Go 1.24).
- `cmd/` holds helper CLIs (e.g., `cmd/create_dbs`, `cmd/create_sqlite_db`).
- `vim/` contains editor integration; `extension/`, `terminal/`, `auth/` host platform and service code.
- Generated artifacts (e.g., `vimango`, `*.db`, cache) are ignored via `.gitignore`.

## Build, Test, and Development Commands
- Build (pure Go, default SQLite/FTS5): `CGO_ENABLED=0 go build --tags=fts5 -o vimango`.
- Build (with CGO: libvim, hunspell, sqlite3): `CGO_ENABLED=1 go build --tags="fts5,cgo" -o vimango`.
- Cross-compile Windows: `GOOS=windows GOARCH=amd64 go build --tags=fts5`.
- Quick build script: `./build.sh` (uses `--tags=fts5`).
- Run locally: `go run . [--go-vim|--go-sqlite|--cgo-sqlite]` or `./vimango`.
- Format: `go fmt ./...`; Vet: `go vet ./...`.

## Coding Style & Naming Conventions
- Follow standard Go style (`gofmt`); use tabs (default Go formatting).
- Package/file names: lowercase with underscores if needed (e.g., `editor_normal.go`).
- Exported identifiers use CamelCase; unexported use lowerCamelCase.
- Keep files focused; prefer small packages over large monoliths within the existing layout.

## Testing Guidelines
- Framework: Go `testing` standard library; place tests as `*_test.go` next to sources.
- Run all tests: `go test ./...`; with coverage: `go test -cover ./...`.
- Name tests `TestXxx`, benchmarks `BenchmarkXxx`; table-driven tests encouraged.
- At present there are few/no tests—add targeted tests for new logic and regressions.

## Commit & Pull Request Guidelines
- Commits: concise, imperative subject (max ~72 chars), descriptive body when needed.
- Examples: `Add :pdf-goldmark command`, `Fix CGO buffer switch column reset`.
- PRs: include problem statement, approach, risk/impact, and manual test steps; attach screenshots if UI/terminal output changes.
- Link related issues; keep PRs focused and minimally scoped.

## Security & Configuration Tips
- Do not commit secrets; `*.json` is ignored (e.g., `config.json`, tokens). Store local DB paths and credentials in `config.json` (ignored).
- To bootstrap local SQLite DBs, use the tools under `cmd/` (see `cmd/create_dbs`).

## CRITICAL: Use ripgrep, not grep

NEVER use grep for project-wide searches (slow, ignores .gitignore). ALWAYS use rg.

- `rg "pattern"` — search content
- `rg --files | rg "name"` — find files
- `rg -t python "def"` — language filters

## JSON

- Use `jq` for parsing and transformations.

## Agent Instructions

- Replace commands: grep→rg, find→rg --files/fd, ls -R→rg --files, cat|grep→rg pattern file
- Cap reads at 250 lines; prefer `rg -n -A 3 -B 3` for context
- Use `jq` for JSON instead of regex

## Research Notification Test

- Use `:researchtest` (alias `:rtest`) to simulate background research status updates without hitting the API.
- Messages appear every ~2 seconds so you can type simultaneously and confirm async notifications don’t disrupt editing.
- Quit the command normally; the test stops after the four canned messages finish.

## Research Debug Logging

- `ResearchManager.logDebug` writes timestamped entries to `vimango_research_debug/research.log`; it no longer queues UI notifications.
- The log directory is created on demand. Keep `vimango_research_debug/` in `.gitignore`; inspect the log locally when diagnosing research issues.
- Continue using `app.addNotification(...)` for user-facing updates (e.g., success/failure, API warnings). Use `logDebug` for internal instrumentation that should land in the file.

## Organizer Redraw Refactor (September 2025)

- Added a typed `RedrawScope` (`None`, `Partial`, `Full`) for organizer key handling so we can request finer-grained paints without touching the editor code (`organizer.go`, `organizer_process_key.go`).
- Refactored organizer rendering to expose row-level helpers (`drawRowAt`, `appendStandardRow`, `appendSearchRow`) so partial redraws can repaint only the current line and keep the divider clean (`organizer_display.go`).
- `App.MainLoop` now interprets `RedrawPartial` by calling `Organizer.drawRowAt` while still refreshing the status bar; full redraws continue to go through `refreshScreen` (`app.go`).
- Insert/normal mode libvim edits that stay on the same row return `RedrawPartial`; pressing <Enter> in insert mode or jumping to a different row still returns `RedrawFull`, which keeps previews, batch operations, and row navigation intact while eliminating flicker for single-line edits.

### Next Steps

1. Extend partial redraw handling to movements that change the active row (redraw outgoing + incoming rows, and keep the cursor indicator aligned).
2. Detect batch operations (mark/star/delete/archive on multiple rows) and trigger targeted row updates instead of full refreshes—may need a bitset-style redraw scope so preview/status refreshes can be requested independently.
