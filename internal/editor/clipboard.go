package editor

import (
	"strings"

	"github.com/slzatz/vimango/internal/pane"
	"github.com/slzatz/vimango/vim"
)

// The system clipboard. vim's registers belong to one process, and each note
// being edited in vimango_hybrid is a process of its own, so text yanked in
// one note was out of reach of another that was already open (registers.json
// carries the registers from one run to the next, not between two that are
// running). The clipboard is what the two have in common, and what every
// other program has too:
//
//   - a yank puts its text there: y, Y, yy, a yank in VISUAL, ":yank";
//   - "+ and "* are the clipboard: "+p puts what is there, from whichever
//     program, "+yy and "+dd write to it, CTRL-R + inserts it.
//
// A delete does not go there (x, dd, cw: text on its way out would replace
// what was copied on purpose, and a clipboard has no "0 to fall back on), and
// neither does a yank into a register that was named ("ayy), which says where
// the text is to go. To cut to the clipboard, name it: "+dd.
//
// "p" stays vim's: it puts the register written last, not the clipboard, so
// text copied in another program arrives with "+p or with the terminal's own
// paste (PasteText).

// Clipboard is the system clipboard, as the editor uses it:
// internal/clipboard's Board in the application, a stand-in in the tests.
type Clipboard interface {
	// Text is the text on the clipboard, and whether there is any.
	Text() (text string, ok bool)
	// SetText puts text there in place of what is, and reports whether it did.
	SetText(text string) bool
}

// UseClipboard connects vim's yanks and its "+ and "* registers to the
// clipboard, for every note opened in this process. Called once at startup
// (it is the one vim's, like RestoreRegisters); nil disconnects them.
func UseClipboard(c Clipboard) {
	if c == nil {
		vim.SetYankCallback(nil)
		vim.SetClipboardCallback(nil)
		return
	}
	vim.SetYankCallback(func(y vim.Yank) {
		if goesToClipboard(y) {
			// a failure is not reported: the text is in vim's register all
			// the same, and this runs in the middle of a key
			c.SetText(yankText(y))
		}
	})
	vim.SetClipboardCallback(func(byte) ([]string, bool, bool) {
		text, ok := c.Text()
		if !ok {
			return nil, false, false
		}
		return clipboardLines(text)
	})
}

// goesToClipboard is the rule above: a yank with no register named, and
// whatever is written to the clipboard registers by name.
func goesToClipboard(y vim.Yank) bool {
	switch y.Register {
	case '+', '*':
		return true
	case 0, '"':
		return y.Op == 'y'
	}
	return false
}

// yankText is a yank as text for the clipboard: whole lines each end in a
// line end, which is how the note that takes them knows they were whole
// lines (clipboardLines). A block goes as its rows, one to a line.
func yankText(y vim.Yank) string {
	text := strings.Join(y.Lines, "\n")
	if y.Type == vim.Linewise {
		text += "\n"
	}
	return text
}

// clipboardLines is the clipboard's text as a register: its lines, and
// whether they are whole lines, which they are when the text ends in a line
// end -- vim's own rule for text it did not put there itself. The clipboard
// holds no register type, so characters yanked up to and including a line
// end come back as lines, and a block as characters.
//
// The text is a paste (pane.PastedText): line ends are "\n" whatever the
// program that copied it wrote, and control characters are left out.
func clipboardLines(text string) (lines []string, linewise bool, ok bool) {
	text = pane.PastedText(text)
	if text == "" {
		return nil, false, false
	}
	linewise = strings.HasSuffix(text, "\n")
	return strings.Split(strings.TrimSuffix(text, "\n"), "\n"), linewise, true
}
