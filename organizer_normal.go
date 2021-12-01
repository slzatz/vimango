package main

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

var n_lookup = map[string]func(){
	"G":                        _G,
	"gg":                       _gg,
	"m":                        mark,
	"*":                        _asterisk,
	"n":                        _n,
	":":                        exCmd,
	string(ctrlKey('l')):       switchToEditorMode,
	string([]byte{0x17, 0x17}): switchToEditorMode,
	string(0x4):                del,           //ctrl-d
	string(0x2):                starEntry,     //ctrl-b -probably want this go backwards (unimplemented) and use ctrl-e for this
	string(0x18):               completeEntry, //ctrl-x
	string(ctrlKey('i')):       entryInfo,     //{{0x9}}
	string(ctrlKey('j')):       controlJ,
	string(ctrlKey('k')):       controlK,
	string(ctrlKey('z')):       controlZ,
	string(ctrlKey('n')):       drawPreviewWithImages,
	" m":                       drawPreviewWithImages,
}

func _gg() {
	org.fc = 0
	org.fr = 0
	org.rowoff = 0
	//org.fr = org.repeat - 1 //this needs to take into account O.rowoff
	if org.view == TASK {
		org.altRowoff = 0
		sess.imagePreview = false
		org.readTitleIntoBuffer() /////////////////////////////////////////////
		org.drawPreview()
	} else {
		c := getContainerInfo(org.rows[org.fr].id)
		if c.id != 0 {
			sess.displayContainerInfo(&c)
			sess.drawPreviewBox()
		}
	}
}

func _G() {
	org.fc = 0
	org.fr = len(org.rows) - 1
	if org.view == TASK {
		org.altRowoff = 0
		sess.imagePreview = false
		org.readTitleIntoBuffer() /////////////////////////////////////////////
		org.drawPreview()
	} else {
		c := getContainerInfo(org.rows[org.fr].id)
		if c.id != 0 {
			sess.displayContainerInfo(&c)
			sess.drawPreviewBox()
		}
	}
}

func exCmd() {
	sess.showOrgMessage(":")
	org.command_line = ""
	org.last_mode = org.mode //at the least picks up NORMAL and NO_ROWS
	org.mode = COMMAND_LINE
}

/*
func _v() {
	org.mode = VISUAL
	org.highlight[0] = org.fc
	org.highlight[1] = org.fc
	sess.showOrgMessage("\x1b[1m-- VISUAL --\x1b[0m")
}

func _p() {
	if len(org.string_buffer) > 0 {
		org.pasteString()
	}
}
*/

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

func completeEntry() {
	toggleCompleted()
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
		glamour.WithStylePath("/home/slzatz/listmango/darkslz.json"),
		glamour.WithWordWrap(0),
		glamour.WithLinkNumbers(true),
	)
	note, _ = r.Render(note)
	// glamour seems to add a '\n' at the start
	note = strings.TrimSpace(note)
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
