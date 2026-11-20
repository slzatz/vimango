package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/slzatz/vimango/internal/editor"
	"github.com/slzatz/vimango/rawmode"
	"github.com/slzatz/vimango/terminal"
)

// What becomes of the open note when vimango ends without :q or :x.
//
// The editor keeps a recovery file for its note while it is open (recovery.go
// in internal/editor), written a second after an edit. That alone survives
// anything, a crash inside libvim included, losing at most the last second.
// What is here closes the gap for the endings Go can see coming: a panic in
// the event loop, and a signal that is about to kill the process (SIGHUP when
// a host app or a terminal window goes away, SIGTERM). Each writes the file as
// the buffer is at that moment.

// defaultRecoveryDir holds the recovery files and the crash logs, relative to
// the working directory like everything else vimango keeps. main sets
// App.recoveryDir to it; tests leave that empty, which keeps no files, or
// give a directory of their own -- never this one, where a test's file could
// be taken for a real editor's.
const defaultRecoveryDir = "recovery"

// crashLogPath is where a panic in process pid is written up. A host that
// finds its editor's recovery file still there after the process ended looks
// here for why (vimango_hybrid).
func crashLogPath(dir string, pid int) string {
	return filepath.Join(dir, fmt.Sprintf("crash.%d.log", pid))
}

// crashed is MainLoop's last word on a panic: the open note's recovery file
// written as the buffer is now, the panic and its stack in a crash log, the
// terminal put back, and an exit that is not 0. It does not try to go on: the
// state the panic left is not to be trusted with a save.
func (a *App) crashed() {
	r := recover()
	if r == nil {
		return
	}
	stack := debug.Stack()
	kept := false
	if ae := a.Session.editor; ae != nil && a.recoveryDir != "" {
		kept = writeRecoveryAfterPanic(ae)
	}
	logPath, logErr := "", fmt.Errorf("no directory for a crash log")
	if a.recoveryDir != "" {
		logPath = crashLogPath(a.recoveryDir, os.Getpid())
		logErr = writeCrashLog(logPath, r, stack, a.Session.editor)
	}

	fmt.Print(terminal.BracketedPasteOff)
	if a.origTermCfg != nil {
		_ = rawmode.Restore(a.origTermCfg)
	}
	fmt.Fprintf(os.Stderr, "\r\nvimango stopped unexpectedly: %v\r\n", r)
	if kept {
		fmt.Fprintf(os.Stderr, "Unsaved changes were kept in %s/ and come back the next time the note is opened.\r\n", a.recoveryDir)
	}
	if logErr == nil {
		fmt.Fprintf(os.Stderr, "Details: %s\r\n", logPath)
	} else {
		fmt.Fprintf(os.Stderr, "%s\r\n", stack)
	}
	ShutdownHEICDecoder()
	os.Exit(2)
}

// writeRecoveryAfterPanic writes the editor's recovery file, and reports
// whether it got through: the panic may have come from the editor, and a
// second one here must not stop the crash log being written.
func writeRecoveryAfterPanic(ae *editor.Editor) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	ae.WriteRecovery()
	return true
}

func writeCrashLog(path string, r any, stack []byte, ae *editor.Editor) error {
	note := "no note open"
	if ae != nil {
		note = fmt.Sprintf("note %d", ae.ID())
	}
	text := fmt.Sprintf("vimango (pid %d) stopped at %s, %s\n\npanic: %v\n\n%s",
		os.Getpid(), time.Now().Format(time.RFC3339), note, r, stack)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(text), 0o600)
}

// keepNoteOnSignal is what a signal that ends vimango does first: the open
// note's recovery file written (by MainLoop, the one goroutine that may touch
// the editor), and the heic_worker taken down (killHEICWorkerOnSignal). If
// MainLoop is inside a command that has not returned, the wait gives up after
// a second and the file stays as it was last written.
func (a *App) keepNoteOnSignal() {
	beforeDyingOf(func() {
		a.flushRecovery(time.Second)
		ShutdownHEICDecoder()
	}, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGINT)
}

// flushRecovery asks MainLoop to write the open note's recovery file and
// waits for it, for no longer than limit. It reports whether it was written.
func (a *App) flushRecovery(limit time.Duration) bool {
	done := make(chan struct{})
	timeout := time.After(limit)
	select {
	case a.recoveryRequests <- done:
	case <-timeout:
		return false
	}
	select {
	case <-done:
		return true
	case <-timeout:
		return false
	}
}
