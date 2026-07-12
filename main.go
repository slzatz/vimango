package main

import (
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"

	"github.com/slzatz/vimango/auth"
	"github.com/slzatz/vimango/rawmode"
	"github.com/slzatz/vimango/vim"
)

// Global app struct
var app *App

func main() {
	// Check for help flag first, before any initialization
	if CheckForHelp(os.Args) {
		os.Exit(0)
	}

	// Reject unknown flags before doing any other work
	ValidateArgs(os.Args)

	// Check for --init flag for first-time setup
	if CheckForInit(os.Args) {
		os.Exit(0)
	}

	// --render-html <id> is headless: print the note as HTML and exit,
	// before any terminal probing / vim / raw-mode setup.
	if id := DetermineRenderHTML(os.Args); id != -1 {
		os.Exit(RunRenderHTML(id))
	}

	// --gdrive-auth: sign in to Google Drive (create/refresh token.json)
	// and exit. No terminal UI.
	if CheckForGDriveAuth(os.Args) {
		os.Exit(RunGDriveAuth())
	}

	// --editor boots straight into a full-width editor (host-embedding mode)
	editorOnly, openId := DetermineEditorBoot(os.Args)

	app = CreateApp()
	app.Session.editorOnly = editorOnly

	// Load user preferences (before DetectKittyCapabilities so we can override imageScale)
	prefs := app.LoadPreferences("preferences.json")

	// Detect kitty terminal capabilities (sets default imageScale=45)
	app.DetectKittyCapabilities()

	// Override defaults with user preferences
	app.imageScale = prefs.ImageScale
	app.imageCacheMaxWidth = prefs.ImageCacheMaxWidth

	// Google Drive is optional - initialize if credentials are available.
	// In host-embedded editor mode stdio is a hidden pty, so a missing
	// token must fail quietly (images degrade with a message) instead of
	// wedging the editor on the interactive OAuth prompt; sign in with
	// `vimango --gdrive-auth` in a real terminal.
	getDriveService := auth.GetDriveService
	if editorOnly {
		getDriveService = auth.GetDriveServiceHeadless
	}
	srv, err := getDriveService()
	if err != nil {
		// Google Drive not configured - this is OK, the app will work without it
		// Users with gdrive: images will see a message explaining how to set it up
		app.Session.googleDrive = nil
	} else {
		app.Session.googleDrive = srv
	}

	// Initialize image cache
	initImageCache()

	// Initialize Vim (libvim via CGO — the only implementation)
	vim.InitializeVim(0)

	// Initialize database connections
	err = app.InitDatabases("config.json")
	if err != nil {
		// Distinguish "config.json itself is missing" from "config references a missing file"
		if _, statErr := os.Stat("config.json"); os.IsNotExist(statErr) {
			fmt.Println("Error: config.json not found.")
			fmt.Println()
			fmt.Println("If this is a new installation, run:")
			fmt.Println("  ./vimango --init")
			fmt.Println()
			fmt.Println("This will create config.json and initialize the SQLite databases.")
			os.Exit(1)
		}
		// Check if database files are missing
		if errors.Is(err, ErrDatabaseNotFound) {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
		// Some other file referenced by config.json is missing, or another failure
		var pathErr *fs.PathError
		if errors.As(err, &pathErr) && os.IsNotExist(pathErr) {
			fmt.Printf("Error: file referenced by config.json not found: %s\n", pathErr.Path)
			fmt.Println("Check paths in config.json (e.g., postgres SSL cert, sqlite DB).")
			os.Exit(1)
		}
		log.Fatal(err)
	}

	// Validate database schema exists
	if err := app.ValidateDatabaseSchema(); err != nil {
		fmt.Printf("Error: Database schema is missing or incomplete.\n")
		fmt.Printf("Details: %v\n", err)
		fmt.Println()
		fmt.Println("Run './vimango --init' to initialize the databases.")
		fmt.Println("(You may need to remove existing .db files first if they are corrupted)")
		os.Exit(1)
	}

	// Validate --open id while we can still print to a cooked terminal
	if openId != -1 && !app.Database.entryExists(openId) {
		fmt.Printf("Error: no note with id %d\n", openId)
		os.Exit(1)
	}

	// Migrate existing databases to UUID-based containers if needed
	if err := app.MigrateToUUID(); err != nil {
		fmt.Printf("Error: Database migration failed.\n")
		fmt.Printf("Details: %v\n", err)
		os.Exit(1)
	}

	// Validate glamour style file exists
	if err := validateGlamourStyle(); err != nil {
		log.Fatalf("Error: %v", err)
	}

	// Initialize research manager if Claude API key is configured
	if app.Config != nil && app.Config.Claude.ApiKey != "" && app.Config.Claude.ApiKey != "CLAUDE_API_KEY_HERE" {
		app.InitResearchManager(app.Config.Claude.ApiKey)
	}

	// Initialize async render manager for non-blocking image loading
	app.InitRenderManager()

	// Set up platform-specific signal handling
	setupSignalHandling(app)

	// Configure Vim settings
	vim.ExecuteCommand("set iskeyword+=*")
	vim.ExecuteCommand("set iskeyword+=`")

	// Enable raw mode
	origCfg, err := rawmode.Enable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error enabling raw mode: %v", err)
		os.Exit(1)
	}

	app.origTermCfg = origCfg
	app.Session.editorMode = false

	// Get window size
	err = app.Screen.GetWindowSize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting window size: %v", err)
		os.Exit(1)
	}
	//os.Exit(0) //debugging

	app.InitApp()

	// Set edPct BEFORE LoadInitialData() so it uses the correct value
	// LoadInitialData() will calculate divider and totaleditorcols based on this
	app.Screen.edPct = prefs.EdPct
	if editorOnly {
		// full-width editor; organizer never rendered (divider = 1)
		app.Screen.edPct = 100
	}

	app.LoadInitialData()

	if editorOnly {
		// openId == -1 opens the current (most recently modified) row
		app.Organizer.editNote(openId)
		if app.Session.activeEditor == nil {
			// e.g. empty database: nothing to edit
			rawmode.Restore(app.origTermCfg)
			fmt.Println("Error: no note to open (database has no saved entries)")
			os.Exit(1)
		}
	}

	app.Run = true
	app.MainLoop()
}
