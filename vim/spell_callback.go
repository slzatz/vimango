package vim

// The z= callback lives in its own file: a file with //export may only have
// declarations in its preamble, and vim.go's includes libvim.h.

import "C"

// spellSuggestRequested is set when the user types z= without a count.
// libvim calls the callback synchronously from inside vimInput, so it runs on
// the goroutine that sent the key and needs no locking.
var spellSuggestRequested bool

//export goSpellSuggestCallback
func goSpellSuggestCallback() {
	spellSuggestRequested = true
}

// TakeSpellSuggestRequest reports whether z= was typed since the last call,
// and clears the request.
func TakeSpellSuggestRequest() bool {
	requested := spellSuggestRequested
	spellSuggestRequested = false
	return requested
}
