package main

import (
	"strings"
	"unicode"

	"github.com/slzatz/vimango/vim"
)

// Text pasted from the terminal.
//
// A terminal sends a paste down the same pipe as the keyboard. vimango asks
// for bracketed paste (terminal.BracketedPasteOn, in main), so the terminal
// marks where a paste begins and ends, and the key reader hands the text over
// whole (terminal.KeyPaste). Without the marks a paste was so many typed keys:
// 'autoindent' indented every line under the one before, a mapping fired on
// every key it was for (vimango.vim's <CR> gave each line of a pasted list a
// second marker), and in NORMAL mode the text ran as commands.
//
// Pasted text is text, wherever it lands: in the note at the cursor, on a
// command line, in a title. It is never a key.

// pastedText is a paste as text: its line ends are "\n" whatever the terminal
// sent (usually "\r"), and it holds no control character but those and tab.
// An <esc> in pasted text would otherwise end the insert, and what followed
// it would be commands.
func pastedText(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	return strings.Map(func(r rune) rune {
		if r != '\n' && r != '\t' && unicode.IsControl(r) {
			return -1
		}
		return r
	}, raw)
}

// onOneLine is pasted text for a place that holds one line -- a command line,
// a title: the line breaks and tabs inside it are spaces, and those at its
// ends are gone. A pasted command is never run by its own line end.
func onOneLine(text string) string {
	text = strings.Trim(text, "\n")
	return strings.NewReplacer("\n", " ", "\t", " ").Replace(text)
}

// pasteText puts a paste where the editor's cursor is and reports whether to
// redraw. What that means follows vim's own bracketed paste:
//
//   - INSERT (and Replace): the text goes in at the cursor, and the insert
//     goes on.
//   - on a command line (':', '/', '?', an expression): it goes on the line,
//     as one line. Nothing is run.
//   - VISUAL: the text replaces the selection.
//   - NORMAL, or anything half-typed there (an operator, a count, the
//     character "r" waits for): what was pending is dropped and the text is
//     put after the cursor as "a" would put it -- before the cursor in the
//     first column, as "i" -- and vim is back in NORMAL.
//   - :help and the spelling list take no text; the paste is dropped.
func (e *Editor) pasteText(raw string) (redraw bool) {
	text := pastedText(raw)
	if text == "" || e.mode == HELP || e.mode == SPELLING {
		return false
	}
	if vim.GetCurrentMode() == vim.ModeCmdline {
		for _, r := range onOneLine(text) {
			vim.SendInput(string(r))
		}
		e.syncCmdline()
		return e.redrawAfter(false) // a search highlights as its pattern grows
	}
	return e.redrawAfter(e.followVim(noKey, func() { pasteIntoVim(text) }))
}

// pasteIntoVim types text into the current buffer the way vim takes a paste:
// with 'paste' set, so that nothing is done to it -- no 'autoindent', no
// 'textwidth', no abbreviations -- and with mapping expansion off, as it is
// for every key vimango sends itself. Being typed, it is one insert: "u"
// takes it back and "." repeats it.
//
// libvim has vim's own code for this (bracketed_paste, behind <PasteStart>),
// but it reads the keys that follow from a terminal libvim does not have, and
// in the fork as it stands it crashes.
func pasteIntoVim(text string) {
	if vim.EvaluateExpression("&paste") != "1" {
		vim.ExecuteCommand("set paste")
		defer vim.ExecuteCommand("set nopaste")
	}
	mode := vim.GetCurrentMode()
	inserting := mode&vim.ModeInsert != 0 // Replace is INSERT with a flag
	if !inserting {
		if mode == vim.ModeVisual {
			vim.SendInput("c")
		} else {
			vim.SendKey("<esc>")
			if vim.GetCursorPosition()[1] == 0 {
				vim.SendInput("i")
			} else {
				vim.SendInput("a")
			}
		}
	}
	for _, r := range text {
		if r == '\n' {
			vim.SendInput("\r")
		} else {
			vim.SendInput(string(r))
		}
	}
	if !inserting {
		vim.SendKey("<esc>")
	}
}

// pasteText is a paste in the organizer, which holds text in two places: the
// title being edited (INSERT) and the command line. Each is one line. Anywhere
// else there is nothing to paste into, and the paste is dropped -- as typed
// keys it would have run as the organizer's commands.
func (o *Organizer) pasteText(raw string) RedrawScope {
	text := onOneLine(pastedText(raw))
	if text == "" {
		return RedrawNone
	}
	switch o.mode {
	case INSERT:
		redraw := RedrawNone
		for _, r := range text {
			redraw = o.InsertModeKeyHandler(int(r))
		}
		return redraw
	case COMMAND_LINE:
		o.command_line += text
		o.tabCompletion.index = 0
		o.tabCompletion.list = nil
		o.showMessage(":%s", o.command_line)
		return RedrawNone
	}
	o.showMessage("Nothing to paste into: i edits the title, : starts a command")
	return RedrawNone
}
