//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

// setupSignalHandling turns terminal resizes (SIGWINCH) into events on
// app.resizeCh, which MainLoop handles. The goroutine here only forwards: it
// must not redraw, since drawing reaches libvim and UI state the main loop may
// be in the middle of using. Signals that arrive while one is already pending
// collapse into it -- the handler reads the size current when it runs.
func setupSignalHandling(app *App) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)

	go func() {
		for range sigCh {
			select {
			case app.resizeCh <- struct{}{}:
			default:
			}
		}
	}()
}
