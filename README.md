# Vimango
So this is basically an application to store notes and other documents.

The notes are stored in a local sqlite database that can be synced to a remote server so you can access and update notes from multiple devices.

There are a few semi-notable features:
- Markdown rendering of notes in the terminal based on charm's glamour package
- Images in the terminal are supported using the kitty unicode placeholder protocol
- **Google Drive image support (optional)** - Pull images directly from Google Drive into your notes
     - The markdown syntax is `![alt text](gdrive:<file id>)`
     - Images are cached both in-memory (via kitty) and on disk (as Base64 PNG files)
     - See [Google Drive Setup](#google-drive-setup-optional) below for configuration
- Terminal Markdown rendering supports kitty's text sizing protocol
- `vimango --render-html <id>` renders a note as a standalone HTML document (it drives vimango_hybrid's preview pane)
- Syncing of notes to a remote PostgreSQL database (optional)
- Note editing supports full vim keybindings via libvim, which was originally develeped to support the Onivim 2 editor
- There is full-text search via sqlite's fts5 extension
- Spell checking with vim's own spell checker (from libvim): misspellings are highlighted as you type, and `]s`, `zg`, `z=` work as in vim
- You can launch deep research via Claude and the results will be stored as a note

This wasn't developed thinking anyone else would use it so there isn't an installable package. You'll need to clone the repository and build it yourself. On macOS, [MACOS_SETUP.md](./MACOS_SETUP.md) walks a completely fresh machine (no git/Homebrew/Go) through the whole setup; [INSTALL.md](./INSTALL.md) is the condensed build guide. There are a few dependencies that you'll need to have installed first.  These are:

 - Go 1.20 or later
 - SQLite3 development files
 - libvim.a (must be built from source — see [Building libvim.a](#building-libvima) below)

## Building libvim.a

The `libvim.a` static library provides vim modal editing via CGO. It must be compiled from source because the binary differs between macOS and Linux — every machine builds its own. CGO is required: libvim is the only vim engine (the pure-Go govim engine was removed; Windows is unsupported).

**Source:** the [slzatz/libvim](https://github.com/slzatz/libvim) fork. Use it rather than the original [onivim/libvim](https://github.com/onivim/libvim), which is unmaintained: the fork carries fixes vimango depends on — notably Visual-mode `I`/`A` behaving as vim's operators, so blockwise `ctrl-v jjI- <esc>` prefixes every selected line, and `vimSetMappingsEnabled`, which makes the mappings in `vimango.vim` work. A `libvim.a` built from upstream, or from a fork checkout older than your vimango, either fails to link (`undefined: vimSetMappingsEnabled`) or builds with the behavior silently missing — so **rebuild it whenever the fork moves**. See that repo's `PATCHES.md`.

Install the prerequisites:

- **macOS:** `brew install ncurses gettext` (the script below does this for you)
- **Debian/Ubuntu:** `sudo apt-get install make libtinfo-dev libacl1-dev`
- **Arch Linux:** `sudo pacman -S --needed make ncurses acl`

Then, from the vimango root, on either platform:
```bash
./scripts/build-libvim.sh
```

It clones the fork to `../libvim` (or uses the checkout already there — `LIBVIM_DIR` points it elsewhere), refuses a checkout that isn't the fork, isn't on `master` or has local changes, pulls `master`, builds with the fork's own `build-macos.sh` / `build-linux.sh`, and copies `libvim.a` into the vimango root. `--clean` rebuilds from scratch, `--tests` runs libvim's API tests, `--no-pull` builds the checkout as it is.

By hand, the equivalent is:
```bash
git clone https://github.com/slzatz/libvim.git ../libvim   # or: git -C ../libvim pull
(cd ../libvim && ./build-macos.sh)                         # Linux: ./build-linux.sh
cp ../libvim/src/libvim.a .
```

The fork's build scripts own the configure flags — the `-Wno-error` set that current clang and GCC 14+ need (libvim builds with `-Werror`), plus `-fPIC` on Linux — and reconfigure only when they change. The fork's `CLAUDE.md` explains each flag.

### Build Notes

- The `libvim.a` file must be placed in the vimango project root directory (next to `go.mod`).
- You do **not** need to copy or regenerate `auto/config.h` or `auto/pathdef.c` from your libvim build; the versions checked into this repo work on both platforms.

## Building heic_worker

The CGO HEIC decoder uses a separate worker subprocess (`heic_worker`) so libheif crashes can't take down vimango. The worker binary must be built per-platform and placed alongside the main `vimango` executable. It is excluded from version control via `.gitignore`.

Install `libheif`:

- **Arch Linux:** `sudo pacman -S libheif`
- **Debian/Ubuntu:** `sudo apt-get install libheif-dev`
- **macOS:** `brew install libheif`

Build and copy:

```bash
cd cmd/heic_worker
CGO_ENABLED=1 go build -o ../../heic_worker
```

If `heic_worker` is missing or built for the wrong architecture, vimango will silently fall through HEIC images (the placeholder will not render). HEIC support is optional — non-HEIC images are unaffected, and vimango itself builds without libheif.

## Quick Start

**First-time setup (recommended):**
```bash
# 1. Clone the repository (the glamour fork it builds against is pinned in go.mod and downloaded by go build)
git clone https://github.com/slzatz/vimango.git
cd vimango

# 2. Build libvim.a from the slzatz/libvim fork into this directory (see "Building libvim.a" above)
./scripts/build-libvim.sh

# 3. Build heic_worker (only if you want CGO HEIC support — see "Building heic_worker" above)
cd cmd/heic_worker && CGO_ENABLED=1 go build -o ../../heic_worker && cd ../..

# 4. Build the application
CGO_ENABLED=1 go build --tags=fts5

# 5. Run first-time setup (creates config.json and databases)
./vimango --init

# 6. Run the application
./vimango
```

**Manual setup (alternative):**
1. Copy the example config: `cp config.json.example config.json`
2. Edit `config.json` with your settings (see below)
3. Create SQLite databases manually or let `--init` do it
4. Build: `CGO_ENABLED=1 go build --tags=fts5`
5. Run: `./vimango`

## Configuration

The `config.json` file structure (copy from `config.json.example`):

```json
{
  "options": {
    "type": "folder",
    "title": "vimango"
  },
  "postgres": {
    "host": "",
    "port": "",
    "user": "",
    "password": "",
    "db": "vimango"
  },
  "sqlite3": {
    "db": "vimango.db",
    "fts_db": "fts5_vimango.db"
  },
  "chroma": {
    "style": "gruvbox_mod.xml"
  },
  "claude": {
    "api_key": ""
  },
  "glamour": {
    "style": "darkslz.json"
  }
}

```

**Notes on configuration:**
- **options**: Notes can be tagged with both "contexts" and "folders" - two parallel tagging systems
- **postgres**: Remote sync is optional - leave empty if not using remote sync
- **sqlite3**: Local database settings - these files will be created automatically
- **chroma**: Syntax highlighting style for code blocks in markdown
- **glamour**: Markdown rendering style - `darkslz.json` and `default.json` are included
- **claude**: API key for deep research feature (optional)

The full application makes heavy use of CGO to access various C libraries (libvim, sqlite3); CGO is required — pure-Go builds are unsupported.

So if this hasn't been offputting enough, after you can clone the repository you can build as follows:

 - `CGO_ENABLED=1 go build --tags=fts5` (the `fts5` tag is mandatory and enforced at compile time by `fts5_guard.go`)

The main runtime options are:

 - `--help`, `-h`: Display help message with all available options and exit

If you actually manage to get the application running there is a help system:

 - `:help` - Show all available ex commands organized by category
 - `:help normal` - In the organizer, all normal mode commands by category; in the editor, the vimango.vim mappings (normal mode there is plain vim)
 - `:help <command>` - Show detailed help for specific ex command with usage and examples
 - `:help <key>` - In the organizer, detailed help for a normal mode command (e.g., `:help Ctrl-D`)
 - `:help <category>` - Show all commands in a specific category (e.g., `:help Navigation`)              

## Google Drive Setup (Optional)

Google Drive integration is **optional**. The application works fine without it - you just won't be able to display images stored in Google Drive. If you try to view a `gdrive:` image without credentials configured, you'll see a message explaining how to set it up.

If you want to use Google Drive images in your notes, follow these steps:

### 1. Create a Google Cloud Project

1. Go to the [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project (or select an existing one)
3. Enable the **Google Drive API** for your project:
   - Go to "APIs & Services" → "Library"
   - Search for "Google Drive API" and enable it

### 2. Create OAuth 2.0 Credentials

1. Go to "APIs & Services" → "Credentials"
2. Click "Create Credentials" → "OAuth client ID"
3. If prompted, configure the OAuth consent screen first:
   - Choose "External" user type (unless you have a Workspace account)
   - Fill in the required fields (app name, user support email, developer email)
   - Add yourself as a test user
4. Create an OAuth client ID:
   - Application type: **Desktop app**
   - Name: "Vimango" (or whatever you prefer)
5. Download the credentials JSON file

### 3. Configure Vimango

1. Rename the downloaded file to `go_credentials.json`
2. Place it in the vimango directory (same directory as the executable)
3. Run `./vimango --gdrive-auth`. It will:
   - Print an authorization URL
   - Open the URL in your browser (or copy/paste it)
   - Sign in with your Google account and authorize the app
   - Paste the authorization code back into the terminal
4. A `token.json` file will be created to store your access token

`--gdrive-auth` is also the fix whenever the stored token stops working
(expired or revoked): it verifies `token.json` with a live API call and
re-runs the sign-in flow if needed. (Running the TUI without a `token.json`
triggers the same prompt as a side effect, but the flag is the reliable,
scriptable way in.)

### 4. Using Google Drive Images

To include a Google Drive image in a note, use the syntax:

```markdown
![description](gdrive:FILE_ID)
```

Where `FILE_ID` is the Google Drive file ID. You can find this in the file's URL:
`https://drive.google.com/file/d/FILE_ID/view`

### Security Notes

- `go_credentials.json` contains your OAuth client credentials - don't share it
- `token.json` contains your access token - don't share it
- Both files should be added to `.gitignore` (they already are in this repo)

Here is a screenshot:
![Vimango Screenshot](images/vimango_screenshot.png)
