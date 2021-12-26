package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/slzatz/vimango/hunspell"
	"github.com/slzatz/vimango/vim"
)

var e_lookup2 = map[string]interface{}{
	"\x17L":              (*Editor).moveOutputWindowRight,
	"\x17J":              (*Editor).moveOutputWindowBelow,
	"\x08":               (*Editor).controlH,
	"\x0c":               controlL,
	"\x0a":               (*Editor).controlJ,
	"\x0b":               (*Editor).controlK,
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
	//"\x06":               (*Editor).findMatchForBrace, // for testing
	//"z=":                 (*Editor).suggest,
	//leader + "l": (*Editor).showVimMessageLog,
	leader + "m": (*Editor).showMarkdownPreview,
	leader + "y": (*Editor).nextStyle,
	//leader + "w": showWindows,
	//leader + "p":         (*Editor).showSpellingPreview,
	leader + "t":  (*Editor).readGoTemplate,
	leader + "co": (*Editor).completion,
	leader + "ho": (*Editor).hover,
	leader + "sh": (*Editor).signatureHelp,
	leader + "dh": (*Editor).documentHighlight,
	leader + "df": (*Editor).definition,
	leader + "rf": (*Editor).reference,
	leader + "sp": (*Editor).spellingCheck,
	leader + "su": (*Editor).spellSuggest,
	//leader + "xx": (*Editor).test,
	"z=": (*Editor).spellSuggest,
}

func (e *Editor) changeSplit(flag int) {
	if e.output == nil {
		return
	}

	op := e.output
	var outputHeight int
	if flag == '=' {
		outputHeight = sess.textLines / 2
	} else if flag == '_' {
		outputHeight = 2
	} else if flag == '+' {
		outputHeight = op.screenlines - 1
	} else if flag == '-' {
		outputHeight = op.screenlines + 1
	} else {
		sess.showOrgMessage("flag = %v", flag)
		return
	}
	sess.showOrgMessage("flag = %v", flag)

	if outputHeight < 2 || outputHeight > sess.textLines-3 {
		return
	}

	e.screenlines = sess.textLines - outputHeight - 1
	op.screenlines = outputHeight
	op.top_margin = sess.textLines - outputHeight + 2

	sess.eraseRightScreen()
	sess.drawRightScreen()
}

func (e *Editor) changeHSplit(flag int) {
	var width int
	if flag == '>' {
		width = sess.screenCols - sess.divider + 1
		moveDividerAbs(width)
	} else if flag == '<' {
		width = sess.screenCols - sess.divider - 1
		moveDividerAbs(width)
	} else {
		sess.showOrgMessage("flag = %v", flag)
		return
	}
}

func (e *Editor) moveOutputWindowRight() {
	if e.output == nil { // && e.is_subeditor && e.is_below) {
		return
	}
	//top_margin = TOP_MARGIN + 1;
	//screenlines = total_screenlines - 1;
	e.output.is_below = false

	sess.positionWindows()
	sess.eraseRightScreen()
	sess.drawRightScreen()
	//editorSetMessage("top_margin = %d", top_margin);

}

func (e *Editor) moveOutputWindowBelow() {
	if e.output == nil { // && e.is_subeditor && e.is_below) {
		return
	}
	//top_margin = TOP_MARGIN + 1;
	//screenlines = total_screenlines - 1;
	e.output.is_below = true

	sess.positionWindows()
	sess.eraseRightScreen()
	sess.drawRightScreen()
	//editorSetMessage("top_margin = %d", top_margin);
}

// should scroll output down
func (e *Editor) controlJ() {
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

// should scroll output up
func (e *Editor) controlK() {
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

func (e *Editor) controlH() {
	// below "if" really for testing
	if e.isModified() {
		sess.showEdMessage("Note you left has been modified")
	}

	if sess.numberOfEditors() == 1 {

		if sess.divider < 10 {
			sess.edPct = 80
			moveDividerPct(80)
		}

		sess.editorMode = false        //needs to be here
		vim.BufferSetCurrent(org.vbuf) ///////////////////////////////////////////////////////////

		//org.readTitleIntoBuffer() // shouldn't be necessary
		org.drawPreview()
		org.mode = NORMAL
		sess.returnCursor()
		return
	}

	eds := sess.editors()
	index := 0
	for i, ed := range eds {
		if ed == e {
			index = i
			break
		}
	}

	sess.showEdMessage("index: %d; length: %d", index, len(eds))

	if index > 0 {
		p = eds[index-1]
		vim.BufferSetCurrent(p.vbuf)
		p.mode = NORMAL
		return
	} else {

		if sess.divider < 10 {
			sess.edPct = 80
			moveDividerPct(80)
		}

		sess.editorMode = false        //needs to be here
		vim.BufferSetCurrent(org.vbuf) ///////////////////////////////////////////////////////////

		//org.readTitleIntoBuffer() // shouldn't be necessary
		org.drawPreview()
		org.mode = NORMAL
		sess.returnCursor()
		return
	}
}

func controlL() {
	// below "if" really for testing
	if p.isModified() {
		sess.showEdMessage("Note you left has been modified")
	}

	eds := sess.editors()
	index := 0
	for i, e := range eds {
		if e == p {
			index = i
			break
		}
	}
	sess.showEdMessage("index: %d; length: %d", index, len(eds))

	if index < len(eds)-1 {
		p = eds[index+1]
		p.mode = NORMAL
		vim.BufferSetCurrent(p.vbuf)
	}

	return
}

// for VISUAL mode
func (e *Editor) decorateWordVisual(c int) {
	if len(e.ss) == 0 {
		return
	}

	if e.highlight[0][0] != e.highlight[1][0] {
		sess.showEdMessage("The text must all be in the same row")
		return
	}

	row := e.ss[e.highlight[0][0]-1]
	beg, end := e.highlight[0][1]-1, e.highlight[1][1]

	var undo bool
	s := row[beg:end]
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
		vim.Input2("xi" + s + "\x1b")
		return
	}

	switch c {
	case ctrlKey('b'):
		s = fmt.Sprintf("**%s**", s)
	case ctrlKey('i'):
		s = fmt.Sprintf("*%s*", s)
	case ctrlKey('e'):
		s = fmt.Sprintf("`%s`", s)
	}

	vim.Input2("xi" + s + "\x1b")
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

	vim.Execute("let cword = expand('<cword>')")
	w := vim.Eval("cword")

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
		vim.Input2("ciw" + w + "\x1b")
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
	vim.Input("ciw" + w + "\x1b")
}

/*
func showLastVimMessage() {
	_ = v.SetCurrentBuffer(messageBuf)

	// don't have to erase message buffer before adding to it
	//_ = v.SetBufferLines(messageBuf, 0, -1, true, [][]byte{})
	_ = v.FeedKeys("\x1bG$\"apqaq", "t", false) //qaq ->record macro to register 'a'  followed by q (stop recording) clears register
	bb, _ := v.BufferLines(messageBuf, 0, -1, true)
	var message string
	var i int
	for i = len(bb) - 1; i >= 0; i-- {
		message = string(bb[i])
		if message != "" {
			break
		}
	}
	v.SetCurrentBuffer(p.vbuf)
	currentBuf, _ := v.CurrentBuffer()
	if message != "" {
		sess.showEdMessage("message: %v", message)
	} else {
		sess.showEdMessage("No message: len bb %v; Current Buf %v", len(bb), currentBuf)
	}
}
*/

/*
// for testing - displays vimMessageLog in preview windows
func (e *Editor) showVimMessageLog() {

	// In this case don't want to erase log of messages
	//_ = v.SetBufferLines(messageBuf, 0, -1, true, [][]byte{})
	_ = v.SetCurrentBuffer(messageBuf)

	// \"ap pastes register a into messageBuf buffer
	// qaq is wierd way to erase a register (record macro to register then immediatly stop recording)
	_ = v.FeedKeys("\x1bG$\"apqaq", "t", false)
	v.SetCurrentBuffer(p.vbuf)

	bb, _ := v.BufferLines(messageBuf, 0, -1, true)

	// NOTE: not word wrapping and probably should
	var rows []string
	for _, b := range bb {
		rows = append(rows, string(b))
	}
	e.overlay = rows
	e.mode = VIEW_LOG
	e.previewLineOffset = 0
	e.drawOverlay()
}
*/
// appears to be no way to actually create new standard windows
// can create floating windows but not sure we want them
// prints [Window:1000]
/*
func showWindows() {
	w, _ := v.Windows()
	sess.showEdMessage("windows: %v", w)
}
*/

func (e *Editor) showMarkdownPreview() {
	if len(e.ss) == 0 {
		return
	}

	//note := readNoteIntoString(e.id)

	//note = generateWWString(note, e.screencols, -1, "\n")
	note := e.generateWWStringFromBuffer2()
	r, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("/home/slzatz/listmango/darkslz.json"),
		glamour.WithWordWrap(0),
	)
	note, _ = r.Render(note)
	note = strings.TrimSpace(note)
	note = strings.ReplaceAll(note, "\n\x1b[0m", "\x1b[0m\n") //headings seem to place \x1b[0m after the return
	note = strings.ReplaceAll(note, "\n\n\n", "\n\n")

	// for some` reason get extra line at top
	//ix := strings.Index(note, "\n") //works for ix = -1
	//e.renderedNote = note[ix+1:]
	e.renderedNote = note

	e.mode = PREVIEW
	e.previewLineOffset = 0
	e.drawPreview()

}

func (e *Editor) nextStyle() {
	sess.styleIndex++
	if sess.styleIndex > len(sess.style)-1 {
		sess.styleIndex = 0
	}
	sess.showEdMessage("New style is %q", sess.style[sess.styleIndex])
}

/*
func (e *Editor) suggest() {
	// clear messageBuf before redirecting z= output
	_ = v.SetBufferLines(messageBuf, 0, -1, true, [][]byte{}) // in test case bytes.Fields(nil)

	_ = v.FeedKeys("qaq", "t", false)

	// so below you get suggestions but with the return
	// you're telling vim you didn't select one
	// gets selected when you type a number
	// in SPELLING mode
	_, err := v.Input("z=\r")
	if err != nil {
		sess.showEdMessage("z= err: %v", err)
	}

	// 1) set current buffer to messageBuf
	// 2) paste register a into messageBuf -> "ap
	// 3) clear register a -> qaq
	// ? step 3 necessary if always clear before using
	_ = v.SetCurrentBuffer(messageBuf)
	_ = v.FeedKeys("\x1bgg\"apqaq", "t", false)

	// set current buffer back to editor
	v.SetCurrentBuffer(e.vbuf)

	bb, _ := v.BufferLines(messageBuf, 0, -1, true)

	// NOTE: not word wrapping and probably should
	var rows []string
	for _, b := range bb {
		rows = append(rows, string(b))
	}
	e.overlay = rows[1:] // first row is blank
	e.mode = SPELLING
	e.previewLineOffset = 0
	e.drawOverlay()
}
*/

func (e *Editor) readGoTemplate() {
	e.readFileIntoNote("go.template")
}

func (e *Editor) completion() {
	if e.isModified() {
		sess.showEdMessage("%sNeed to write note first%s", RED_BG, RESET)
		return
		//sendDidChangeNotification(e.bufferToString())
		//e.drawDiagnostics()
	}
	//e.drawDiagnostics()
	sendCompletionRequest(uint32(e.fr), uint32(e.fc+1)) //+1 seems to work better
}

func (e *Editor) hover() {
	if e.isModified() {
		sess.showEdMessage("%sNeed to write note first%s", RED_BG, RESET)
		return
		//sendDidChangeNotification(e.bufferToString())
		//e.drawDiagnostics()
	}
	sendHoverRequest(uint32(e.fr), uint32(e.fc+1)) //+1 seems to work better
}

func (e *Editor) signatureHelp() {
	if e.isModified() {
		sess.showEdMessage("%sNeed to write note first%s", RED_BG, RESET)
		return
		//sendDidChangeNotification(e.bufferToString())
		//e.drawDiagnostics()
	}
	sendSignatureHelpRequest(uint32(e.fr), uint32(e.fc)) //+1 seems to work better
}

func (e *Editor) documentHighlight() {
	if e.isModified() {
		sess.showEdMessage("%sNeed to write note first%s", RED_BG, RESET)
		return
		//sendDidChangeNotification(e.bufferToString())
		//e.drawDiagnostics()
	}
	sendDocumentHighlightRequest(uint32(e.fr), uint32(e.fc)) //+1 seems to work better
}

func (e *Editor) definition() {
	if e.isModified() {
		sess.showEdMessage("%sNeed to write note first%s", RED_BG, RESET)
		return
		//sendDidChangeNotification(e.bufferToString())
		//e.drawDiagnostics()
	}
	sendDefinitionRequest(uint32(e.fr), uint32(e.fc)) //+1 seems to work better
}

func (e *Editor) reference() {
	if e.isModified() {
		sendDidChangeNotification(e.bufferToString())
		e.drawDiagnostics()
	}
	sendReferenceRequest(uint32(e.fr), uint32(e.fc)) //+1 seems to work better
}

func (e *Editor) spellingCheck() {
	if e.isModified() {
		sess.showEdMessage("%sYou need to write the note before highlighting text%s", RED_BG, RESET)
		return
	}
	e.highlightMispelledWords()
}

func (e *Editor) spellSuggest() {
	h := hunspell.Hunspell("/usr/share/hunspell/en_US.aff", "/usr/share/hunspell/en_US.dic")
	vim.Execute("let cword = expand('<cword>')")
	w := vim.Eval("cword")
	if ok := h.Spell(w); ok {
		sess.showEdMessage("%q is spelled correctly", w)
		return
	}
	s := h.Suggest(w)
	sess.showEdMessage("%q -> %s", w, strings.Join(s, "|"))
}
