package main

import (
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Only the main loop may draw or touch UI state (see MainLoop). These tests
// pin down the two things that used to do so from other goroutines: the
// resize signal, and a preview render whose images loaded in the background.

// A resize must arrive as an event for the main loop, not as a redraw run on
// the signal goroutine. The app here has no Screen at all, so the old
// behaviour -- handleResize called straight from the goroutine -- would crash
// the test binary rather than fail an assertion.
func TestResizeSignalBecomesAMainLoopEvent(t *testing.T) {
	a := &App{resizeCh: make(chan struct{}, 1)}
	setupSignalHandling(a)

	for i := 0; i < 5; i++ {
		if err := syscall.Kill(syscall.Getpid(), syscall.SIGWINCH); err != nil {
			t.Fatal(err)
		}
	}

	select {
	case <-a.resizeCh:
	case <-time.After(2 * time.Second):
		t.Fatal("SIGWINCH produced no event on resizeCh")
	}
	// a burst collapses: at most one more can be pending behind the first
	time.Sleep(50 * time.Millisecond)
	if n := len(a.resizeCh); n > 1 {
		t.Errorf("%d resize events pending, want at most 1", n)
	}
}

const kittyPlaceholder = "\U0010EEEE" // the cell glamour's image markers become

// previewApp is an organizer on one note (id 1) in a terminal that shows
// images, with a render manager -- enough for StartRender and applyResult. It
// runs in a temp directory so the image cache it creates is not the real one.
func previewApp(t *testing.T) (*App, *Organizer) {
	t.Helper()
	repo, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())

	sess := &Session{imgSizeY: 800}
	scr := &Screen{screenCols: 120, screenLines: 40, textLines: 30, divider: 40, totaleditorcols: 78, Session: sess}
	org := &Organizer{Session: sess, Screen: scr, view: TASK, mode: NORMAL, rows: []Row{{id: 1, title: "a note"}}}
	cfg := &dbConfig{}
	cfg.Glamour.Style = filepath.Join(repo, "default.json")
	a := &App{
		Session: sess, Screen: scr, Organizer: org, Config: cfg,
		kitty: true, kittyPlace: true, showImages: true, imageScale: 45,
	}
	a.RenderManager = NewRenderManager(a)

	prevApp, prevCache := app, globalImageCache
	app, globalImageCache = a, nil
	t.Cleanup(func() {
		a.RenderManager.Stop()
		app, globalImageCache = prevApp, prevCache
	})
	return a, org
}

// noteWithImage is markdown showing a small PNG that exists on disk, so the
// background load needs neither Google Drive nor the network.
func noteWithImage(t *testing.T) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	img.Set(1, 1, color.RGBA{0, 0, 0xff, 0xff})
	path := filepath.Join(t.TempDir(), "pic.png")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return "Preamble.\n\n![pic](" + path + ")\n"
}

// captureStdout returns what fn writes to the terminal.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	real := os.Stdout
	os.Stdout = w
	out := make(chan string)
	go func() {
		b, _ := io.ReadAll(r)
		out <- string(b)
	}()
	fn()
	os.Stdout = real
	w.Close()
	return <-out
}

// startImageRender renders the note and waits for its images to come back
// from the background goroutine, returning the result as MainLoop receives it.
func startImageRender(t *testing.T, a *App, markdown string) RenderResult {
	t.Helper()
	drawn := captureStdout(t, func() {
		a.RenderManager.StartRender(1, markdown, a.Screen.totaleditorcols)
	})
	if !strings.Contains(drawn, "Preamble.") {
		t.Fatalf("StartRender did not draw the note's text at once: %q", drawn)
	}
	if strings.Contains(strings.Join(a.Organizer.note, "\n"), kittyPlaceholder) {
		t.Fatal("the first, text-only render already holds an image")
	}
	select {
	case res := <-a.RenderManager.resultCh:
		return res
	case <-time.After(5 * time.Second):
		t.Fatal("the background image load delivered nothing")
		return RenderResult{}
	}
}

// The background goroutine only loads: o.note changes, and the image reaches
// the terminal, when the main loop applies the result.
func TestLoadedImagesAreAppliedByTheMainLoop(t *testing.T) {
	a, org := previewApp(t)
	res := startImageRender(t, a, noteWithImage(t))

	if strings.Contains(strings.Join(org.note, "\n"), kittyPlaceholder) {
		t.Fatal("o.note was replaced before the main loop applied the result")
	}
	if prep := res.prepared[extractImageURLs(res.req.Markdown)[0]]; prep == nil || prep.err != nil || len(prep.data) == 0 {
		t.Fatalf("image was not loaded in the background: %+v", prep)
	}

	drawn := captureStdout(t, func() { a.RenderManager.applyResult(res) })

	if !strings.Contains(strings.Join(org.note, "\n"), kittyPlaceholder) {
		t.Error("after applyResult the note holds no image placeholder")
	}
	if !strings.Contains(drawn, "\x1b_G") {
		t.Error("applyResult did not transmit the image to the terminal")
	}
	if !strings.Contains(drawn, kittyPlaceholder) {
		t.Error("applyResult did not redraw the pane with the image")
	}
}

// A result is dropped -- nothing drawn, o.note untouched -- when the pane is
// no longer showing that render.
func TestImageResultThatCannotBeShownIsDropped(t *testing.T) {
	cases := map[string]func(a *App, org *Organizer){
		// the bug this replaces: the pane was erased under the open editor
		"note opened in the editor": func(a *App, org *Organizer) { org.Session.editorMode = true },
		"help is over the preview":  func(a *App, org *Organizer) { org.mode = HELP },
		"a sync report is over it":  func(a *App, org *Organizer) { org.mode = NAVIGATE_NOTICE },
		"container view":            func(a *App, org *Organizer) { org.view = FOLDER },
		"another row is selected":   func(a *App, org *Organizer) { org.rows[0].id = 2 },
		"a newer render replaced it": func(a *App, org *Organizer) {
			captureStdout(t, func() { a.RenderManager.StartRender(1, "just text now", a.Screen.totaleditorcols) })
		},
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			a, org := previewApp(t)
			res := startImageRender(t, a, noteWithImage(t))
			change(a, org)
			before := strings.Join(org.note, "\n")

			drawn := captureStdout(t, func() { a.RenderManager.applyResult(res) })

			if drawn != "" {
				t.Errorf("a dropped result wrote to the terminal: %q", drawn)
			}
			if after := strings.Join(org.note, "\n"); after != before {
				t.Errorf("a dropped result replaced o.note:\n%q\nwas\n%q", after, before)
			}
		})
	}
}
