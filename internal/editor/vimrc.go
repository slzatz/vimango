package editor

import (
	"os"

	"github.com/slzatz/vimango/vim"
)

// vimrcFile is vimango's vimrc: vimscript sourced into libvim once at startup.
// It is cwd-relative, like config.json and registers.json, so a host app's
// `vimango --editor` processes pick up the same file as the TUI.
//
// Mappings defined there apply only to keys the user types in the note editor
// (ProcessKey wraps its pane.SendToVim in vim.SendUserKey). vimango's own key
// sequences and the organizer's title editing run with expansion off.
//
// A left-hand side can be several keys ("jk", ",b"). vim holds the first
// until the next key shows whether the mapping is meant, or until 'timeoutlen'
// has passed -- which MainLoop times, libvim having no clock (MappingWait).
//
// Limits worth knowing when writing one:
//   - <esc> never reaches a mapping: vimango sends it to vim itself
const vimrcFile = "vimango.vim"

// SourceVimrc sources vimrcFile if it exists. A missing file is normal, and a
// file with errors must not stop startup, so any failure is silent; check a
// changed file with `vim -u vimango.vim` if a mapping does not take.
func SourceVimrc() {
	if _, err := os.Stat(vimrcFile); err != nil {
		return
	}
	vim.ExecuteCommand("silent! source " + vimrcFile)
}
