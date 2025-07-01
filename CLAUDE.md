# GEMINI.md

This file provides guidance to GEMINI.md when working with code in this repository.

## Build Commands
- **Linux/Unix Pure Go**: `CGO_ENABLED=0 go build --tags=fts5` (no CGO dependencies)
- **Linux/Unix with CGO**: `CGO_ENABLED=1 go build --tags="fts5,cgo"` (includes libvim, hunspell, sqlite3)
- **Windows Cross-Compilation**: `GOOS=windows GOARCH=amd64 go build --tags=fts5` (pure Go only)
- Run: `go run main.go`
- Test: `go test ./...`
- Test single package: `go test ./path/to/package`
- Test single function: `go test -run TestFunctionName`

## Runtime Options
- `--go-vim`: Use pure Go vim implementation (default: CGO-based libvim)
- `--go-sqlite`: Use pure Go SQLite driver (modernc.org/sqlite) - default
- `--cgo-sqlite`: Use CGO SQLite driver (mattn/go-sqlite3) - only available in CGO builds
- Spell check: Available in CGO builds, shows graceful message in pure Go builds

## Project Structure
- Main application functionality in root directory
- Command-specific code in `cmd/` directory
- Database operations in db-related files

## Key Points
- The application is written in Go.
- The vimango application is a note taking application that stores notes and their titles in a local SQLite database.
- The application supports dual SQLite driver selection:
  - `modernc.org/sqlite` (Pure Go, default) - Works on all platforms
  - `mattn/go-sqlite3` (CGO-based) - Only available on Linux/Unix with CGO enabled
- The application uses vim editor functionality for editing notes and can switch between CGO-based libvim and a pure Go implementation.
- The application supports full-text search using the `fts5` extension of SQLite.
- The application uses a terminal-based user interface for interaction.
- Cross-platform compatibility: Windows builds automatically use pure Go implementations for all components.
- The application supports conditional spell checking:
  - `hunspell` (CGO-based) - Available on Linux/Unix with CGO enabled
  - Graceful degradation - Shows helpful messages when spell check unavailable

## Platform-Specific Behavior
- **Linux/Unix**: Supports vim, SQLite, and spell check options via build flags
- **Windows**: Automatically uses pure Go implementations (no CGO dependencies)
- **Cross-Platform**: Full Windows compatibility achieved through platform-specific signal handling and terminal operations

## Cross-Compilation Support
The application now supports full Windows cross-compilation from Linux/Unix systems:
- Platform-specific signal handling (SIGWINCH on Unix, no-op on Windows)
- Platform-agnostic terminal window size detection via rawmode package
- Conditional compilation for Unix-specific filesystem operations
- All platform-specific code isolated using build constraints
