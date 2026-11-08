package main

import (
	"strings"

	"github.com/slzatz/vimango/vim"
)

// The editor intercepts no normal-mode keys: every key reaches vim, and
// editing shortcuts are vim mappings in vimango.vim (see vimrc.go). Anything
// wanting an argument or a name is an ex command; see setEditorExCmds. What is
// left here are the bodies of some of those ex commands.

// splitExRange peels a leading ex range off a command line, returning the range
// (empty if there is none) and the remainder. vim writes "'<,'>" itself when
// ':' is pressed from VISUAL mode; "%" is the only other range vimango is
// likely to meet.
func splitExRange(line string) (rng, rest string) {
	const visual = "'<,'>"
	switch {
	case strings.HasPrefix(line, visual):
		return visual, line[len(visual):]
	case strings.HasPrefix(line, "%"):
		return "%", line[1:]
	}
	return "", line
}

func (e *Editor) spellingCheck(_ int) {
	/* Really need to look at this and decide if there will be a spellcheck flag in NORMAL mode */
	if !IsSpellCheckAvailable() {
		e.ShowMessage(BR, "%s", ShowSpellCheckNotAvailableMessage())
		return
	}

	if e.isModified() {
		e.ShowMessage(BR, "%sYou need to write the note before highlighting text%s", RED_BG, RESET)
		return
	}
	e.highlightMispelledWords()
}

func (e *Editor) spellSuggest(_ int) {
	if !IsSpellCheckAvailable() {
		e.ShowMessage(BR, "%s", ShowSpellCheckNotAvailableMessage())
		return
	}

	curPos := vim.GetCursorPosition()
	w, _, _ := GetWordAtIndex(e.ss[curPos[0]-1], curPos[1])
	//w := vim.EvaluateExpression("expand('<cword>')")

	if CheckSpelling(w) {
		e.ShowMessage(BR, "%q is spelled correctly", w)
		return
	}

	suggestions := GetSpellingSuggestions(w)
	e.ShowMessage(BR, "%q -> %s", w, strings.Join(suggestions, "|"))
}

// Ex-command forms of the normal-mode shortcuts. The registry wants
// func(*Editor); the normal-mode table wants func(*Editor, int).

func (e *Editor) spellCmd()   { e.spellingCheck(0) }
func (e *Editor) suggestCmd() { e.spellSuggest(0) }
