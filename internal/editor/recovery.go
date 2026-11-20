package editor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/slzatz/vimango/internal/atomicfile"
)

// The recovery file: vim's swap file, for a note.
//
// An editor that stops without :q or :x -- a panic, a crash in libvim, its
// host app crashing or killed, the machine losing power -- takes its unsaved
// text with it. So while a note is open the editor keeps a file for it in
// Host.RecoveryDir, named <note id>.<pid>.json:
//
//	{"id": 5, "pid": 1234, "title": "...", "written": "2026-11-20 09:50:00",
//	 "base": "<sha256 of the note as the database held it>",
//	 "modified": true, "text": "<the buffer>"}
//
// It is written when the note opens, after :w, and soon after an edit (the
// app's event loop says when: RecoveryPending, WriteRecovery); "text" is the
// buffer when it differs from the database, and empty when it does not. It
// is removed when the editor closes its note (closeNote), and by nothing
// else. So a file whose process is gone is an editor that ended some other
// way, and a host that sees its editor's file still there after the process
// exited knows it did not end with :q or :x (vimango_hybrid; the pid is in
// the host report).
//
// The next editor opened on the note takes the newest such file that has
// text (adoptRecovery): the buffer holds the recovered text, modified, and
// the message line says so; :w keeps it, :q! drops it. Files of processes
// still running are another editor's, and are left alone.

// recoveryState is what the editor knows of its recovery file.
type recoveryState struct {
	path    string // the file; "" when the host keeps none
	tick    int    // vim's change tick when it was last written; -1 before
	failed  bool   // the last write failed, and that has been said
	opening string // what to say once the editor is first drawn
	notice  string // ...and what to show in the notice box then: changes recovered
}

type recoveryFile struct {
	ID       int    `json:"id"`
	PID      int    `json:"pid"`
	Title    string `json:"title"`
	Written  string `json:"written"` // UTC, SQLite's format
	Base     string `json:"base"`    // sha256 of dbText, hex
	Modified bool   `json:"modified"`
	Text     string `json:"text"`
}

const recoveryTimeFormat = "2006-01-02 15:04:05"

// RecoveryPath is the recovery file of the editor of note id in process pid.
func RecoveryPath(dir string, id, pid int) string {
	return filepath.Join(dir, fmt.Sprintf("%d.%d.json", id, pid))
}

func textHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// processRunning reports whether pid is a process that is still there. One
// that is there but not ours to signal (EPERM) is running.
func processRunning(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// startRecovery is the recovery file for a note just opened: one left by an
// editor that ended without closing the note is taken into the buffer, and
// this editor's own file is written. Called by Open once the note is read in.
func (e *Editor) startRecovery() {
	e.recovery.tick = -1
	if e.RecoveryDir == "" {
		return
	}
	e.recovery.path = RecoveryPath(e.RecoveryDir, e.id, os.Getpid())
	if err := os.MkdirAll(e.RecoveryDir, 0o755); err != nil {
		e.recovery.opening = fmt.Sprintf("No recovery file for this note: %v", err)
		e.recovery.failed = true
		return
	}
	atomicfile.RemoveStale(e.RecoveryDir) // what a write cut short by a kill left
	found, leftPath := e.findRecovery()
	if found != nil {
		e.vbuf.SetLines(0, -1, strings.Split(found.Text, "\n"))
		e.resyncFromVim()
		// The text the recovered changes were made to is not the row's now:
		// a sync changed it since. :w puts the recovered text over that, as
		// after a :checkstale that could not reload.
		e.stale = found.Base != textHash(e.dbText)
		when := found.Written + " UTC"
		if t, err := time.ParseInLocation(recoveryTimeFormat, found.Written, time.UTC); err == nil {
			when = t.Local().Format("Jan 2 at 15:04")
		}
		notice := "## Unsaved changes recovered\n\n" +
			"An editor of this note stopped on " + when + " without saving, and what it had not saved is in the note now. " +
			"It is not in the database yet.\n\n" +
			"- `:w` or `:x` keeps it\n" +
			"- `:q!` drops it, and the note stays as the database has it\n\n"
		if e.stale {
			notice += "**The note has also changed in the database since then** (a sync, or another editor): " +
				"`:w` puts the recovered text over that version.\n\n"
		}
		e.recovery.notice = notice + "Esc closes this."
		e.recovery.opening = "Unsaved changes recovered — :w keeps them, :q! drops them"
	}
	// this editor's file first, and only then the one it took over, so that
	// the text is on disk throughout
	e.WriteRecovery()
	if found != nil && !e.recovery.failed {
		os.Remove(leftPath)
	}
}

// findRecovery is the newest recovery file for this note that a process no
// longer running left with text in it, and its path. Files of such processes
// that hold no text are removed: they recover nothing. Files that cannot be
// read are left where they are, for a person to look at.
func (e *Editor) findRecovery() (*recoveryFile, string) {
	names, _ := filepath.Glob(filepath.Join(e.RecoveryDir, strconv.Itoa(e.id)+".*.json"))
	var newest *recoveryFile
	var newestPath string
	for _, name := range names {
		pidText := strings.TrimSuffix(strings.TrimPrefix(filepath.Base(name), strconv.Itoa(e.id)+"."), ".json")
		pid, err := strconv.Atoi(pidText)
		if err != nil || pid == os.Getpid() || processRunning(pid) {
			continue
		}
		data, err := os.ReadFile(name)
		if err != nil {
			continue
		}
		var f recoveryFile
		if err := json.Unmarshal(data, &f); err != nil || f.ID != e.id {
			continue
		}
		if !f.Modified {
			os.Remove(name)
			continue
		}
		if newest == nil || f.Written > newest.Written {
			newest, newestPath = &f, name
		}
	}
	return newest, newestPath
}

// RecoveryPending reports whether the buffer has changed since the recovery
// file was last written. The app's event loop asks after every key, and
// calls WriteRecovery a moment later.
func (e *Editor) RecoveryPending() bool {
	return e.recovery.path != "" && e.vbuf.GetLastChangedTick() != e.recovery.tick
}

// WriteRecovery writes the recovery file as the buffer is now: its text if it
// is not the database's, and none if it is. A failure is said once, on the
// message line, and again only after a write has worked.
func (e *Editor) WriteRecovery() {
	if e.recovery.path == "" {
		return
	}
	tick := e.vbuf.GetLastChangedTick()
	f := recoveryFile{
		ID:       e.id,
		PID:      os.Getpid(),
		Title:    e.title,
		Written:  time.Now().UTC().Format(recoveryTimeFormat),
		Base:     textHash(e.dbText),
		Modified: e.isModified(),
	}
	if f.Modified {
		f.Text = strings.Join(e.vbuf.Lines(), "\n")
	}
	data, err := json.Marshal(f)
	if err == nil {
		err = atomicfile.Write(e.recovery.path, data, 0o600)
	}
	if err != nil {
		if !e.recovery.failed {
			e.recovery.failed = true
			msg := fmt.Sprintf("Could not write the recovery file: %v", err)
			if e.ScreenLines == 0 {
				e.recovery.opening = msg // not laid out yet: said when first drawn
			} else {
				e.ShowAside("%s", msg)
			}
		}
		return
	}
	e.recovery.failed = false
	e.recovery.tick = tick
}

// endRecovery removes the recovery file: the note is closed, with its text in
// the database or thrown away on purpose. Its absence is what tells a host
// that the editor ended with :q or :x.
func (e *Editor) endRecovery() {
	if e.recovery.path == "" {
		return
	}
	if err := os.Remove(e.recovery.path); err != nil && !os.IsNotExist(err) {
		e.ShowAside("Could not remove the recovery file: %v", err)
	}
	e.recovery.path = ""
}
