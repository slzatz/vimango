package main

import (
	"github.com/slzatz/vimango/vim"
)

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
