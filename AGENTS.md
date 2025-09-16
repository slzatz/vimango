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
