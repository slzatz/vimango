package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/slzatz/vimango/hunspell"
	"github.com/slzatz/vimango/vim"
)

func (a *App) setEditorNormalCmds() map[string]func(*Editor, int) {
	return map[string]func(*Editor, int){
		"\x17L":              (*Editor).moveOutputWindowRight,
		"\x17J":              (*Editor).moveOutputWindowBelow,
		"\x08":               (*Editor).moveLeft,         //Ctrl-H
		"\x0c":               (*Editor).moveRight,        //Ctrl-L
		"\x0a":               (*Editor).scrollOutputDown, //Ctrl-J
		"\x0b":               (*Editor).scrollOutputUp,   //Ctrl-K
		"\x02":               (*Editor).decorateWord,
		leader + "b":         (*Editor).decorateWord,
		"\x05":               (*Editor).decorateWord,
		string(ctrlKey('i')): (*Editor).decorateWord,
		"\x17=":              (*Editor).changeSplit,
		"\x17_":              (*Editor).changeSplit,
		"\x17-":              (*Editor).changeSplit,
		"\x17+":              (*Editor).changeSplit,
		"\x17>":              (*Editor).changeHSplit,
		"\x17<":              (*Editor).changeHSplit,
		leader + "m":         (*Editor).showMarkdownPreview,
		leader + "y":         (*Editor).nextStyle,
		leader + "t":         (*Editor).readGoTemplate,
		leader + "sp":        (*Editor).spellingCheck,
		leader + "su":        (*Editor).spellSuggest,
		//leader + "l": (*Editor).showVimMessageLog,
		//leader + "xx": (*Editor).test,
		//"z=": (*Editor).spellSuggest,
		string(ctrlKey('z')): (*Editor).switchImplementation,
	}
}

func (e *Editor) changeSplit(flag int) {
	if e.output == nil {
		return
	}

	op := e.output
	var outputHeight int
	if flag == '=' {
		outputHeight = e.Screen.textLines / 2
	} else if flag == '_' {
		outputHeight = 2
	} else if flag == '+' {
		outputHeight = op.screenlines - 1
	} else if flag == '-' {
		outputHeight = op.screenlines + 1
	} else {
		e.ShowMessage(BR, "flag = %v", flag)
		return
	}
	e.ShowMessage(BR, "flag = %v", flag)

	if outputHeight < 2 || outputHeight > e.Screen.textLines-3 {
		return
	}

	e.screenlines = e.Screen.textLines - outputHeight - 1
	op.screenlines = outputHeight
	op.top_margin = e.Screen.textLines - outputHeight + 2

	e.Screen.eraseRightScreen()
	e.Screen.drawRightScreen()
}

func (e *Editor) changeHSplit(flag int) {
	var width int
	if flag == '>' {
		width = e.Screen.screenCols - e.Screen.divider + 1
		app.moveDividerAbs(width)
	} else if flag == '<' {
		width = e.Screen.screenCols - e.Screen.divider - 1
		app.moveDividerAbs(width)
	} else {
		e.ShowMessage(BL, "flag = %v", flag)
		return
	}
}

func (e *Editor) moveOutputWindowRight(_ int) {
	if e.output == nil { // && e.is_subeditor && e.is_below) {
		return
	}
	//top_margin = TOP_MARGIN + 1;
	//screenlines = total_screenlines - 1;
	e.output.is_below = false

	e.Screen.positionWindows()
	e.Screen.eraseRightScreen()
	e.Screen.drawRightScreen()
	//editorSetMessage("top_margin = %d", top_margin);

}

func (e *Editor) moveOutputWindowBelow(_ int) {
	if e.output == nil { // && e.is_subeditor && e.is_below) {
		return
	}
	//top_margin = TOP_MARGIN + 1;
	//screenlines = total_screenlines - 1;
	e.output.is_below = true

	e.Screen.positionWindows()
	e.Screen.eraseRightScreen()
	e.Screen.drawRightScreen()
	//editorSetMessage("top_margin = %d", top_margin);
}

// should scroll output down
func (e *Editor) scrollOutputDown(_ int) {
	op := e.output
	if op == nil {
		e.command = ""
		return
	}
	if op.rowOffset < len(op.rows)-1 {
		op.rowOffset++
		op.drawText()
	}
	e.command = ""
}

// scroll output window up
func (e *Editor) scrollOutputUp(_ int) {
	if e.output == nil {
		e.command = ""
		return
	}
	if e.output.rowOffset > 0 {
		e.output.rowOffset--
	}
	e.output.drawText()
	e.command = ""
}

func (e *Editor) moveLeft(_ int) {
	// below "if" really for testing
	if e.isModified() {
		e.ShowMessage(BR, "Note you left has been modified")
	}

	if e.Session.numberOfEditors() == 1 {

		if e.Screen.divider < 10 {
			e.Screen.edPct = 80
			app.moveDividerPct(80)
		}
		e.Session.editorMode = false
		vim.SetCurrentBuffer(app.Organizer.vbuf)
		app.Organizer.drawPreview()
		app.Organizer.mode = NORMAL
		app.returnCursor()
		return
	}

	eds := e.Session.editors()
	index := 0
	for i, ed := range eds {
		if ed == e {
			index = i
			break
		}
	}

	e.ShowMessage(BL, "index: %d; length: %d", index, len(eds))

	if index > 0 {
		ae := eds[index-1]
		vim.SetCurrentBuffer(ae.vbuf)
		ae.mode = NORMAL
		e.Session.activeEditor = ae
		return
	} else {

		if e.Screen.divider < 10 {
			e.Screen.edPct = 80
			app.moveDividerPct(80)
		}
		e.Session.editorMode = false
		vim.SetCurrentBuffer(app.Organizer.vbuf)
		app.Organizer.drawPreview()
		app.Organizer.mode = NORMAL
		app.returnCursor()
		return
	}
}

func (e *Editor) moveRight(_ int) {
	// below "if" really for testing
	if e.isModified() {
		e.ShowMessage(BR, "Note you left has been modified")
	}

	eds := e.Session.editors()
	index := 0
	for i, z := range eds {
		if z == e {
			index = i
			break
		}
	}
	e.ShowMessage(BR, "index: %d; length: %d", index, len(eds))

	if index < len(eds)-1 {
		ae := eds[index+1]
		ae.mode = NORMAL
		vim.SetCurrentBuffer(ae.vbuf)
		e.Session.activeEditor = ae
	}

	return
}

// for VISUAL mode
func (e *Editor) decorateWordVisual(c int) {
	if len(e.ss) == 0 {
		return
	}

	if e.highlight[0][0] != e.highlight[1][0] {
		e.ShowMessage(BR, "The text must all be in the same row")
		return
	}

	row := e.ss[e.highlight[0][0]-1]
	beg, end := e.highlight[0][1], e.highlight[1][1]

	var undo bool
	var s string
	// in VISUAL mode like INSERT mode, the cursor can go beyond end of row
	if len(row) == end {
		s = row[beg:end]
	} else {
		s = row[beg : end+1]
	}
	e.ShowMessage(BR, "end = %d", end)
	if strings.HasPrefix(s, "**") {
		if c == ctrlKey('b') {
			undo = true
		}
	} else if s[0] == '*' {
		if c == ctrlKey('i') {
			undo = true
		}
	} else if s[0] == '`' {
		if c == ctrlKey('e') {
			undo = true
		}
	}
	s = strings.Trim(s, "*`")
	if undo {
		/*
			v.SetBufferText(e.vbuf, e.fr, beg, e.fr, end, [][]byte{word})
			v.SetWindowCursor(w, [2]int{e.fr + 1, beg}) //set screen cx and cy from pos
		*/
		vim.SendMultiInput("xi" + s + "\x1b")
		return
	}

	// Definitely weird and needs to be looked at again but lose space at end of row
	var space string
	if len(row) >= end-1 {
		space = " "
	}
	switch c {
	case ctrlKey('b'):
		s = fmt.Sprintf("%s**%s**", space, s)
	case ctrlKey('i'):
		s = fmt.Sprintf("%s*%s*", space, s)
	case ctrlKey('e'):
		s = fmt.Sprintf("%s`%s`", space, s)
	}

	vim.SendMultiInput("xi" + s + "\x1b")
	/*
		v.SetBufferText(e.vbuf, e.fr, beg, e.fr, end, [][]byte{[]byte(newText)})
		v.SetWindowCursor(w, [2]int{e.fr + 1, beg}) //set screen cx and cy from pos
	*/
}

func (e *Editor) decorateWord(c int) {
	if len(e.ss) == 0 {
		return
	}

	if e.ss[e.fr][e.fc] == ' ' {
		return
	}

	vim.ExecuteCommand("let cword = expand('<cword>')")
	w := vim.EvaluateExpression("cword")

	if w == "" {
		return
	}

	var undo bool
	if strings.HasPrefix(w, "**") {
		if c == ctrlKey('b') || c == 'b' {
			undo = true
		}
	} else if w[0] == '*' {
		if c == ctrlKey('i') || c == 'i' {
			undo = true
		}
	} else if w[0] == '`' {
		if c == ctrlKey('e') || c == 'e' {
			undo = true
		}
	}
	w = strings.Trim(w, "*`")
	if undo {
		vim.SendMultiInput("ciw" + w + "\x1b")
		return
	}

	switch c {
	case ctrlKey('b'), 'b':
		w = fmt.Sprintf("**%s**", w)
	case ctrlKey('i'), 'i':
		w = fmt.Sprintf("*%s*", w)
	case ctrlKey('e'), 'e':
		w = fmt.Sprintf("`%s`", w)
	}
	vim.SendInput("ciw" + w + "\x1b")
}

func (e *Editor) showMarkdownPreview(_ int) {
	if len(e.ss) == 0 {
		return
	}
	note := e.generateWWStringFromBuffer2()
	r, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("darkslz.json"),
		glamour.WithWordWrap(0),
	)
	note, _ = r.Render(note)
	note = WordWrap(note, e.Screen.totaleditorcols)
	note = strings.TrimSpace(note)
	e.renderedNote = note
	e.mode = PREVIEW
	e.previewLineOffset = 0
	e.drawPreview()
}

func (e *Editor) nextStyle(_ int) {
	e.Session.styleIndex++
	if e.Session.styleIndex > len(e.Session.style)-1 {
		e.Session.styleIndex = 0
	}
	e.ShowMessage(BR, "New style is %q", e.Session.style[e.Session.styleIndex])
}

func (e *Editor) readGoTemplate(_ int) {
	e.readFileIntoNote("go.template")
}

func (e *Editor) spellingCheck(_ int) {
	/* Really need to look at this and decide if there will be a spellcheck flag in NORMAL mode */
	if e.isModified() {
		e.ShowMessage(BR, "%sYou need to write the note before highlighting text%s", RED_BG, RESET)
		return
	}
	e.highlightMispelledWords()
}

func (e *Editor) spellSuggest(_ int) {
	h := hunspell.Hunspell("/usr/share/hunspell/en_US.aff", "/usr/share/hunspell/en_US.dic")
	w := vim.EvaluateExpression("expand('<cword>')")
	if ok := h.Spell(w); ok {
		e.ShowMessage(BR, "%q is spelled correctly", w)
		return
	}
	s := h.Suggest(w)
	e.ShowMessage(BR, "%q -> %s", w, strings.Join(s, "|"))
}

func (e *Editor) switchImplementation(_ int) {
	// Toggle between C and Go implementations
	currentImpl := vim.GetActiveImplementation()
	if currentImpl == vim.ImplC {
		// Switch to Go implementation
		e.ShowMessage(BL, "Switching to Go implementation")
		vim.SwitchToGoImplementation()
	} else {
		// Switch to C implementation
		e.ShowMessage(BL, "Switching to C implementation")
		vim.SwitchToCImplementation()
	}
}
