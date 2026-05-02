// webview_worker is a subprocess binary that owns the macOS main thread for
// running the WebKit/Cocoa event loop. It receives line-delimited JSON
// commands on stdin and applies them via webview.Dispatch so they run on the
// UI thread.
//
// Build: cd cmd/webview_worker && CGO_ENABLED=1 go build -o ../../webview_worker
package main

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/webview/webview_go"
)

type command struct {
	Cmd     string `json:"cmd"`
	TitleB64 string `json:"title_b64,omitempty"`
	HTMLB64  string `json:"html_b64,omitempty"`
}

func logf(format string, args ...interface{}) {
	logPath := filepath.Join(os.TempDir(), "vimango_webview_worker.log")
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[%s] ", time.Now().Format(time.RFC3339))
	fmt.Fprintf(f, format, args...)
	fmt.Fprintln(f)
}

func main() {
	w := webview.New(false)
	defer w.Destroy()

	w.SetSize(1200, 800, webview.HintNone)
	w.SetTitle("Vimango")

	go readCommands(w)

	w.Run()
}

func readCommands(w webview.WebView) {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var c command
		if err := json.Unmarshal(line, &c); err != nil {
			logf("invalid command: %v", err)
			continue
		}
		switch c.Cmd {
		case "update":
			title, errT := base64.StdEncoding.DecodeString(c.TitleB64)
			html, errH := base64.StdEncoding.DecodeString(c.HTMLB64)
			if errT != nil || errH != nil {
				logf("base64 decode failed: title=%v html=%v", errT, errH)
				continue
			}
			titleStr := string(title)
			htmlStr := string(html)
			w.Dispatch(func() {
				w.SetTitle(fmt.Sprintf("Vimango - %s", titleStr))
				w.SetHtml(htmlStr)
			})
		case "close":
			w.Terminate()
			return
		default:
			logf("unknown command: %q", c.Cmd)
		}
	}
	if err := scanner.Err(); err != nil {
		logf("stdin scan error: %v", err)
	}
	w.Terminate()
}
