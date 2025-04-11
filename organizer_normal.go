package main

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/slzatz/vimango/vim"
)

var n_lookup = map[string]func(){
	//"dd": noop,
	"dd":                       noop, //delete
	"m":                        mark,
	":":                        exCmd,
	string(ctrlKey('l')):       switchToEditorMode,
	string([]byte{0x17, 0x17}): switchToEditorMode,
	string(0x4):                noop, //ctrl-d delete
	//string(0x2):                starEntry,     //ctrl-b -probably want this go backwards (unimplemented) and use ctrl-e for this
	string(0x1):          starEntry, //ctrl-b -probably want this go backwards (unimplemented) and use ctrl-e for this
	string(0x18):         noop,   //ctrl-x archive
	string(ctrlKey('i')): entryInfo, //{{0x9}}
	string(ctrlKey('j')): controlJ,
	string(ctrlKey('k')): controlK,
	string(ctrlKey('z')): controlZ,
	string(ctrlKey('n')): drawPreviewWithImages,
	" m":                 drawPreviewWithImages,
}

func exCmd() {
	sess.showOrgMessage(":")
	org.command_line = ""
	org.last_mode = org.mode //at the least picks up NORMAL and NO_ROWS
	org.mode = COMMAND_LINE
}

func noop() {
	return
}

func _asterisk() {
	org.getWordUnderCursor()
	org.findNextWord()
}

func mark() {
	if org.view != TASK {
		sess.showOrgMessage("You can only mark tasks")
		return
	}

	if _, found := org.marked_entries[org.rows[org.fr].id]; found {
		delete(org.marked_entries, org.rows[org.fr].id)
	} else {
		org.marked_entries[org.rows[org.fr].id] = struct{}{}
	}
	sess.showOrgMessage("Toggle mark for item %d", org.rows[org.fr].id)
}

func _n() {
	org.findNextWord()
}

func (o *Organizer) del() {
  id := o.rows[o.fr].id
  state := o.rows[o.fr].deleted
	err := DB.toggleDeleted(id, state, o.view.String())
	if err != nil {
		o.showOrgMessage("Error toggling %s id %d to deleted: %v", o.view, id, err)
		return
  }
	o.rows[org.fr].deleted = !state
	o.showOrgMessage("Toggle deleted for %s id %d succeeded (new)", o.view, id)
}

func starEntry() {
	toggleStar()
}

func (o *Organizer) archive() {
  id := o.rows[o.fr].id
  state := o.rows[o.fr].archived
	err := o.Database.toggleArchived(id, state, o.view.String())
	if err != nil {
		o.showOrgMessage("Error toggling %s id %d to archived: %v", o.view, id, err)
		return
  }
	o.rows[o.fr].archived = !state
	o.showOrgMessage("Toggle archive for %s id %d succeeded (new)", o.view, id)
}

func entryInfo() {
	e := getEntryInfo(getId())
	sess.displayEntryInfo(&e)
	sess.drawPreviewBox()
}

func switchToEditorMode() {
	if len(windows) == 0 {
		sess.showOrgMessage("There are no active editors")
		return
	}
	sess.eraseRightScreen()
	sess.drawRightScreen()
	sess.editorMode = true
	vim.BufferSetCurrent(p.vbuf)
}

func controlJ() {
	//if len(org.note) > org.altRowoff+org.textLines {
	org.altRowoff++
	org.drawPreview()
	//}
}

func controlK() {
	if org.altRowoff > 0 {
		org.altRowoff--
		org.drawPreview()
	}
}

func controlZ() {
	id := org.rows[org.fr].id
	note := DB.readNoteIntoString(id)
	note = generateWWString(note, org.totaleditorcols)
	r, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("darkslz.json"),
		glamour.WithWordWrap(0),
		//glamour.WithLinkNumbers(true), //12312023 -- trying to use standard glamour
	)
	note, _ = r.Render(note)
	// glamour seems to add a '\n' at the start
	note = strings.TrimSpace(note)
	note = strings.ReplaceAll(note, "^^^", "\n") ///////////////04052022
	org.note = strings.Split(note, "\n")
	sess.eraseRightScreen()
	if !sess.imagePreview {
		org.drawPreviewWithoutImages()
	} else {
		org.drawPreviewWithImages()
	}
	org.mode = LINKS
	sess.showOrgMessage("\x1b[1mType a number to choose a link\x1b[0m")
}
func drawPreviewWithImages() {
	sess.eraseRightScreen()
	org.drawPreviewWithImages()
	sess.imagePreview = true
}
