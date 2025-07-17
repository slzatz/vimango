//go:build !cgo || windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// OpenNoteInWebview opens a note in the default system browser as fallback
func OpenNoteInWebview(title, htmlContent string) error {
	// Create a temporary HTML file
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, fmt.Sprintf("vimango_%s.html", title))
	
	// Write HTML content to temp file
	err := os.WriteFile(tempFile, []byte(htmlContent), 0644)
	if err != nil {
		return fmt.Errorf("failed to create temporary HTML file: %v", err)
	}
	
	// Open in default browser
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", tempFile)
	case "darwin":
		cmd = exec.Command("open", tempFile)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", tempFile)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
	
	return cmd.Start()
}

// IsWebviewAvailable returns false for non-CGO builds
func IsWebviewAvailable() bool {
	return false
}

// IsWebviewRunning returns false for non-CGO builds
func IsWebviewRunning() bool {
	return isWebviewRunning
}

// ShowWebviewUnavailableMessage shows a message when webview is not available
func ShowWebviewUnavailableMessage() {
	fmt.Println("Webview is not available in this build (requires CGO).")
	fmt.Println("Note will be opened in your default browser instead.")
}