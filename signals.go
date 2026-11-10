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

// beforeDyingOf runs cleanup when one of the signals arrives, and then lets
// the signal kill the process as it would have: whoever is waiting on it
// still sees a death by that signal, not an exit code.
//
// For what a deferred call or the end of main cannot be trusted with, since a
// signal's default action runs neither.
func beforeDyingOf(cleanup func(), signals ...os.Signal) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, signals...)
	go func() {
		sig := <-ch
		cleanup()
		signal.Reset(sig)
		_ = syscall.Kill(os.Getpid(), sig.(syscall.Signal))
	}()
}

// killHEICWorkerOnSignal takes the heic_worker subprocess down with this
// process when it is killed rather than exits. The worker is not tied to its
// parent (see ShutdownHEICDecoder) and the normal exits kill it by hand, but
// vimango_hybrid stops a --render-html it no longer wants with SIGTERM --
// every time the selection moves off a note whose images are still loading --
// and closing a terminal window sends the TUI SIGHUP. Each of those left a
// worker behind, holding its 136MB for as long as the machine was up.
func killHEICWorkerOnSignal() {
	beforeDyingOf(ShutdownHEICDecoder, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)
}
