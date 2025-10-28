//go:build cgo && !windows

package main

import (
	"fmt"
	"log"
	"sync"

	"github.com/webview/webview_go"
)

var (
	// Mutex to ensure only one webview can run at a time
	webviewMutex sync.Mutex
	webviewRunning bool
	activeWebview webview.WebView
)

func init() {
	// Override the default - webview is available with CGO
	isWebviewAvailableDefault = true
}

// OpenNoteInWebview opens a note in a webview window or updates existing one
func OpenNoteInWebview(title, htmlContent string) error {
	// Check if webview is already running (with minimal mutex usage)
	webviewMutex.Lock()
	if webviewRunning && activeWebview != nil {
		// Update existing webview content using Dispatch to ensure thread safety
		// Dispatch runs the function on the webview's event loop thread
		w := activeWebview
		webviewMutex.Unlock()

		w.Dispatch(func() {
			w.SetTitle(fmt.Sprintf("Vimango - %s", title))
			w.SetHtml(htmlContent)
		})
		return nil
	}
	webviewRunning = true
	webviewMutex.Unlock()
	
	// Ensure we reset the flag when function exits
	defer func() {
		webviewMutex.Lock()
		webviewRunning = false
		activeWebview = nil
		webviewMutex.Unlock()
	}()
	
	w := webview.New(false) // false = not debug mode
	defer w.Destroy()

	// Store reference to active webview
	webviewMutex.Lock()
	activeWebview = w
	webviewMutex.Unlock()

	w.SetTitle(fmt.Sprintf("Vimango - %s", title))
	w.SetSize(1200, 800, webview.HintNone)
	
	// Load the HTML content directly
	w.SetHtml(htmlContent)
	
	// Run the webview (this blocks until window is closed)
	// Mutex is NOT held during this call, allowing other operations
	w.Run()
	
	return nil
}

// IsWebviewRunning returns true if a webview is currently running
func IsWebviewRunning() bool {
	webviewMutex.Lock()
	defer webviewMutex.Unlock()
	return webviewRunning
}

// ShowWebviewUnavailableMessage shows a message when webview is not available
func ShowWebviewUnavailableMessage() {
	log.Println("Webview is available in this build")
}