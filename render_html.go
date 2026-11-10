package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/slzatz/vimango/auth"
)

// DetermineWikiLinks reports whether --wiki-links was given: the caller
// declaring itself a host that will intercept vimango://note/ clicks. Off
// by default, because a link nothing handles is worse than no link at all
// (wikilink.go).
func DetermineWikiLinks(args []string) bool {
	for _, arg := range args {
		if arg == "--wiki-links" {
			return true
		}
	}
	return false
}

// DetermineTitle reports whether --title was given: put the note's title
// at the top of a --render-html page. A host pane has its own title bar
// and leaves it off; an export (a PDF) has nothing else to name it.
func DetermineTitle(args []string) bool {
	for _, arg := range args {
		if arg == "--title" {
			return true
		}
	}
	return false
}

// DetermineRenderHTML returns the note id passed to --render-html, or -1 if
// the flag is absent. ValidateArgs has already guaranteed the value is
// present and numeric.
func DetermineRenderHTML(args []string) int {
	for i, arg := range args {
		if arg == "--render-html" && i+1 < len(args) {
			if id, err := strconv.Atoi(args[i+1]); err == nil {
				return id
			}
		}
	}
	return -1
}

// RunRenderHTML renders note <id> as a standalone HTML document on stdout
// and returns a process exit code. Headless: no terminal probes, no vim, no
// raw mode — safe to spawn from a host app with stdout piped (the hybrid
// app's preview pane). Google Drive images resolve to inline data URIs via
// the same auth token and image cache the TUI uses; if Drive is not
// configured the image markdown is passed through untouched.
func RunRenderHTML(id int, linkNotes, showTitle bool) int {
	// The HEIC decoder leaves a plugin subprocess running that outlives
	// this process unless it is killed. Deferred rather than placed at
	// the end: every return below is an early one, and main() calls
	// os.Exit on our result, which runs no defers of its own.
	defer ShutdownHEICDecoder()
	// ...and a host that no longer wants this render kills it (SIGTERM),
	// which runs no defers either.
	killHEICWorkerOnSignal()

	app = CreateApp()

	prefs := app.LoadPreferences("preferences.json")
	app.imageCacheMaxWidth = prefs.ImageCacheMaxWidth

	// Optional: without Drive the note still renders, with a visible
	// placeholder where each gdrive: image would be. Headless variant —
	// stdio is piped, so the interactive OAuth prompt the TUI boot path
	// falls into on a missing token.json would block forever here.
	if srv, err := auth.GetDriveServiceHeadless(); err == nil {
		app.Session.googleDrive = srv
	} else {
		fmt.Fprintf(os.Stderr, "Warning: Google Drive unavailable: %v\n", err)
	}
	initImageCache()

	if err := app.InitDatabases("config.json"); err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not open databases: %v\n", err)
		return 1
	}

	title, note, err := app.Store.TitleAndNote(id)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: no note with id %d: %v\n", id, err)
		return 1
	}

	// Embedded-pane style: left-aligned content instead of a centered
	// reading column, and no <h1> unless asked for (--title) — the host
	// app's panes show the title in their own native header.
	html, err := RenderNoteAsHTML(title, note, linkNotes, showTitle)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not render note %d: %v\n", id, err)
		return 1
	}

	fmt.Print(html)
	return 0
}
