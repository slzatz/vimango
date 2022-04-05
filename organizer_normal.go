package main

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/slzatz/vimango/vim"
)

var n_lookup = map[string]func(){
	//"dd": noop,
	"dd":                       del,
	"m":                        mark,
	":":                        exCmd,
	string(ctrlKey('l')):       switchToEditorMode,
	string([]byte{0x17, 0x17}): switchToEditorMode,
	string(0x4):                del, //ctrl-d
	//string(0x2):                starEntry,     //ctrl-b -probably want this go backwards (unimplemented) and use ctrl-e for this
	string(0x1):          starEntry, //ctrl-b -probably want this go backwards (unimplemented) and use ctrl-e for this
	string(0x18):         archive,   //ctrl-x
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

func del() {
	toggleDeleted()
}

func starEntry() {
	toggleStar()
}

func archive() {
	toggleArchived()
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
	note := readNoteIntoString(id)
	note = generateWWString(note, org.totaleditorcols)
	r, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("darkslz.json"),
		glamour.WithWordWrap(0),
		glamour.WithLinkNumbers(true),
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
