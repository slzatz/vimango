# CLAUDE.md

This file provides guidance to Claude when working with code in this repository.

## Build Commands
- **Linux/Unix with CGO**: `CGO_ENABLED=1 go build --tags="fts5,cgo"` (includes libvim, hunspell, sqlite3)
NOTE: When updating, fixing or adding to the code, the CGO build is the most comprehensive to ensure all features work as expected.
- **Linux/Unix Pure Go**: `CGO_ENABLED=0 go build --tags=fts5` (no CGO dependencies)
- **Windows Cross-Compilation**: `GOOS=windows GOARCH=amd64 go build --tags=fts5` (pure Go only)
- Run: `go run main.go`

### Tests
NOTE: Generally we have not been running tests but have tested key functionality manually.
- Test: `go test ./...`
- Test single package: `go test ./path/to/package`
- Test single function: `go test -run TestFunctionName`

## Runtime Options
- `--help`, `-h`: Display help message with all available options and exit
- `--go-vim`: Use pure Go vim implementation (default: CGO-based libvim)
- `--go-sqlite`: Use pure Go SQLite driver (modernc.org/sqlite) - default
- `--cgo-sqlite`: Use CGO SQLite driver (mattn/go-sqlite3) - only available in CGO builds
- Spell check: Available in CGO builds, shows graceful message in pure Go builds

## Help System
The application supports standard `--help` and `-h` flags to display comprehensive usage information:
- Shows all command-line options with descriptions
- Includes platform-specific behavior notes
- Displays build instructions and examples
- Reports runtime platform and architecture information
- Implementation: `help.go` with early detection in `main.go`

## Project Structure
- Main application functionality in root directory
- Command-specific code in `cmd/` directory
- Database operations in db-related files

## Key Points
- The application is written in Go.
- The vimango application is a note taking application that stores notes and their titles in a local SQLite database.
- The application operates in two main modes: an editor mode for editing notes and an organizer mode for managing and viewing notes. On the terminal screen the Organizer with the titles of notes is on the left and the Editor for editing notes is on the right.
- The application supports markdown rendering of notes to enhance the readability of the content.
- The ability to manage and display images is an important feature of the application.
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
The command system is loosely based on the vim command system and the existence of modes. The "super" modes of Organizer and Editor each have normal commands and Ex Commands.  For editing both of Organizer note titles and Editor notes, the use of libvim provides a wide range of vim commands including essentially all vim normal mode commands. The application features a comprehensive command registry system with full discoverability for both ex commands and non-vim normal mode commands:

### Help System
- `:help` - Show all available ex commands organized by category
- `:help normal` - Show all normal mode commands organized by category
- `:help <command>` - Show detailed help for specific ex command with usage and examples
- `:help <key>` - Show detailed help for specific normal mode command (e.g., `:help Ctrl-H`)
- `:help <category>` - Show all commands in a specific category (e.g., `:help Navigation`)
- `:h` - Short alias for help command

### Ex Command Organization
**Organizer Ex Commands (69+ commands in 9 categories):**
- **Navigation**: open, opencontext, openfolder, openkeyword
- **Data Management**: sync
- **Search & Filter**: find, contexts, folders, keywords, recent, log
- **View Control**: sort, showall, image, webview, closewebview, vertical resize
- **Entry Management**: new, e (edit), copy, set context, set folder, deletekeywords, deletemarks
- **Container Management**: cc (context), ff (folder), kk (keyword)
- **Research**: research, researchdebug (rd)
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

**Organizer Normal Mode Commands (11+ commands in 5 categories):**
- **Entry Actions**: m (mark), Ctrl-D (delete), Ctrl-A (star), Ctrl-X (archive)
- **Navigation**: Ctrl-J (scroll preview down), Ctrl-K (scroll preview up)
- **Information**: Ctrl-I (show entry info)
- **Mode Switching**: : (ex command mode), Ctrl-L (switch to editor)
- **Preview**: Ctrl-W (web view), Ctrl-Q (close web view)

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

## Deep Research System
The application includes an AI-powered deep research capability that leverages Claude API with combined web search and web fetch tools to generate comprehensive research reports with significantly enhanced depth and quality.

### Research Commands
- `:research` - Perform deep research on current entry content and save results as new note
- `:researchdebug` (`:rd`) - Same as research but includes debug information and API response analysis

### Enhanced Research Features
- **Dual-Tool Approach**: Combines web search (breadth) with web fetch (depth) for comprehensive coverage
  - **Web Search**: Discovers and evaluates sources (15 max uses)
  - **Web Fetch**: Accesses complete content from promising sources (8 max uses)
  - **Intelligent Workflow**: Search → evaluate → fetch → deep analysis
- **Asynchronous Processing**: Research runs in background without blocking the user interface
- **Advanced Quality Tiers**:
  - "Premium Deep Research" - Extensive search + full document analysis
  - "Comprehensive" - Thorough web research with document analysis
  - "Detailed" - Moderate web research
  - "Standard" - Basic research
  - "Limited" - Minimal research
- **Enhanced Metrics**: Separate tracking of web searches vs web fetches performed
- **Automatic Note Creation**: Results are automatically saved as new vimango entries
- **Usage Statistics**: Tracks and reports token usage, search count, fetch count, duration, and quality ratings
- **Citations**: Web fetch enables accurate citations from complete documents
- **Error Recovery**: Comprehensive panic recovery and error handling with web fetch specific diagnostics
- **Debug Mode**: Optional detailed logging for troubleshooting and analysis with tool usage summaries

### Configuration
Research functionality requires Claude API key configuration in `config.json`:
```json
{
  "claude": {
    "api_key": "your-claude-api-key"
  }
}
```

## Markdown Rendering Configuration
The application uses the glamour library to render markdown with custom styling for previews and note display.

### Glamour Style Configuration
Style files are configured in `config.json`:
```json
{
  "glamour": {
    "style": "darkslz.json"
  }
}
```

### Style File Requirements
- **Required Files**: At least one of the following must exist:
  - Configured style file (specified in config.json)
  - `default.json` (fallback style)
- **File Location**: Style files must be in the application's working directory
- **File Format**: JSON format compatible with glamour's style specification
- **Startup Validation**: Application validates style file existence at startup and exits gracefully if neither file is found

### Style File Fallback Logic
1. Try configured style from `config.json` (`glamour.style`)
2. Fall back to `default.json` if configured style is missing
3. Exit with clear error message if neither file exists

### Error Handling
If style files are missing, the application displays:
```
Error: glamour style files not found:
  Configured style: <filename>
  Fallback style: default.json
Please ensure at least one of these files exists
```

#### Web Fetch Requirements
For enhanced deep research with web fetch capabilities:
- **API Key Permissions**: Ensure your Claude API key has web search and web fetch permissions enabled
- **Model Compatibility**: Uses Claude Sonnet 4 (`claude-sonnet-4-20250514`) for web fetch support
- **Beta Access**: Web fetch requires beta feature access with header `anthropic-beta: web-fetch-2025-09-10`
- **Content Limits**: Configured with 100,000 token limit for large document processing
- **Citations**: Automatically enabled for fetched content to provide accurate source attribution

## Terminal Graphics Protocol Support

The application uses the kitty graphics protocol for inline image display. This protocol is supported by multiple terminal emulators:

### Supported Terminals
- **Kitty**: Full support including images, Unicode placeholders, and text sizing (OSC 66)
- **Ghostty**: Supports images and Unicode placeholders; text sizing not yet supported (falls back gracefully)
- **Other terminals**: Any terminal implementing the kitty graphics protocol should work for images

### Feature Detection
The application automatically detects terminal capabilities:
- `IsTermKitty()` in `term_misc.go` - Detects kitty graphics protocol support (kitty, ghostty, etc.)
- `isActualKittyTerminal()` in `kitty_capabilities.go` - Distinguishes actual kitty from other compatible terminals
- Text sizing (OSC 66) is only enabled for actual kitty terminal; other terminals get standard ANSI heading styles

### Text Sizing Protocol
The kitty text sizing protocol (OSC 66, added in kitty 0.40.0) allows scaled headings in markdown rendering:
- Configured via glamour style files (e.g., `darkslz.json`) using `kitty_scale`, `kitty_numerator`, `kitty_denominator`, `kitty_valign` properties
- Automatically disabled for terminals that don't support it (like ghostty)
- Headings fall back to standard ANSI styling (bold, colors) when text sizing is unavailable

### Environment Variable Override
- `VIMANGO_DISABLE_KITTY_TEXT_SIZING` - Escape hatch to disable text sizing if it causes issues

### Image Commands
- `:toggleimages` (`:ti`) - Toggle inline image display on/off
- `:imagereset` (`:ir`) - Clear terminal image cache and rerender current note
- `:imagescale [+|-|N]` - Scale images up, down, or to specific column width
- `:clearcache` (`:clc`) - Clear disk image cache

### Implementation Files
- `kitty_capabilities.go` - Terminal detection and capability flags
- `term_misc.go` - `IsTermKitty()` function for protocol support detection
- `kitty_placeholders.go` - Kitty graphics protocol commands
- `organizer_display.go` - Image rendering and placeholder handling

## Image Management
- The markdown notes may contain image placeholders of the form ![text](imagepath) where typically that image path is the id of an image file in google drive.
- Google drive image files are represented, for example, as ![2072 in lily_2025](gdrive:19_FwuxjvgIwxn-b4Ia75DrXRGLZeB2fe) where the string after gdrive: is the file id in google drive.
- An important feature of the application is the caching of images since downloading from google drive introduces some delays.
- There are two levels of image caching:
  - **Disk cache** (`./image_cache/`): Stores images as base64 encoded PNG files. Persistent across sessions and terminal-agnostic.
  - **Terminal memory cache**: The kitty graphics protocol caches images in the terminal emulator for fast display. Reset on each application launch.
- The disk cache works with any kitty-graphics-compatible terminal (kitty, ghostty, etc.)

## HEIC Image Support
The application supports HEIC/HEIF image format (commonly used by Apple devices for high-efficiency image storage) when built with CGO.

### System Dependencies
HEIC support requires the following system libraries to be installed at runtime:
- **libheif** >= v1.16.2 - Core HEIF/HEIC container format library
- **libde265** - HEVC/H.265 decoder (required by libheif for HEIC decoding)
- **aom** - AV1 codec support (note: package name is `aom` on Arch Linux, `libaom-dev` on Debian/Ubuntu)

**Arch Linux:**
```bash
sudo pacman -S libheif libde265 aom
```

**Debian/Ubuntu:**
```bash
# May need strukturag PPA for recent libheif versions
sudo add-apt-repository ppa:strukturag/libheif
sudo apt update
sudo apt install libheif-dev libde265-dev libaom-dev
```

### Build Requirements
HEIC support uses the `go-libheif` package which requires a separate worker binary for safe image decoding. The worker binary architecture isolates potential libheif crashes from the main application.

**Build steps:**

1. Build the HEIC worker binary first:
   ```bash
   cd cmd/heic_worker && go build -o ../../heic_worker
   ```

2. Build main application with CGO:
   ```bash
   CGO_ENABLED=1 go build --tags="fts5,cgo"
   ```

**Important:** The `heic_worker` binary must be placed alongside the main `vimango` executable (same directory).

### Graceful Degradation
HEIC support degrades gracefully in all failure scenarios - the main application will never crash due to missing HEIC dependencies:

- **Missing system libraries** (libheif, libde265, aom): Worker binary fails to start, HEIC reported as unavailable
- **Missing worker binary**: HEIC reported as unavailable with descriptive error
- **Pure Go builds** (CGO_ENABLED=0): HEIC images skipped with informative message
- **Corrupted HEIC file**: Worker process handles the crash, main app receives error message

When HEIC is unavailable, attempting to display a HEIC image will show an error message rather than crashing.

### Behavior
- **CGO builds with dependencies**: HEIC images are automatically detected by magic bytes and converted to PNG for display/caching
- **First access**: HEIC is decoded and converted to PNG, then cached as base64 in `./image_cache/`
- **Subsequent access**: Cached PNG is used directly - no HEIC decoding needed
- **Pure Go builds**: HEIC images are gracefully skipped with an informative message

### HEIC Detection
HEIC files are detected by magic bytes (ftyp box with heic/heix/hevc/hevx/mif1/msf1 brand identifiers), not by file extension, ensuring reliable detection for Google Drive images where extension may not be preserved.

### Why a Worker Binary?
The `go-libheif` package uses a subprocess architecture for safety. The libheif C library can crash (segfault) when processing malformed images. By running decoding in a separate worker process:
- Crashes in libheif only terminate the worker, not the main application
- The main app receives an error and can continue operating
- This is especially important for a TUI application where crashes would corrupt the terminal state

### Implementation Files
- `heic.go` - Shared interface and HEIC magic bytes detection
- `heic_cgo.go` - CGO-based HEIC decoding using go-libheif, with stdout/stderr suppression for TUI compatibility
- `heic_nocgo.go` - Stub implementation for non-CGO builds
- `cmd/heic_worker/main.go` - Worker binary for safe subprocess decoding

## Local-Only Operation (UUID-Based Containers)

The application supports full local-only operation without requiring PostgreSQL synchronization. This is achieved through UUID-based container identification.

### How It Works
- **Containers** (contexts, folders, keywords) are identified by UUID rather than PostgreSQL-assigned `tid` values
- **Tasks** reference containers via `context_uuid`, `folder_uuid`, and `keyword_uuid` foreign keys
- **Local creation**: Containers created locally generate their own UUIDs immediately
- **Sync compatibility**: The `tid` column is preserved for backward compatibility with PostgreSQL sync

### Database Schema
Containers include both `tid` (for sync) and `uuid` (for local operations):
```sql
CREATE TABLE context (
    id INTEGER PRIMARY KEY,
    tid INTEGER,           -- PostgreSQL sync ID (may be NULL locally)
    uuid TEXT NOT NULL UNIQUE,  -- Primary identifier for local operations
    title TEXT NOT NULL,
    ...
);
```

Tasks reference containers by UUID:
```sql
CREATE TABLE task (
    ...
    context_uuid TEXT DEFAULT '00000000-0000-0000-0000-000000000001',
    folder_uuid TEXT DEFAULT '00000000-0000-0000-0000-000000000002',
    FOREIGN KEY(context_uuid) REFERENCES context (uuid),
    FOREIGN KEY(folder_uuid) REFERENCES folder (uuid)
);
```

### Default Containers
- Default context UUID: `00000000-0000-0000-0000-000000000001` (title: "none")
- Default folder UUID: `00000000-0000-0000-0000-000000000002` (title: "none")

### Migration

**New Installations**: Databases created with `--init` include UUID columns from the start.

**Existing Local Databases**: Use the standalone migration utility:
```bash
cd cmd/migrate_local && go build
./migrate_local /path/to/listmanager.db
```

**Existing PostgreSQL Databases**: Run the migration script:
```bash
# Backup first!
pg_dump -h host -U user -d db > backup.sql

# Then migrate
psql -h host -U user -d db -f cmd/create_dbs/postgres_migrate_uuid.sql
```

**Automatic Migration** (app.go): The application also runs `MigrateToUUID()` on startup, which is idempotent and safe to run multiple times.

### PostgreSQL Synchronization with UUIDs

The sync system now supports full UUID synchronization between SQLite and PostgreSQL:

**Container Sync**:
- Client-generated UUIDs are sent to PostgreSQL when creating new containers
- PostgreSQL assigns `tid` values which are synced back to the client
- Both `tid` and `uuid` are maintained for bidirectional sync

**Task Sync**:
- Tasks sync both `context_tid`/`folder_tid` and `context_uuid`/`folder_uuid`
- When syncing tasks created in local-only mode, `tid` is resolved from `uuid`
- Fallback to default containers if resolution fails

**Keyword Sync**:
- `task_keyword` table includes `keyword_uuid` column
- Keywords sync with both `tid` and `uuid` references

### Implementation Files
- `init.go` - Schema definitions with UUID columns, UUID generation helper
- `dbfunc.go` - Container/task CRUD operations using UUID
- `app.go` - `MigrateToUUID()` migration function
- `common.go` - Container and Entry structs with uuid fields
- `organizer_cmd_line.go` - Container assignment commands
- `synchronize.go` - Full UUID sync support for containers, tasks, and keywords
- `cmd/migrate_local/main.go` - Standalone local database migration utility
- `cmd/create_dbs/postgres_migrate_uuid.sql` - PostgreSQL migration script
- `cmd/create_dbs/postgres_init4.sql` - New PostgreSQL schema with UUID support
