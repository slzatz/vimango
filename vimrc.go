package main

import (
	"os"

	"github.com/slzatz/vimango/vim"
)

// vimrcFile is vimango's vimrc: vimscript sourced into libvim once at startup.
// It is cwd-relative, like config.json and registers.json, so a host app's
// `vimango --editor` processes pick up the same file as the TUI.
//
// Mappings defined there apply only to keys the user types in the note editor
// (editorProcessKey wraps its sendToVim in vim.SendUserKey). vimango's own key
// sequences and the organizer's title editing run with expansion off.
//
// libvim limits worth knowing when writing one:
//   - single-key left-hand sides only: a partially matched multi-key mapping
//     ("jk", ",b") waits in the typeahead buffer and swallows keys
//   - no <C-r> in the insert-mode part of a right-hand side; call a function
//   - keys vimango intercepts before vim (Ctrl-H, Ctrl-L, and
//     <esc>, which it sends itself) never reach a mapping
const vimrcFile = "vimango.vim"

// sourceVimrc sources vimrcFile if it exists. A missing file is normal, and a
// file with errors must not stop startup, so any failure is silent; check a
// changed file with `vim -u vimango.vim` if a mapping does not take.
func sourceVimrc() {
	if _, err := os.Stat(vimrcFile); err != nil {
		return
	}
	vim.ExecuteCommand("silent! source " + vimrcFile)
}
