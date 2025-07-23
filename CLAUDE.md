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
- Platform-specific signal handling:
  - Unix: SIGWINCH signal detection for terminal resize events
  - Windows: Polling-based terminal resize detection (100ms intervals)
- Platform-agnostic terminal window size detection via rawmode package
- Conditional compilation for Unix-specific filesystem operations
- All platform-specific code isolated using build constraints

## Terminal Resize Handling
- **Unix/Linux**: Uses SIGWINCH signal for immediate resize detection
- **Windows**: Implements polling-based resize detection that checks terminal size every 100ms
- Both platforms call the same `signalHandler()` method to update screen layout
- Automatic screen redraw and layout adjustment on terminal resize

## Command System
The application features a comprehensive command registry system with full discoverability for both ex commands and normal mode commands:

### Help System
- `:help` - Show all available ex commands organized by category
- `:help normal` - Show all normal mode commands organized by category
- `:help <command>` - Show detailed help for specific ex command with usage and examples
- `:help <key>` - Show detailed help for specific normal mode command (e.g., `:help Ctrl-H`)
- `:help <category>` - Show all commands in a specific category (e.g., `:help Navigation`)
- `:h` - Short alias for help command

### Ex Command Organization
**Organizer Ex Commands (66+ commands in 8 categories):**
- **Navigation**: open, opencontext, openfolder, openkeyword
- **Data Management**: new, write, sync, bulkload, refresh
- **Search & Filter**: find, contexts, folders, keywords, recent, log
- **View Control**: sort, showall, image, webview, vertical resize
- **Entry Management**: e (edit), copy, deletekeywords, deletemarks
- **Container Management**: cc (context), ff (folder), kk (keyword)
- **Output & Export**: print, ha, printlist, save, savelog
- **System**: quit, which

**Editor Ex Commands (20+ commands in 5 categories):**
- **File Operations**: write, writeall, read, save
- **Editing**: syntax, number, fmt, run
- **Layout**: vertical resize, resize
- **Output**: ha, print, pdf
- **System**: quit, quitall

### Normal Mode Command Organization
**Editor Normal Mode Commands (17+ commands in 6 categories):**
- **Movement**: Ctrl-H (move left), Ctrl-L (move right)
- **Text Editing**: Ctrl-B (bold), Ctrl-I (italic), Ctrl-E (code), \<leader\>b (bold)
- **Preview**: \<leader\>m (markdown preview), \<leader\>w (web view)
- **Window Management**: \<C-w\>L, \<C-w\>J, \<C-w\>=, \<C-w\>_, \<C-w\>-, \<C-w\>+, \<C-w\>\>, \<C-w\>\<
- **Output Control**: Ctrl-J (scroll down), Ctrl-K (scroll up)
- **Utility**: \<leader\>y (next style), \<leader\>t (go template), \<leader\>sp (spell check), \<leader\>su (spell suggest)
- **System**: Ctrl-Z (switch vim implementation)

**Organizer Normal Mode Commands (10+ commands in 5 categories):**
- **Entry Actions**: m (mark), Ctrl-D (delete), Ctrl-A (star), Ctrl-X (archive)
- **Navigation**: Ctrl-J (scroll preview down), Ctrl-K (scroll preview up)
- **Information**: Ctrl-I (show entry info)
- **Mode Switching**: : (ex command mode), Ctrl-L (switch to editor)
- **Preview**: Ctrl-W (web view)

### Enhanced Error Messages
- Smart command suggestions for typos using fuzzy matching from both ex and normal command registries
- "Did you mean" suggestions when commands are not found
- Helpful guidance to use `:help` for ex commands or `:help normal` for normal mode commands

### Implementation Details
- **File**: `command_registry.go` - Core command registry system with metadata and key display helpers
- **Files**: `editor_normal.go`, `organizer_normal.go` - Normal mode command registration
- **Files**: `editor_cmd_line.go`, `organizer_cmd_line.go` - Enhanced help system integration
- **Backward Compatible**: All existing commands and key bindings work unchanged
- **Type Safe**: Uses Go generics for type-safe command function signatures
- **Self-Documenting**: Help text is co-located with command definitions for both ex and normal commands
- **Key Display**: Human-readable key representations in help (e.g., `\x08` displayed as `Ctrl-H`)
- **Extensible**: New commands require help metadata, ensuring documentation stays current
