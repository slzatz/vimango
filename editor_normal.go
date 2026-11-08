package main

import "strings"

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
