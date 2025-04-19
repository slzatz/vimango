package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/slzatz/vimango/rawmode"
	"github.com/slzatz/vimango/vim"
)

// Global app struct
var app *App

func main() {
	app = CreateApp()

	// Initialize Vim
	vim.Init(0)
	
	// Initialize database connections
	err := app.InitDatabases("config.json")
	if err != nil {
		log.Fatal(err)
	}

	// Set up signal handling for terminal resize
	signal_chan := make(chan os.Signal, 1)
	signal.Notify(signal_chan, syscall.SIGWINCH)
	
	go func() {
		for {
			_ = <-signal_chan
			app.signalHandler() 
		}
	}()
	
	// Configure Vim settings
	vim.Execute("set iskeyword+=*")
	vim.Execute("set iskeyword+=`")
	
	// Enable raw mode
	origCfg, err := rawmode.Enable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error enabling raw mode: %v", err)
		os.Exit(1)
	}
	
	app.origTermCfg = origCfg
	app.Session.editorMode = false
	
	// Get window size
	err = app.Screen.GetWindowSize()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting window size: %v", err)
		os.Exit(1)
	}
	
	app.InitApp()
	app.LoadInitialData()
	app.Run = true
	app.MainLoop()
}
