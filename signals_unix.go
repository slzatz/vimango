//go:build !windows

package main

import (
	"os"
	"os/signal"
	"syscall"
)

// setupSignalHandling sets up platform-specific signal handling
func setupSignalHandling(app *App) {
	// Set up signal handling for terminal resize
	signal_chan := make(chan os.Signal, 1)
	signal.Notify(signal_chan, syscall.SIGWINCH)

	go func() {
		for {
			_ = <-signal_chan
			app.signalHandler()
		}
	}()
}