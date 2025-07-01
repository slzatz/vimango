//go:build windows

package main

// setupSignalHandling sets up platform-specific signal handling
// On Windows, we don't handle SIGWINCH as it doesn't exist
func setupSignalHandling(app *App) {
	// Windows doesn't support SIGWINCH signal handling
	// Terminal resize detection would need alternative implementation
	// For now, this is a no-op on Windows
}