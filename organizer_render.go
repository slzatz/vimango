package main

import (
	"strings"
	"sync"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
)

// Rendering a note for the organizer's preview pane.
//
// The rule this file keeps: only the main loop touches the terminal, the
// organizer and the kitty bookkeeping (session images, ids, the "current
// render" tables glamour's image lookup reads). The one thing worth doing in
// the background is the slow part of an image -- Drive download, disk read,
// decode -- so that is all the background goroutine does. It hands the loaded
// images to the main loop over resultCh, and the main loop transmits them,
// renders and draws (applyResult).
//
// Until 2026-11-10 the background goroutine also transmitted the images and
// ran glamour, and a second goroutine erased the pane and replaced o.note.
// Both raced the main loop's own drawing, and two renders in flight shared
// the "current render" tables.

// imageLimits is the pixel box a freshly loaded image is fitted to. It is
// read from the screen on the main loop when a render starts, so the loader
// goroutines never read Screen themselves.
type imageLimits struct {
	maxW, maxH int
}

// RenderRequest is one note's preview render. A newer request cancels it.
type RenderRequest struct {
	NoteID   int
	Markdown string
	MaxCols  int
	Limits   imageLimits
	CancelCh chan struct{}
}

// RenderResult is what the background hands back: the request and its images,
// loaded but not yet sent to the terminal.
type RenderResult struct {
	req      *RenderRequest
	prepared map[string]*preparedImage
}

// RenderManager coordinates the preview render of the organizer's current note.
type RenderManager struct {
	app            *App
	organizer      *Organizer
	currentRequest *RenderRequest // main loop only

	// resultCh carries loaded images to the main loop (MainLoop selects on it)
	resultCh chan RenderResult
	stopCh   chan struct{}
}

// NewRenderManager creates the render manager
func NewRenderManager(app *App) *RenderManager {
	return &RenderManager{
		app:       app,
		organizer: app.Organizer,
		resultCh:  make(chan RenderResult, 1),
		stopCh:    make(chan struct{}),
	}
}

// Stop cancels the render in flight; its goroutine exits without delivering.
func (rm *RenderManager) Stop() {
	close(rm.stopCh)
	rm.cancelCurrent()
}

func (rm *RenderManager) cancelCurrent() {
	if rm.currentRequest != nil {
		close(rm.currentRequest.CancelCh)
		rm.currentRequest = nil
	}
}

// StartRender draws a note in the preview pane. Called on the main loop.
// Text is drawn at once; if the note has images the terminal does not already
// hold, they are loaded in the background and the pane is redrawn with them
// when they arrive (applyResult).
func (rm *RenderManager) StartRender(noteID int, markdown string, maxCols int) {
	rm.cancelCurrent()

	o := rm.organizer
	var imageURLs []string
	if rm.imagesOn() {
		imageURLs = extractImageURLs(markdown)
	}
	if len(imageURLs) == 0 {
		o.note = rm.renderTextOnly(markdown, maxCols)
		o.drawRenderedNote()
		return
	}

	req := &RenderRequest{
		NoteID:   noteID,
		Markdown: markdown,
		MaxCols:  maxCols,
		Limits:   rm.imageLimits(),
		CancelCh: make(chan struct{}),
	}
	rm.currentRequest = req

	// Every image already in the terminal: nothing to wait for (the images
	// come off the disk cache), so render with them now.
	if rm.allImagesInKittyCache(imageURLs) {
		prepared := prepareImages(req, imageURLs, rm.stopCh)
		o.note = rm.renderWithImages(req, prepared)
		o.drawRenderedNote()
		return
	}

	// Text now, images when they have loaded.
	o.note = rm.renderTextOnly(markdown, maxCols)
	o.drawRenderedNote()
	o.ShowMessage(BR, "Loading %d image(s)...", len(imageURLs))

	go func() {
		prepared := prepareImages(req, imageURLs, rm.stopCh)
		if prepared == nil {
			return // cancelled
		}
		select {
		case rm.resultCh <- RenderResult{req: req, prepared: prepared}:
		case <-req.CancelCh:
		case <-rm.stopCh:
		}
	}()
}

// applyResult finishes a render whose images have loaded. Called on the main
// loop, which is what makes it safe to write to the terminal and to o.note.
func (rm *RenderManager) applyResult(res RenderResult) {
	if res.req != rm.currentRequest {
		return // a newer render replaced this one
	}
	o := rm.organizer
	// The pane may no longer be showing this note: the user opened it in the
	// editor, put :help or a sync report over it, or moved to another row.
	// Every way back to the preview calls displayNote, which renders afresh,
	// so a result that cannot be shown is simply dropped.
	if !o.previewVisible() || len(o.rows) == 0 || o.rows[o.fr].id != res.req.NoteID {
		return
	}

	o.note = rm.renderWithImages(res.req, res.prepared)
	o.Screen.eraseRightScreen()
	o.ShowMessage(BR, "") // clear "Loading..."
	o.drawRenderedNote()
}

// previewVisible reports whether the right pane is showing the rendered note
// rather than the editor or a notice (help, sync report, container list).
func (o *Organizer) previewVisible() bool {
	if o.Session.editor != nil || o.view != TASK {
		return false
	}
	switch o.mode {
	case NAVIGATE_NOTICE, HELP, CONTAINER:
		return false
	}
	return true
}

func (rm *RenderManager) imagesOn() bool {
	return rm.app.kitty && rm.app.kittyPlace && rm.app.showImages
}

// imageLimits reads the pixel box for loaded images off the screen.
func (rm *RenderManager) imageLimits() imageLimits {
	lim := imageLimits{maxW: int(rm.app.Screen.ws.Xpixel), maxH: int(rm.app.Screen.ws.Ypixel)}
	if lim.maxW == 0 {
		lim.maxW = rm.app.Screen.totaleditorcols * 8
	}
	if lim.maxH == 0 {
		lim.maxH = rm.app.Session.imgSizeY
	}
	return lim
}

// allImagesInKittyCache checks if all image URLs are in kitty's session cache
// This means they can be reused without network/disk loading
func (rm *RenderManager) allImagesInKittyCache(imageURLs []string) bool {
	if globalImageCache == nil {
		return false
	}

	for _, url := range imageURLs {
		// Check if we have cached metadata with a valid image ID
		entry, ok := globalImageCache.GetKittyMeta(url)
		if !ok || entry.ImageID == 0 {
			return false
		}

		// Check if this image ID is in the kitty session cache
		kittySessionImageMux.RLock()
		sessionEntry, inSession := kittySessionImages[entry.ImageID]
		kittySessionImageMux.RUnlock()

		if !inSession {
			return false
		}

		// Check if the session entry is confirmed or we trust the cache
		if !sessionEntry.confirmed && !trustKittyCache {
			return false
		}
	}

	return true
}

// renderTextOnly renders markdown without images (fast path)
func (rm *RenderManager) renderTextOnly(markdown string, maxCols int) []string {
	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath(getGlamourStylePath()),
		glamour.WithWordWrap(0),
	)
	if err != nil {
		return []string{"Error creating renderer: " + err.Error()}
	}
	note, err := r.Render(markdown)
	if err != nil {
		return []string{"Error rendering: " + err.Error()}
	}
	return rm.finishRender(note, maxCols)
}

// finishRender is the tail both renders share: text-size markers, search
// highlighting, wrapping to the pane.
func (rm *RenderManager) finishRender(note string, maxCols int) []string {
	note = strings.TrimSpace(note)

	// Decode any Kitty text sizing markers (OSC 66) that were protected during glamour processing
	note = ansi.DecodeKittyTextSizeMarkers(note)

	// Handle search highlighting
	if rm.organizer.taskview == BY_FIND {
		note = strings.ReplaceAll(note, "qx", "\x1b[48;5;31m")
		note = strings.ReplaceAll(note, "qy", "\x1b[0m")
	}

	note = WordWrap(note, maxCols-PREVIEW_RIGHT_PADDING, 0)
	return strings.Split(note, "\n")
}

// prepareImages loads a note's images -- from the disk cache, Google Drive, the
// web or a file -- with a few workers, and returns them by URL. It sends
// nothing to the terminal and reads no UI state, so it may run on any
// goroutine. Returns nil if the request was cancelled (or the app is stopping)
// before the images were all in.
func prepareImages(req *RenderRequest, imageURLs []string, stopCh <-chan struct{}) map[string]*preparedImage {
	cancelled := func() bool {
		select {
		case <-req.CancelCh:
			return true
		case <-stopCh:
			return true
		default:
			return false
		}
	}
	if cancelled() {
		return nil
	}

	// each URL once, however often the note shows it
	var urls []string
	seen := make(map[string]bool)
	for _, url := range imageURLs {
		if !seen[url] {
			seen[url] = true
			urls = append(urls, url)
		}
	}

	jobCh := make(chan string, len(urls))
	for _, url := range urls {
		jobCh <- url
	}
	close(jobCh)

	resCh := make(chan *preparedImage, len(urls))
	var wg sync.WaitGroup
	for i := 0; i < min(6, len(urls)); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for url := range jobCh {
				if cancelled() {
					return
				}
				resCh <- prepareKittyImage(url, req.Limits)
			}
		}()
	}
	wg.Wait()
	close(resCh)

	if cancelled() {
		return nil
	}
	prepared := make(map[string]*preparedImage, len(urls))
	for prep := range resCh {
		prepared[prep.url] = prep
	}
	return prepared
}

// renderWithImages sends the loaded images to the terminal and renders the
// note around them. Main loop only: it writes to the terminal and owns the
// "current render" tables that glamour's image lookup (kittyImageCacheLookup)
// reads back in order.
func (rm *RenderManager) renderWithImages(req *RenderRequest, prepared map[string]*preparedImage) []string {
	seedKittySessionFromCache()

	currentRenderImageMux.Lock()
	currentRenderImageDims = make(map[uint32]struct{ cols, rows int })
	currentRenderImageURLs = make(map[uint32]string)
	currentRenderImageOrder = currentRenderImageOrder[:0]
	currentRenderOrderIdx = 0
	currentRenderImageMux.Unlock()

	// Transmit in markdown order, so the ids line up with glamour's lookups
	for _, url := range extractImageURLs(req.Markdown) {
		imageID, cols, rows := transmitPreparedKittyImage(prepared[url], req.MaxCols-PREVIEW_RIGHT_PADDING)
		if imageID != 0 {
			currentRenderImageMux.Lock()
			currentRenderImageDims[imageID] = struct{ cols, rows int }{cols, rows}
			currentRenderImageOrder = append(currentRenderImageOrder, imageID)
			currentRenderImageURLs[imageID] = url
			currentRenderImageMux.Unlock()
		}
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath(getGlamourStylePath()),
		glamour.WithWordWrap(0),
		glamour.WithKittyImages(true, kittyImageCacheLookup),
	)
	if err != nil {
		return []string{"Error creating renderer: " + err.Error()}
	}
	note, err := r.Render(req.Markdown)
	if err != nil {
		return []string{"Error rendering: " + err.Error()}
	}

	// Replace glamour's text markers with actual Unicode placeholder grids
	note = replaceKittyImageMarkers(note)
	return rm.finishRender(note, req.MaxCols)
}
