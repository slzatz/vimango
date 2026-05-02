//go:build cgo && !windows

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
)

var (
	webviewMutex sync.Mutex
	workerCmd    *exec.Cmd
	workerStdin  io.WriteCloser
	workerPath   string
)

func init() {
	workerPath = locateWebviewWorker()
	isWebviewAvailableDefault = workerPath != ""
}

// locateWebviewWorker finds the webview_worker binary next to the main
// executable, then in the current directory. Returns "" if not found.
func locateWebviewWorker() string {
	execPath, err := os.Executable()
	if err == nil {
		candidate := filepath.Join(filepath.Dir(execPath), "webview_worker")
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if _, err := os.Stat("./webview_worker"); err == nil {
		return "./webview_worker"
	}
	return ""
}

// OpenNoteInWebview opens a note in the webview subprocess, spawning it on
// first call and updating its content on subsequent calls. Falls back to the
// system browser if the worker binary is unavailable.
func OpenNoteInWebview(title, htmlContent string) error {
	webviewMutex.Lock()
	defer webviewMutex.Unlock()

	if workerPath == "" {
		return openInBrowser(title, htmlContent)
	}

	if workerCmd != nil && workerStdin != nil {
		if err := sendUpdate(workerStdin, title, htmlContent); err == nil {
			return nil
		}
		// Write failed — worker likely died. Fall through and respawn.
		closeWorkerLocked()
	}

	cmd := exec.Command(workerPath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create webview worker stdin pipe: %v", err)
	}
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		stdin.Close()
		return fmt.Errorf("failed to start webview worker: %v", err)
	}

	workerCmd = cmd
	workerStdin = stdin

	if err := sendUpdate(stdin, title, htmlContent); err != nil {
		closeWorkerLocked()
		return fmt.Errorf("failed to send initial content to webview worker: %v", err)
	}

	go func(c *exec.Cmd) {
		c.Wait()
		webviewMutex.Lock()
		defer webviewMutex.Unlock()
		if workerCmd == c {
			workerCmd = nil
			if workerStdin != nil {
				workerStdin.Close()
				workerStdin = nil
			}
		}
	}(cmd)

	return nil
}

// IsWebviewRunning reports whether the worker subprocess is alive.
func IsWebviewRunning() bool {
	webviewMutex.Lock()
	defer webviewMutex.Unlock()
	return workerCmd != nil && workerCmd.ProcessState == nil
}

// CloseWebview asks the worker subprocess to close its window and exit.
func CloseWebview() error {
	webviewMutex.Lock()
	defer webviewMutex.Unlock()

	if workerCmd == nil || workerStdin == nil {
		return fmt.Errorf("no webview window is currently open")
	}

	payload, _ := json.Marshal(map[string]string{"cmd": "close"})
	payload = append(payload, '\n')
	if _, err := workerStdin.Write(payload); err != nil {
		// Best-effort: kill the process if we can't talk to it.
		if workerCmd.Process != nil {
			workerCmd.Process.Kill()
		}
	}
	return nil
}

// ShowWebviewUnavailableMessage is invoked when webview is requested but the
// worker binary is missing.
func ShowWebviewUnavailableMessage() {
	// No-op: caller falls back to the browser via OpenNoteInWebview.
}

func sendUpdate(stdin io.Writer, title, html string) error {
	payload, err := json.Marshal(map[string]string{
		"cmd":       "update",
		"title_b64": base64.StdEncoding.EncodeToString([]byte(title)),
		"html_b64":  base64.StdEncoding.EncodeToString([]byte(html)),
	})
	if err != nil {
		return err
	}
	payload = append(payload, '\n')
	_, err = stdin.Write(payload)
	return err
}

func closeWorkerLocked() {
	if workerStdin != nil {
		workerStdin.Close()
		workerStdin = nil
	}
	if workerCmd != nil && workerCmd.Process != nil {
		workerCmd.Process.Kill()
	}
	workerCmd = nil
}

// openInBrowser writes the rendered HTML to a temp file and opens it in the
// platform's default browser. Used when the webview_worker binary is missing.
func openInBrowser(title, htmlContent string) error {
	safe := sanitizeFilename(title)
	if safe == "" {
		safe = "note"
	}
	tempFile := filepath.Join(os.TempDir(), fmt.Sprintf("vimango_%s.html", safe))
	if err := os.WriteFile(tempFile, []byte(htmlContent), 0644); err != nil {
		return fmt.Errorf("failed to write temp HTML file: %v", err)
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", tempFile)
	case "linux":
		cmd = exec.Command("xdg-open", tempFile)
	default:
		return fmt.Errorf("unsupported platform for browser fallback: %s", runtime.GOOS)
	}
	return cmd.Start()
}

func sanitizeFilename(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
			out = append(out, c)
		case c == '-', c == '_':
			out = append(out, c)
		case c == ' ':
			out = append(out, '_')
		}
	}
	return string(out)
}
