package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/slzatz/vimango/internal/localdb"
)

// renderWorkspace is a directory as --render-html finds one: a config.json
// naming a vimango.db that holds one note with this text, and no Drive
// sign-in. The test runs in it, and gets the note's id.
func renderWorkspace(t *testing.T, config, title, text string) (id int) {
	t.Helper()
	dir := t.TempDir()
	t.Chdir(dir)
	if config == "" {
		config = `{"sqlite3": {"db": "vimango.db"}}`
	}
	if err := os.WriteFile("config.json", []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	db, err := localdb.Open(filepath.Join(dir, "vimango.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := localdb.Create(db); err != nil {
		t.Fatal(err)
	}
	err = db.QueryRow("INSERT INTO task (tid, title, note, added) VALUES (77, ?, ?, datetime('now')) RETURNING id;", title, text).Scan(&id)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// --render-html needs no App: no organizer, no terminal, none of what the TUI
// builds at startup. It used to make a whole one to get at the database, two
// colours and the Drive sign-in, and the renderer reached them through the
// global.
func TestRenderHTMLBuildsNoApp(t *testing.T) {
	const note = "---\nstatus: draft\n---\n\n# Heading\n\nSee [[tid:77]] and [[Another note]].\n\n> [!NOTE]\n> A callout.\n\n- [x] done\n"
	id := renderWorkspace(t, "", "The <title>", note)
	prev := app
	app = nil
	t.Cleanup(func() { app = prev })

	var stdout, stderr bytes.Buffer
	if code := renderNote(id, true, false, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d; stderr: %s", code, stderr.String())
	}
	if app != nil {
		t.Error("rendering made an App")
	}
	html := stdout.String()
	for _, want := range []string{
		"<!DOCTYPE html>",
		"<title>The &lt;title&gt;</title>",
		`class="frontmatter"`,     // the YAML block became its card
		`id="heading"`,            // goldmark's heading ids
		`href="vimango://tid/77"`, // a tid link, resolved through the store...
		">The &lt;title&gt;</a>",  // ...to the note's current title
		`href="vimango://note/Another%20note"`,
		"callout",
		`type="checkbox"`,
	} {
		if !strings.Contains(html, want) {
			t.Errorf("the page lacks %q", want)
		}
	}
	if strings.Contains(html, "<h1>The") {
		t.Error("the title is on the page without --title")
	}
	// the one warning expected: nobody is signed in to Drive here
	if got := stderr.String(); !strings.Contains(got, "Google Drive unavailable") || strings.Contains(got, "Error") {
		t.Errorf("stderr: %q", got)
	}
}

func TestRenderHTMLFlags(t *testing.T) {
	id := renderWorkspace(t, "", "A title", "See [[Another note]].\n")
	page := func(linkNotes, showTitle bool) string {
		t.Helper()
		var stdout, stderr bytes.Buffer
		if code := renderNote(id, linkNotes, showTitle, &stdout, &stderr); code != 0 {
			t.Fatalf("exit code %d; stderr: %s", code, stderr.String())
		}
		return stdout.String()
	}
	if html := page(false, false); strings.Contains(html, "<h1>") || strings.Contains(html, "vimango://note/") || !strings.Contains(html, `<span class="wikilink"`) {
		t.Error("without flags: want no <h1>, and the reference as a span")
	}
	if html := page(false, true); !strings.Contains(html, "<h1>A title</h1>") {
		t.Error("--title did not put the title on the page")
	}
	if html := page(true, false); !strings.Contains(html, `<a class="wikilink" href="vimango://note/Another%20note"`) {
		t.Error("--wiki-links did not make the reference an anchor")
	}
}

// What goes wrong is said on stderr and in the exit code, and nothing is
// written to stdout, which the host reads as the page.
func TestRenderHTMLFailures(t *testing.T) {
	id := renderWorkspace(t, "", "A title", "text")
	run := func(id int) (code int, out, errs string) {
		var stdout, stderr bytes.Buffer
		code = renderNote(id, false, false, &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}

	if code, out, errs := run(id + 1000); code != 1 || out != "" || !strings.Contains(errs, "no note with id") {
		t.Errorf("a note that is not there: exit %d, stdout %q, stderr %q", code, out, errs)
	}

	if err := os.Rename("vimango.db", "elsewhere.db"); err != nil {
		t.Fatal(err)
	}
	if code, out, errs := run(id); code != 1 || out != "" || !strings.Contains(errs, "could not open databases") {
		t.Errorf("no database: exit %d, stdout %q, stderr %q", code, out, errs)
	}
	if _, err := os.Stat("vimango.db"); err == nil {
		t.Error("rendering created an empty vimango.db")
	}

	if err := os.Remove("config.json"); err != nil {
		t.Fatal(err)
	}
	if code, out, errs := run(id); code != 1 || out != "" || !strings.Contains(errs, "could not open databases") {
		t.Errorf("no config.json: exit %d, stdout %q, stderr %q", code, out, errs)
	}
}

// config.json's two colours reach the stylesheet, and only as colours.
func TestRenderHTMLAccents(t *testing.T) {
	for _, tc := range []struct{ accent, code, wantAccent, wantCode string }{
		{"", "", defaultAccent, defaultAccent},
		{"#ff8800", "", "#ff8800", "#ff8800"},        // code follows the accent
		{"#ff8800", "#00aa00", "#ff8800", "#00aa00"}, // unless it has its own
		{"none", "#abc", "currentColor", "#abc"},
		{"red; } body { display: none", "expression(1)", defaultAccent, defaultAccent}, // not a colour: the default
	} {
		accent, code := accents(tc.accent, tc.code)
		if accent != tc.wantAccent || code != tc.wantCode {
			t.Errorf("accents(%q, %q) = %q, %q; want %q, %q", tc.accent, tc.code, accent, code, tc.wantAccent, tc.wantCode)
		}
		// a renderer built by hand with the raw values is no less safe
		html, err := htmlRenderer{accent: tc.accent, codeAccent: tc.code}.render("t", "text")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(html, "--accent: "+tc.wantAccent+";") || !strings.Contains(html, "--code-accent: "+tc.wantCode+";") {
			t.Errorf("accent %q, code %q: the stylesheet does not have %s and %s", tc.accent, tc.code, tc.wantAccent, tc.wantCode)
		}
		if strings.Contains(html, "display: none") || strings.Contains(html, "expression(") {
			t.Errorf("accent %q reached the stylesheet as written", tc.accent)
		}
	}

	// and from config.json, through --render-html
	id := renderWorkspace(t, `{"sqlite3": {"db": "vimango.db"}, "webview": {"accent": "#123456", "code_accent": "off"}}`, "t", "text")
	var stdout, stderr bytes.Buffer
	if code := renderNote(id, false, false, &stdout, &stderr); code != 0 {
		t.Fatal(stderr.String())
	}
	if html := stdout.String(); !strings.Contains(html, "--accent: #123456;") || !strings.Contains(html, "--code-accent: currentColor;") {
		t.Error("config.json's webview colours are not in the stylesheet")
	}
}

// A Google Drive image is put in the page itself: from the cache when it is
// there, in either form the cache holds. With nobody signed in, one that is
// not cached becomes a placeholder that says what to do, and the rest of the
// note still renders.
func TestRenderHTMLDriveImages(t *testing.T) {
	const pngBase64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
	const raw, uri, missing = "gdrive:rawRAWrawRAWrawRAWrawRAWraw01", "gdrive:uriURIuriURIuriURIuriURIuri02", "gdrive:goneGONEgoneGONEgoneGONEgon03"
	cache := newTestImageCache(t)
	// the TUI stores bare base64; --render-html stores the data URI it made
	if err := cache.StoreCachedImage(raw, pngBase64); err != nil {
		t.Fatal(err)
	}
	if err := cache.StoreCachedImage(uri, "data:image/jpeg;base64,/9j/abc"); err != nil {
		t.Fatal(err)
	}

	note := "Before.\n\n![one](" + raw + ")\n\n![two](" + uri + ")\n\n![three <b>](" + missing + ")\n\nAfter.\n"
	html, err := htmlRenderer{cache: cache}.render("t", note)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`<img src="data:image/png;base64,` + pngBase64 + `" alt="one"`,
		`<img src="data:image/jpeg;base64,/9j/abc" alt="two"`,
		`<span class="img-placeholder"`,
		"three &lt;b&gt;",
		"vimango --gdrive-auth",
		"<p>Before.</p>", "<p>After.</p>",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("the page lacks %q", want)
		}
	}
	if strings.Contains(html, "gdrive:") {
		t.Error("a gdrive: address was left in the page")
	}

	// with no cache at all it is the same answer for all three
	html, err = htmlRenderer{}.render("t", note)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(html, `<span class="img-placeholder"`); n != 3 {
		t.Errorf("with no cache and no sign-in, %d placeholders; want 3", n)
	}
}
