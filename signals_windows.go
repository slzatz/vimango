//go:build windows

package main

import (
	"time"

	"github.com/slzatz/vimango/rawmode"
)

// setupSignalHandling sets up platform-specific signal handling
// On Windows, we use polling to detect terminal resize since SIGWINCH doesn't exist
func setupSignalHandling(app *App) {
	// Start background goroutine to poll for terminal size changes
	go func() {
		// Get initial terminal size
		ws, err := rawmode.GetWindowSize()
		if err != nil {
			return // If we can't get initial size, skip resize detection
		}

		prevRows := ws.Row
		prevCols := ws.Col

		ticker := time.NewTicker(100 * time.Millisecond) // Poll every 100ms
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Check if application is still running
				if !app.Run {
					return
				}

				// Get current terminal size
				currentWs, err := rawmode.GetWindowSize()
				if err != nil {
					continue // Skip this iteration if we can't get size
				}

				// Check if size has changed
				if currentWs.Row != prevRows || currentWs.Col != prevCols {
					// Size changed, call the signal handler
					app.signalHandler()

					// Update previous size
					prevRows = currentWs.Row
					prevCols = currentWs.Col
				}
			}
		}
	}()
}
