// Package clipboard is the system clipboard, as text: what Cmd-C and Cmd-V
// work on in every other program.
//
// There is one on macOS (the general pasteboard, reached through AppKit, with
// no program started). On other platforms System returns nil and vimango
// goes without.
package clipboard

// Board is a clipboard.
type Board struct {
	name string // the pasteboard's name; "" is the system's
}
