package main

import (
	"fmt"
	"io"
	"log"
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
	return noteIDFlag(args, "--render-html")
}

// DetermineRenderMarkdown returns the note id passed to --render-markdown,
// or -1 if the flag is absent.
func DetermineRenderMarkdown(args []string) int {
	return noteIDFlag(args, "--render-markdown")
}

// noteIDFlag is the note id that follows flag in args, or -1.
func noteIDFlag(args []string, flag string) int {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
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
	// the end: main() calls os.Exit on our result, which runs no defers
	// of its own.
	defer ShutdownHEICDecoder()
	// ...and a host that no longer wants this render kills it (SIGTERM),
	// which runs no defers either.
	killHEICWorkerOnSignal()

	return renderNote(id, linkNotes, showTitle, os.Stdout, os.Stderr)
}

// renderNote is --render-html itself: the note with an id, as HTML on stdout,
// with anything that went wrong on stderr and in the exit code it returns.
//
// It builds no App. What rendering needs is a note from the store and an
// htmlRenderer made from config.json, preferences.json and the Drive sign-in;
// nothing here or beneath it touches the global app, which stays nil in this
// mode.
func renderNote(id int, linkNotes, showTitle bool, stdout, stderr io.Writer) int {
	config, err := loadConfig("config.json")
	if err != nil {
		fmt.Fprintf(stderr, "Error: could not open databases: %v\n", err)
		return 1
	}
	notes, err := openStore(config)
	if err != nil {
		fmt.Fprintf(stderr, "Error: could not open databases: %v\n", err)
		return 1
	}
	defer notes.Close()

	r := htmlRenderer{linkNotes: linkNotes, showTitle: showTitle, titleForTID: notes.TitleForTID}
	r.accent, r.codeAccent = config.Webview.Accent, config.Webview.CodeAccent

	r.images, r.cache = driveForRender(stderr)

	title, note, err := notes.TitleAndNote(id)
	if err != nil {
		fmt.Fprintf(stderr, "Error: no note with id %d: %v\n", id, err)
		return 1
	}

	html, err := r.render(title, note)
	if err != nil {
		fmt.Fprintf(stderr, "Error: could not render note %d: %v\n", id, err)
		return 1
	}

	fmt.Fprint(stdout, html)
	return 0
}

// driveForRender is where a headless render's Google Drive images come from:
// the sign-in, and the disk cache the TUI's inline images share.
//
// Both are optional. Without Drive the note still renders, with a visible
// placeholder where each gdrive: image that is not cached would be; the
// headless sign-in, because stdio is piped, so the interactive OAuth prompt
// the TUI boot path falls into on a missing token.json would block forever
// here. Without a cache every image is downloaded for every render.
func driveForRender(stderr io.Writer) (images driveImages, cache *ImageCache) {
	var err error
	images.maxWidth = readPreferences("preferences.json").ImageCacheMaxWidth
	if images.service, err = auth.GetDriveServiceHeadless(); err != nil {
		fmt.Fprintf(stderr, "Warning: Google Drive unavailable: %v\n", err)
	}
	if cache, err = NewImageCache(); err != nil {
		log.Printf("Warning: Failed to initialize image cache: %v", err)
		cache = nil
	}
	return images, cache
}

// RunRenderMarkdown prints note <id> as markdown that needs no vimango to
// show its pictures -- each Google Drive image replaced by the image itself,
// as a data URI -- and returns a process exit code. Headless, as
// RunRenderHTML is, and for a host that hands the note to a markdown tool of
// its own: the hybrid app's slides, which are Marp's.
func RunRenderMarkdown(id int) int {
	// as in RunRenderHTML: the HEIC worker outlives this process unless
	// it is killed, on a normal exit and on a signal alike
	defer ShutdownHEICDecoder()
	killHEICWorkerOnSignal()

	return renderNoteMarkdown(id, os.Stdout, os.Stderr)
}

// renderNoteMarkdown is --render-markdown itself: the note's text on stdout,
// changed only where a Google Drive image was (htmlRenderer.markdown), with
// anything that went wrong on stderr and in the exit code it returns. Like
// renderNote, it builds no App.
func renderNoteMarkdown(id int, stdout, stderr io.Writer) int {
	config, err := loadConfig("config.json")
	if err != nil {
		fmt.Fprintf(stderr, "Error: could not open databases: %v\n", err)
		return 1
	}
	notes, err := openStore(config)
	if err != nil {
		fmt.Fprintf(stderr, "Error: could not open databases: %v\n", err)
		return 1
	}
	defer notes.Close()

	var r htmlRenderer
	r.images, r.cache = driveForRender(stderr)

	_, note, err := notes.TitleAndNote(id)
	if err != nil {
		fmt.Fprintf(stderr, "Error: no note with id %d: %v\n", id, err)
		return 1
	}

	markdown, err := r.markdown(note)
	if err != nil {
		fmt.Fprintf(stderr, "Error: could not render note %d: %v\n", id, err)
		return 1
	}

	fmt.Fprint(stdout, markdown)
	return 0
}
