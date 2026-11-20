package main

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/slzatz/vimango/internal/editor"
	"github.com/slzatz/vimango/terminal"
)

// The event loop keeps the open note's recovery file a second behind the
// buffer, writes it at once when a signal that will end the process asks
// (flushRecovery), and :q! -- like any proper close -- removes it.
func TestEventLoopKeepsTheNoteForRecovery(t *testing.T) {
	if testing.Short() {
		t.Skip("waits for the one-second write")
	}
	a, ids := editorOnlyApp(t, "alpha")
	a.recoveryDir = t.TempDir()
	a.keyEvents = make(chan terminal.Key, 16)
	a.recoveryRequests = make(chan chan struct{})
	path := editor.RecoveryPath(a.recoveryDir, ids[0], os.Getpid())
	kept := func() (text string, there bool) {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", false
		}
		var f struct{ Text string }
		json.Unmarshal(data, &f)
		return f.Text, true
	}
	typed := func(s string) {
		for _, r := range s {
			a.keyEvents <- terminal.Key{Regular: r}
		}
		for len(a.keyEvents) > 0 {
			time.Sleep(5 * time.Millisecond)
		}
	}

	captureStdout(t, func() {
		if err := a.openEditor(ids[0], ""); err != nil {
			t.Fatal(err)
		}
		ended := make(chan struct{})
		go func() {
			a.handleEvents()
			close(ended)
		}()

		typed("x")
		if !a.flushRecovery(time.Second) {
			t.Error("the event loop did not answer a flush")
		}
		if text, _ := kept(); text != "lpha" {
			t.Errorf("flushed: the recovery file holds %q, want \"lpha\"", text)
		}

		typed("x")
		time.Sleep(1500 * time.Millisecond)
		if text, _ := kept(); text != "pha" {
			t.Errorf("a second after an edit: the recovery file holds %q, want \"pha\"", text)
		}

		typed(":q!\r")
		<-ended
	})
	if _, there := kept(); there {
		t.Error(":q! left the recovery file")
	}
}

// A panic in the event loop keeps the open note's unsaved text in its
// recovery file, writes the panic up in a crash log beside it, and ends the
// process with status 2. In a process of its own, since it ends one.
func TestPanicKeepsTheNote(t *testing.T) {
	if dir := os.Getenv("VIMANGO_TEST_PANIC_DIR"); dir != "" {
		a, ids := editorOnlyApp(t, "alpha")
		a.recoveryDir = dir
		e := openIn(t, a, ids[0])
		pressKeys(e, "x")
		func() {
			defer a.crashed()
			panic("boom")
		}()
		t.Fatal("crashed returned")
	}
	if testing.Short() {
		t.Skip("starts a process")
	}
	dir := t.TempDir()
	cmd := exec.Command(os.Args[0], "-test.run=^TestPanicKeepsTheNote$")
	cmd.Env = append(os.Environ(), "VIMANGO_TEST_PANIC_DIR="+dir)
	out, err := cmd.CombinedOutput()
	var exit *exec.ExitError
	if !errors.As(err, &exit) || exit.ExitCode() != 2 {
		t.Fatalf("the process ended with %v, want status 2; it said %s", err, out)
	}
	if !strings.Contains(string(out), "vimango stopped unexpectedly: boom") {
		t.Errorf("it said %q", out)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	if len(files) != 1 {
		t.Fatalf("recovery files: %v", files)
	}
	var f struct{ Text string }
	data, _ := os.ReadFile(files[0])
	json.Unmarshal(data, &f)
	if f.Text != "lpha" {
		t.Errorf("the recovery file holds %q, want \"lpha\"", f.Text)
	}
	logs, _ := filepath.Glob(filepath.Join(dir, "crash.*.log"))
	if len(logs) != 1 {
		t.Fatalf("crash logs: %v", logs)
	}
	if log, _ := os.ReadFile(logs[0]); !strings.Contains(string(log), "panic: boom") || !strings.Contains(string(log), "TestPanicKeepsTheNote") {
		t.Errorf("the crash log says %q", log)
	}
}

// A flush with no event loop to answer it gives up: a signal does not wait
// for ever on a loop stuck inside a command.
func TestFlushGivesUp(t *testing.T) {
	a := CreateApp()
	start := time.Now()
	if a.flushRecovery(50 * time.Millisecond) {
		t.Error("flushed with no event loop")
	}
	if time.Since(start) > time.Second {
		t.Error("the flush did not give up in time")
	}
}
