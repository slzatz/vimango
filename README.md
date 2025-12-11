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
- For HTML rendering, there is a built-in webviewer that uses the go bindings to the webview library
- Syncing of notes to a remote PostgreSQL database (optional)
- Note editing supports full vim keybindings via libvim, which was originally develeped to support the Onivim 2 editor
- There is full-text search via sqlite's fts5 extension
- Spell checking through the use of the hunspell library
- You can launch deep research via Claude and the results will be stored as a note

This wasn't developed thinking anyone else would use it so there isn't an installable package. You'll need to clone the repository and build it yourself.  There are a few dependencies that you'll need to have installed first.  These are:

 - Go 1.20 or later
 - SQLite3 development files
 - Hunspell development files
 - libvimi.a is included in the repository

## Quick Start

**First-time setup (recommended):**
```bash
# 1. Clone and build
git clone https://github.com/slzatz/vimango.git
cd vimango
CGO_ENABLED=1 go build --tags="fts5,cgo"

# 2. Run first-time setup (creates config.json and databases)
./vimango --init

# 3. Run the application
./vimango
```

**Manual setup (alternative):**
1. Copy the example config: `cp config.json.example config.json`
2. Edit `config.json` with your settings (see below)
3. Create SQLite databases manually or let `--init` do it
4. Build: `CGO_ENABLED=1 go build --tags="fts5,cgo"`
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

The full application makes heavy use of CGO to access various C libraries but it can be compiled without using CGO.

So if this hasn't been offputting enough, after you can clone the repository you can build as follows:

 - **Linux with CGO**: `CGO_ENABLED=1 go build --tags="fts5,cgo"` (includes libvim, hunspell, sqlite3)
 - **Linux Pure Go**: `CGO_ENABLED=0 go build --tags=fts5` (no CGO dependencies)
 - **Windows Cross-Compilation**: `GOOS=windows GOARCH=amd64 go build --tags=fts5` (pure Go only)

The main runtime options are:

 - `--help`, `-h`: Display help message with all available options and exit
 - `--go-vim`: Use pure Go vim implementation (default: CGO-based libvim)
 - `--go-sqlite`: Use pure Go SQLite driver (modernc.org/sqlite) - default
 - `--cgo-sqlite`: Use CGO SQLite driver (mattn/go-sqlite3)

If you actually manage to get the application running there is a help system:

 - `:help` - Show all available ex commands organized by category
 - `:help normal` - Show all normal mode commands organized by category
 - `:help <command>` - Show detailed help for specific ex command with usage and examples
 - `:help <key>` - Show detailed help for specific normal mode command (e.g., `:help Ctrl-H`)
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
3. Run vimango - on first run, it will:
   - Print an authorization URL
   - Open the URL in your browser (or copy/paste it)
   - Sign in with your Google account and authorize the app
   - Paste the authorization code back into the terminal
4. A `token.json` file will be created to store your access token

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
