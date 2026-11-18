package main

import (
	"strings"
	"testing"
	"time"

	"github.com/slzatz/vimango/internal/editor"
	"github.com/slzatz/vimango/terminal"
	"github.com/slzatz/vimango/vim"
)

// Keys and pastes all the way: from the key reader's channel, through the
// event loop, into the note and the database.

// The whole path, through the event loop: a key that begins a mapping, and
// then nothing. The loop's timer must end the wait by itself. "j" is mapped
// as well as "jk", so the timeout (mappings on) gives X where the <esc> that
// follows (mappings off) would have given a plain j.
func TestEventLoopTimesOutAHalfTypedMapping(t *testing.T) {
	if testing.Short() {
		t.Skip("waits on a timer")
	}
	a, ids := editorOnlyApp(t, "alpha")
	a.keyEvents = make(chan terminal.Key, 16)
	vim.ExecuteCommand("mapclear | mapclear!")
	vim.ExecuteCommand("inoremap j X")  // not joined with "|": the space
	vim.ExecuteCommand("inoremap jk Y") // before it would be part of the X
	vim.ExecuteCommand("set timeout timeoutlen=40")
	t.Cleanup(func() {
		vim.ExecuteCommand("mapclear | mapclear!")
		vim.ExecuteCommand("set timeout timeoutlen=1000")
	})

	captureStdout(t, func() {
		if err := a.openEditor(ids[0], ""); err != nil {
			t.Fatal(err)
		}
		go func() {
			for _, r := range "ij" {
				a.keyEvents <- terminal.Key{Regular: r}
			}
			time.Sleep(600 * time.Millisecond) // 'timeoutlen' passes
			for _, r := range "\x1b:x\r" {
				a.keyEvents <- terminal.Key{Regular: r}
			}
		}()
		a.handleEvents() // :x ends it: --editor has one note
	})

	if text, err := a.Store.Note(ids[0]); err != nil || text != "Xalpha" {
		t.Errorf("the note is %q (%v), want \"Xalpha\": the timeout did not fire", text, err)
	}
}

// The whole way: a paste from the key reader, through the event loop, into
// the note.
func TestEventLoopPastes(t *testing.T) {
	a, ids := editorOnlyApp(t, "- foo")
	a.keyEvents = make(chan terminal.Key, 64)
	vim.ExecuteCommand("mapclear | mapclear!")
	editor.SourceVimrc()
	t.Cleanup(func() { vim.ExecuteCommand("mapclear | mapclear!") })

	captureStdout(t, func() {
		if err := a.openEditor(ids[0], ""); err != nil {
			t.Fatal(err)
		}
		a.keyEvents <- terminal.Key{Regular: 'A'}
		a.keyEvents <- terminal.Key{Special: terminal.KeyPaste, Paste: "\r- bar\r  - baz"}
		for _, r := range "\x1b:x\r" {
			a.keyEvents <- terminal.Key{Regular: r}
		}
		a.handleEvents() // :x ends it: --editor has one note
	})

	if text, err := a.Store.Note(ids[0]); err != nil || text != "- foo\n- bar\n  - baz" {
		t.Errorf("the note is %q (%v)", text, err)
	}
}

// A mapping that types itself again never ends: the event loop is inside
// libvim for good. A Ctrl-C the key reader reads meanwhile stops it, the
// message line says so, and the note goes on as before -- here an x and :x,
// which save it.
func TestCtrlCInterruptsACommandThatNeverEnds(t *testing.T) {
	a, ids := editorOnlyApp(t, "alpha\nbravo")
	a.keyEvents = make(chan terminal.Key, 16)
	vim.ExecuteCommand("mapclear | mapclear!")
	vim.ExecuteCommand("nmap Q jkQ")
	t.Cleanup(func() { vim.ExecuteCommand("mapclear | mapclear!") })

	ctrlCKey := terminal.Key{Regular: ctrlC}
	if a.readerTakes(ctrlCKey) {
		t.Fatal("a Ctrl-C with nothing running was not passed on")
	}

	took := make(chan bool, 1)
	out := captureStdout(t, func() {
		if err := a.openEditor(ids[0], ""); err != nil {
			t.Fatal(err)
		}
		a.keyEvents <- terminal.Key{Regular: 'Q'}
		go func() {
			time.Sleep(200 * time.Millisecond) // the loop is inside Q
			took <- a.readerTakes(ctrlCKey)
			for _, r := range "gg0x:x\r" {
				a.keyEvents <- terminal.Key{Regular: r}
			}
		}()
		a.handleEvents() // :x ends it: --editor has one note
	})

	if !<-took {
		t.Error("the reader passed on a Ctrl-C while a command ran")
	}
	if !strings.Contains(out, "Interrupted") {
		t.Error("the message line did not say Interrupted")
	}
	if strings.Contains(out, ":qa") {
		t.Error("the interrupt was taken as a typed Ctrl-C")
	}
	if text, err := a.Store.Note(ids[0]); err != nil || text != "lpha\nbravo" {
		t.Errorf("the note is %q (%v), want \"lpha\\nbravo\"", text, err)
	}
}
