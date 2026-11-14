package editor

import "github.com/slzatz/vimango/vim"

// Mode is what the editor is doing with the keys it is given. Most of the
// time that is vim's own mode, under the editor's name for it (modeMap); the
// rest are the editor's: a ':' line whose Enter it takes for itself
// (EX_COMMAND), the list a spelling suggestion is picked from (SPELLING), and
// a notice over the text (HELP).
//
// The organizer has modes of its own, in a type of its own: until 2026-11-14
// the two shared one.
type Mode int

const (
	NORMAL      Mode = iota // just seeing this after an escape
	NORMAL_BUSY             // Replace and almost any other keystroke in normal
	INSERT
	EX_COMMAND // a ':' command line
	VISUAL_LINE
	VISUAL
	VISUAL_BLOCK
	SEARCH   // any other command line: '/', '?', an expression
	SPELLING // choosing a z= suggestion by number (spellcheck.go)
	HELP     // a notice is up: :help, or what a vim command listed
	PENDING  // half of an operator (the "d" of "d3w")
	OTHER    // Just in case
)

var modeMap = map[vim.Mode]Mode{
	vim.ModeNormal:     NORMAL, // seems to be only after an escape
	vim.ModeVisual:     VISUAL,
	vim.ModeOpPending:  PENDING,
	vim.ModeCmdline:    SEARCH, // also EX_COMMAND: syncCmdline says which
	vim.ModeInsert:     INSERT,
	vim.ModeNormalBusy: NORMAL_BUSY, // just about any keystroke when in NORMAL mode
}

// visualModeMap is the kind of selection, from what vim.GetVisualType reports:
// the character of the key that starts it (v, V, Ctrl-V).
var visualModeMap = map[int]Mode{
	22:  VISUAL_BLOCK,
	86:  VISUAL_LINE,
	118: VISUAL,
}

func (m Mode) String() string {
	return [...]string{
		"NORMAL",
		"NORMAL BUSY",
		"INSERT",
		"EX COMMAND",
		"VISUAL LINE",
		"VISUAL",
		"VISUAL BLOCK",
		"SEARCH",
		"SPELLING",
		"HELP",
		"PENDING",
		"OTHER",
	}[m]
}
