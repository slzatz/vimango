// Package pane is what vimango's two panes -- the organizer's list of notes
// and the editor -- both use, and neither owns: the registry that ex commands
// and their help are kept in, cutting text to fit a number of screen columns,
// how a key from the terminal is given to vim, and what pasted text is.
//
// The editor does not import the application around it and the application's
// own files are the organizer's, so what both need lives here.
package pane
