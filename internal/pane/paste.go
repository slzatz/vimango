package pane

import (
	"strings"
	"unicode"
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
// command line, in a title. It is never a key. What each pane does with one
// is its own pasteText; what a paste is, is here.

// PastedText is a paste as text: its line ends are "\n" whatever the terminal
// sent (usually "\r"), and it holds no control character but those and tab.
// An <esc> in pasted text would otherwise end the insert, and what followed
// it would be commands.
func PastedText(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	return strings.Map(func(r rune) rune {
		if r != '\n' && r != '\t' && unicode.IsControl(r) {
			return -1
		}
		return r
	}, raw)
}

// OnOneLine is pasted text for a place that holds one line -- a command line,
// a title: the line breaks and tabs inside it are spaces, and those at its
// ends are gone. A pasted command is never run by its own line end.
func OnOneLine(text string) string {
	text = strings.Trim(text, "\n")
	return strings.NewReplacer("\n", " ", "\t", " ").Replace(text)
}
