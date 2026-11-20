package editor

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/slzatz/vimango/internal/localdb"
	"github.com/slzatz/vimango/vim"
)

// recoveryEditor opens note text the way the app does (Open), with recovery
// files kept in a directory of the test's own. The editor is laid out, so
// that what it says can be read.
func recoveryEditor(t *testing.T, text string, prepare func(dir string, id int)) (*Editor, string) {
	t.Helper()
	vim.InitializeVim(0)
	vim.SendKey("<esc>")
	db := newTestStore(t)
	id, err := db.Add("a note", false, localdb.DefaultContextUUID, localdb.DefaultFolderUUID)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.SetNote(id, text); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if prepare != nil {
		prepare(dir, id)
	}
	h := &testHost{}
	e, err := Open(Host{Store: db, Notices: h, OnClose: func() { h.closed++ }, RecoveryDir: dir}, id, "")
	if err != nil {
		t.Fatal(err)
	}
	e.Layout = hostAppLayout
	return e, dir
}

func readRecovery(t *testing.T, path string) (recoveryFile, bool) {
	t.Helper()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return recoveryFile{}, false
	}
	if err != nil {
		t.Fatal(err)
	}
	var f recoveryFile
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return f, true
}

// deadPID is the pid of a process that has ended.
func deadPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command("true")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

func leaveRecovery(t *testing.T, dir string, f recoveryFile) string {
	t.Helper()
	data, _ := json.Marshal(f)
	path := RecoveryPath(dir, f.ID, f.PID)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// While a note is open its recovery file holds the buffer's text when that
// is not the database's, and nothing when it is; closing the note as it
// should be closed -- :x, :q, :q! -- removes it, and nothing else does.
func TestRecoveryFileFollowsTheBuffer(t *testing.T) {
	for _, close := range []string{"x", "q!"} {
		t.Run(close, func(t *testing.T) {
			e, dir := recoveryEditor(t, "alpha\nbravo", nil)
			path := RecoveryPath(dir, e.id, os.Getpid())
			if f, ok := readRecovery(t, path); !ok || f.Modified || f.Text != "" || f.ID != e.id || f.PID != os.Getpid() {
				t.Fatalf("opened: the recovery file is %+v (there: %v)", f, ok)
			}
			if e.RecoveryPending() {
				t.Error("opened: a write is pending with nothing changed")
			}

			captureStdout(t, func() { e.sendKeys("x") })
			if !e.RecoveryPending() {
				t.Fatal("an edit left no write pending")
			}
			captureStdout(t, e.WriteRecovery)
			if f, _ := readRecovery(t, path); !f.Modified || f.Text != "lpha\nbravo" {
				t.Errorf("after an edit: %+v", f)
			}
			if e.RecoveryPending() {
				t.Error("written, and still pending")
			}

			captureStdout(t, func() { e.exCommand("w") })
			if f, _ := readRecovery(t, path); f.Modified || f.Text != "" {
				t.Errorf("after :w: %+v", f)
			}

			captureStdout(t, func() {
				e.sendKeys("x")
				e.WriteRecovery()
				e.exCommand("q") // refused: unsaved changes
			})
			if f, ok := readRecovery(t, path); !ok || f.Text != "pha\nbravo" {
				t.Errorf("after a refused :q: %+v (there: %v)", f, ok)
			}

			captureStdout(t, func() { e.exCommand(close) })
			if hostOf(e).open() {
				t.Fatalf(":%s did not close the note", close)
			}
			if _, ok := readRecovery(t, path); ok {
				t.Errorf(":%s left the recovery file", close)
			}
		})
	}
}

// An editor opened on a note takes what an editor that stopped without
// closing it left: the text is in the buffer, modified, and said to be
// recovered; the file it came from gives way to this editor's own. :x puts it
// in the database.
func TestUnsavedChangesAreRecovered(t *testing.T) {
	var left string
	e, dir := recoveryEditor(t, "alpha\nbravo", func(dir string, id int) {
		left = leaveRecovery(t, dir, recoveryFile{
			ID: id, PID: deadPID(t), Title: "a note", Written: "2026-11-20 09:50:00",
			Base: textHash("alpha\nbravo"), Modified: true, Text: "alpha\nbravo\ncharlie",
		})
	})
	if got := strings.Join(e.vbuf.Lines(), "\n"); got != "alpha\nbravo\ncharlie" {
		t.Fatalf("the buffer holds %q", got)
	}
	if !e.Modified() || e.Stale() {
		t.Errorf("recovered: modified %v, stale %v; want true, false", e.Modified(), e.Stale())
	}
	if _, ok := readRecovery(t, left); ok {
		t.Error("the file recovered from is still there")
	}
	if f, _ := readRecovery(t, RecoveryPath(dir, e.id, os.Getpid())); f.Text != "alpha\nbravo\ncharlie" {
		t.Errorf("this editor's file holds %q", f.Text)
	}
	drawn := stripANSI(captureStdout(t, e.Draw))
	shown := hostOf(e).shown
	if len(shown) != 1 || !strings.Contains(shown[0], "Unsaved changes recovered") || strings.Contains(shown[0], "changed in the database") {
		t.Errorf("the notice shown: %q", shown)
	}
	if e.mode != HELP || !strings.Contains(drawn, "Unsaved changes recovered") {
		t.Errorf("mode %v; drawn %q", e.mode, drawn)
	}
	captureStdout(t, func() { e.ProcessKey(0x1b) }) // Esc ends the notice
	if drawn := stripANSI(captureStdout(t, e.Draw)); len(hostOf(e).shown) != 1 || strings.Contains(drawn, "recovered") {
		t.Errorf("the recovery is said again: %q", drawn)
	}
	captureStdout(t, func() { e.exCommand("x") })
	if text, _ := e.Store.Note(e.id); text != "alpha\nbravo\ncharlie" {
		t.Errorf("after :x the database holds %q", text)
	}
}

// Recovered changes made to a text the database no longer holds -- a sync
// changed the note after the editor stopped -- are recovered, and the note is
// stale, as after a :checkstale that could not reload.
func TestRecoveredOverANewerNoteIsStale(t *testing.T) {
	e, _ := recoveryEditor(t, "from a sync", func(dir string, id int) {
		leaveRecovery(t, dir, recoveryFile{
			ID: id, PID: deadPID(t), Written: "2026-11-20 09:50:00",
			Base: textHash("alpha"), Modified: true, Text: "alpha and more",
		})
	})
	if e.line(0) != "alpha and more" || !e.Stale() {
		t.Errorf("the buffer holds %q, stale %v", e.line(0), e.Stale())
	}
	captureStdout(t, e.Draw)
	if shown := hostOf(e).shown; len(shown) != 1 || !strings.Contains(shown[0], "changed in the database since") {
		t.Errorf("the recovery did not say the note changed since: %q", shown)
	}
}

// What is not to be recovered is not: a running process's file is another
// editor's and is left alone; a file with no text recovers nothing and goes;
// of two with text, the newer is taken and the other kept for the next open.
func TestWhatIsNotRecovered(t *testing.T) {
	var live, clean, older, newer string
	e, _ := recoveryEditor(t, "alpha", func(dir string, id int) {
		live = leaveRecovery(t, dir, recoveryFile{ID: id, PID: os.Getppid(), Written: "2026-11-20 10:00:00",
			Base: textHash("alpha"), Modified: true, Text: "another editor's"})
		clean = leaveRecovery(t, dir, recoveryFile{ID: id, PID: deadPID(t), Written: "2026-11-20 10:00:00",
			Base: textHash("alpha")})
		older = leaveRecovery(t, dir, recoveryFile{ID: id, PID: deadPID(t), Written: "2026-11-19 10:00:00",
			Base: textHash("alpha"), Modified: true, Text: "older"})
		newer = leaveRecovery(t, dir, recoveryFile{ID: id, PID: deadPID(t), Written: "2026-11-20 09:00:00",
			Base: textHash("alpha"), Modified: true, Text: "newer"})
		// another note's
		leaveRecovery(t, dir, recoveryFile{ID: id + 1, PID: deadPID(t), Written: "2026-11-20 11:00:00",
			Base: textHash("x"), Modified: true, Text: "another note"})
	})
	if e.line(0) != "newer" {
		t.Errorf("recovered %q, want the newer", e.line(0))
	}
	for path, want := range map[string]bool{live: true, clean: false, older: true, newer: false} {
		if _, there := readRecovery(t, path); there != want {
			t.Errorf("%s: there %v, want %v", filepath.Base(path), there, want)
		}
	}
}

// A directory the file cannot be written in is said, and the note opens.
func TestRecoveryThatCannotBeWritten(t *testing.T) {
	e, _ := recoveryEditor(t, "alpha", func(dir string, id int) {
		os.Chmod(dir, 0o500)
		t.Cleanup(func() { os.Chmod(dir, 0o700) })
	})
	if e.line(0) != "alpha" {
		t.Errorf("the note opened holding %q", e.line(0))
	}
	if drawn := stripANSI(captureStdout(t, e.Draw)); !strings.Contains(drawn, "Could not write the recovery file") {
		t.Errorf("the failure was not said: %q", drawn)
	}
}
