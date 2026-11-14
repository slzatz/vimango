package pane

import (
	"github.com/slzatz/vimango/terminal"
	"github.com/slzatz/vimango/vim"
)

// Keys that produce no character -- the arrows, Home, the function keys --
// arrive from the key reader as synthetic codes well above any rune a
// terminal will send (terminal.KeyArrowLeft and up).
//
// IsSyntheticKey is what keeps that safe. Only the codes VimKeyName knows mean
// anything to vim; every other synthetic code has to be dropped rather than
// forwarded, because string(rune(c)) on one of them is a Greek letter, not a
// keystroke. A key added to the terminal package is therefore inert by
// default instead of inserting mojibake into a note.

const backspace = 127 // a real byte, unlike the reader's codes

var termcodes = map[int]string{
	terminal.KeyArrowUp:    "<up>",
	terminal.KeyArrowDown:  "<down>",
	terminal.KeyArrowRight: "<right>",
	terminal.KeyArrowLeft:  "<left>",
	backspace:              "<bs>", //? also works "\x08"
	terminal.KeyHome:       "<home>",
	terminal.KeyDelete:     "<del>",
	terminal.KeyPageUp:     "<pageup>",
	terminal.KeyPageDown:   "<pagedown>",
}

// VimKeyName is vim's name for a key that has to be sent to it by name
// ("<up>", "<bs>"): the synthetic keys vim understands, and Backspace. ok is
// false for every other key.
func VimKeyName(c int) (name string, ok bool) {
	name, ok = termcodes[c]
	return name, ok
}

// IsSyntheticKey reports whether c is one of the reader's codes rather than a
// character the user actually typed.
func IsSyntheticKey(c int) bool { return c >= terminal.KeyArrowLeft }

// SendToVim forwards one key. VimKeyName names the synthetic keys vim
// understands; any other synthetic code is dropped, since vim would insert its
// code point as text.
func SendToVim(c int) {
	if z, found := termcodes[c]; found {
		vim.SendKey(z)
		return
	}
	if IsSyntheticKey(c) {
		return
	}
	vim.SendInput(string(rune(c)))
}
