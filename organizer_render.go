package main

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/glamour"
)

// RenderRequest represents an async rendering request for a note
type RenderRequest struct {
	RequestID string // Unique: "note_<id>_<timestamp>"
	NoteID    int    // Database ID of note
	Markdown  string // Note content
	MaxCols   int    // Maximum columns for rendering
	CancelCh  chan struct{}
	CreatedAt time.Time
}

// RenderResult represents the result of a background render
type RenderResult struct {
	RequestID     string
	NoteID        int
	RenderedLines []string
	Success       bool
}

// RenderManager coordinates async rendering of notes
type RenderManager struct {
	app            *App
	organizer      *Organizer
	currentRequest *RenderRequest
	mutex          sync.RWMutex

	// Channel for render results
	resultCh chan RenderResult

	// Lifecycle
	running bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// NewRenderManager creates and initializes the render manager
func NewRenderManager(app *App) *RenderManager {
	rm := &RenderManager{
		app:       app,
		organizer: app.Organizer,
		resultCh:  make(chan RenderResult, 5),
		stopCh:    make(chan struct{}),
	}

	rm.start()
	return rm
}

// start initializes the result handler goroutine
func (rm *RenderManager) start() {
	rm.running = true

	// Start result handler
	rm.wg.Add(1)
	go rm.resultHandler()
}

// Stop shuts down the render manager
func (rm *RenderManager) Stop() {
	rm.running = false
	close(rm.stopCh)

	// Cancel current request if any
	rm.mutex.Lock()
	if rm.currentRequest != nil {
		select {
		case <-rm.currentRequest.CancelCh:
			// Already closed
		default:
			close(rm.currentRequest.CancelCh)
		}
	}
	rm.mutex.Unlock()

	// Wait for handlers with timeout
	done := make(chan struct{})
	go func() {
		rm.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Clean shutdown
	case <-time.After(2 * time.Second):
		// Timeout, force continue
	}

	close(rm.resultCh)
}

// StartRender initiates async rendering of a note
// It immediately renders text-only, then starts background full render with images
// If all images are already in kitty's session cache, it skips text-only and renders directly
func (rm *RenderManager) StartRender(noteID int, markdown string, maxCols int) {
	// Cancel any previous render
	rm.mutex.Lock()
	if rm.currentRequest != nil {
		select {
		case <-rm.currentRequest.CancelCh:
			// Already closed
		default:
			close(rm.currentRequest.CancelCh)
		}
	}

	// Create new render request
	req := &RenderRequest{
		RequestID: fmt.Sprintf("note_%d_%d", noteID, time.Now().UnixNano()),
		NoteID:    noteID,
		Markdown:  markdown,
		MaxCols:   maxCols,
		CancelCh:  make(chan struct{}),
		CreatedAt: time.Now(),
	}
	rm.currentRequest = req
	rm.mutex.Unlock()

	// Check if images are enabled
	if !app.kitty || !app.showImages {
		// No images mode - just render text
		textLines := rm.renderTextOnly(markdown, maxCols)
		rm.organizer.note = textLines
		rm.organizer.drawRenderedNote()
		return
	}

	// Check if there are any images in the markdown
	imageURLs := extractImageURLs(markdown)
	if len(imageURLs) == 0 {
		// No images in this note - just render text
		textLines := rm.renderTextOnly(markdown, maxCols)
		rm.organizer.note = textLines
		rm.organizer.drawRenderedNote()
		return
	}

	// Check if ALL images are in kitty's session cache
	// If so, full render will be fast and we can skip the text-only phase
	if rm.allImagesInKittyCache(imageURLs) {
		// Fast path: all images cached, render directly with images
		lines := rm.renderFullWithImages(req)
		if lines != nil {
			rm.organizer.note = lines
			rm.organizer.drawRenderedNote()
		}
		return
	}

	// Slow path: some images need loading
	// Phase 1: Render text-only immediately (no images)
	textLines := rm.renderTextOnly(markdown, maxCols)

	// Display text-only version immediately
	rm.organizer.note = textLines
	rm.organizer.drawRenderedNote()

	// Show status that images are loading
	rm.organizer.ShowMessage(BR, "Loading %d image(s)...", len(imageURLs))

	// Phase 2: Start background goroutine for full render with images
	go rm.backgroundRender(req)
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
	// Render markdown without kitty images
	options := []glamour.TermRendererOption{
		glamour.WithStylePath(getGlamourStylePath()),
		glamour.WithWordWrap(0),
		// Note: NOT enabling kitty images here
	}

	r, err := glamour.NewTermRenderer(options...)
	if err != nil {
		return []string{"Error creating renderer: " + err.Error()}
	}

	note, err := r.Render(markdown)
	if err != nil {
		return []string{"Error rendering: " + err.Error()}
	}

	note = strings.TrimSpace(note)

	// Handle search highlighting
	if rm.organizer.taskview == BY_FIND {
		note = strings.ReplaceAll(note, "qx", "\x1b[48;5;31m")
		note = strings.ReplaceAll(note, "qy", "\x1b[0m")
	}

	note = WordWrap(note, maxCols-PREVIEW_RIGHT_PADDING)

	return strings.Split(note, "\n")
}

// backgroundRender performs the full render with images in a background goroutine
func (rm *RenderManager) backgroundRender(req *RenderRequest) {
	// Check for cancellation before starting
	select {
	case <-req.CancelCh:
		return
	case <-rm.stopCh:
		return
	default:
	}

	// Perform the full render using existing renderMarkdown logic
	// This includes image loading, transmission, and proper placeholder sizing
	lines := rm.renderFullWithImages(req)

	// Check for cancellation before sending result
	select {
	case <-req.CancelCh:
		return
	case <-rm.stopCh:
		return
	default:
	}

	// Send result
	select {
	case rm.resultCh <- RenderResult{
		RequestID:     req.RequestID,
		NoteID:        req.NoteID,
		RenderedLines: lines,
		Success:       true,
	}:
	case <-req.CancelCh:
	case <-rm.stopCh:
	}
}

// renderFullWithImages performs the complete render with images
// This replicates the logic from renderMarkdown but returns lines instead of setting o.note
func (rm *RenderManager) renderFullWithImages(req *RenderRequest) []string {
	o := rm.organizer
	markdown := req.Markdown
	maxCols := req.MaxCols

	// Check cancellation
	select {
	case <-req.CancelCh:
		return nil
	default:
	}

	// Pre-transmit kitty images (same logic as renderMarkdown)
	if app.kitty && app.kittyPlace && app.showImages {
		seedKittySessionFromCache()

		// Clear the dimension map for this render
		currentRenderImageMux.Lock()
		currentRenderImageDims = make(map[uint32]struct{ cols, rows int })
		currentRenderImageURLs = make(map[uint32]string)
		nextImageLookupID = 1
		currentRenderImageOrder = currentRenderImageOrder[:0]
		currentRenderOrderIdx = 0
		currentRenderImageMux.Unlock()

		imageURLs := extractImageURLs(markdown)
		if len(imageURLs) > 0 {
			// Check cancellation before image loading
			select {
			case <-req.CancelCh:
				return nil
			default:
			}

			// Parallel prepare with bounded workers, then ordered transmit
			preparedMap := make(map[string]*preparedImage)
			type job struct{ url string }
			type result struct {
				prep *preparedImage
			}

			jobCh := make(chan job)
			resCh := make(chan result, len(imageURLs))
			var wg sync.WaitGroup

			workers := 6
			for i := 0; i < workers; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := range jobCh {
						// Check cancellation in worker
						select {
						case <-req.CancelCh:
							return
						default:
							resCh <- result{prep: prepareKittyImage(j.url)}
						}
					}
				}()
			}

			// Deduplicate URLs
			seen := make(map[string]bool)
			for _, url := range imageURLs {
				if !seen[url] {
					seen[url] = true
					jobCh <- job{url: url}
				}
			}
			close(jobCh)

			go func() {
				wg.Wait()
				close(resCh)
			}()

			for r := range resCh {
				if r.prep != nil {
					preparedMap[r.prep.url] = r.prep
				}
			}

			// Check cancellation before transmission
			select {
			case <-req.CancelCh:
				return nil
			default:
			}

			// Ordered transmit to keep kitty IDs aligned with markdown order
			for _, url := range imageURLs {
				prep := preparedMap[url]
				imageID, cols, rows := transmitPreparedKittyImage(prep, o.Screen.totaleditorcols-PREVIEW_RIGHT_PADDING)
				if imageID != 0 {
					currentRenderImageMux.Lock()
					currentRenderImageDims[imageID] = struct{ cols, rows int }{cols, rows}
					currentRenderImageOrder = append(currentRenderImageOrder, imageID)
					currentRenderImageURLs[imageID] = url
					currentRenderImageMux.Unlock()
				}
			}
		}
	}

	// Check cancellation before glamour render
	select {
	case <-req.CancelCh:
		return nil
	default:
	}

	// Configure renderer WITH kitty support
	options := []glamour.TermRendererOption{
		glamour.WithStylePath(getGlamourStylePath()),
		glamour.WithWordWrap(0),
	}

	if app.kitty && app.kittyPlace && app.showImages {
		options = append(options, glamour.WithKittyImages(true, kittyImageCacheLookup))
	}

	r, _ := glamour.NewTermRenderer(options...)
	note, _ := r.Render(markdown)

	// Replace glamour's text markers with actual Unicode placeholder grids
	if app.kitty && app.kittyPlace && app.showImages {
		note = replaceKittyImageMarkers(note)
	}

	note = strings.TrimSpace(note)

	// Handle search highlighting
	if o.taskview == BY_FIND {
		note = strings.ReplaceAll(note, "qx", "\x1b[48;5;31m")
		note = strings.ReplaceAll(note, "qy", "\x1b[0m")
	}

	note = WordWrap(note, maxCols-PREVIEW_RIGHT_PADDING)

	return strings.Split(note, "\n")
}

// resultHandler processes render results and updates the display
func (rm *RenderManager) resultHandler() {
	defer rm.wg.Done()

	for {
		select {
		case <-rm.stopCh:
			return
		case result, ok := <-rm.resultCh:
			if !ok {
				return
			}

			// Check if result is still relevant
			rm.mutex.RLock()
			current := rm.currentRequest
			rm.mutex.RUnlock()

			if current == nil || current.RequestID != result.RequestID {
				// User navigated away, discard result
				continue
			}

			// Check organizer is still on same note
			o := rm.organizer
			if len(o.rows) == 0 || o.rows[o.fr].id != result.NoteID {
				// User navigated away, discard
				continue
			}

			// Apply the full render result
			if result.Success && result.RenderedLines != nil {
				// Erase the right screen before displaying the new content
				o.Screen.eraseRightScreen()
				o.note = result.RenderedLines
				// Clear the loading message
				o.ShowMessage(BR, "")
				// Trigger redraw via notification
				rm.app.addNotification("_REDRAW_PREVIEW_")
			}
		}
	}
}

