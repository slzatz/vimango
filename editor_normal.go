package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/slzatz/vimango/vim"
)

func (a *App) setEditorNormalCmds(editor *Editor) map[string]func(*Editor, int) {
	registry := NewCommandRegistry[func(*Editor, int)]()

	// Movement commands
	registry.Register("\x08", (*Editor).moveLeft, CommandInfo{
		Name:        keyToDisplayName("\x08"),
		Description: "Move to previous editor or return to organizer",
		Usage:       "Ctrl-H",
		Category:    "Editor Selection",
		Examples:    []string{"Ctrl-H - Switch to previous editor"},
	})

	registry.Register("\x0c", (*Editor).moveRight, CommandInfo{
		Name:        keyToDisplayName("\x0c"),
		Description: "Move to next editor",
		Usage:       "Ctrl-L",
		Category:    "Editor Selection",
		Examples:    []string{"Ctrl-L - Switch to next editor"},
	})

	// Text Editing commands
	registry.Register(string(ctrlKey('b')), (*Editor).decorateWord, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('b'))),
		Aliases:     []string{leader + "b"},
		Description: "Make word bold (toggle **word**)",
		Usage:       "Ctrl-B",
		Category:    "Markup Shortcuts",
		Examples:    []string{"Ctrl-B - Toggle bold formatting on current word"},
	})

	registry.Register(string(ctrlKey('e')), (*Editor).decorateWord, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('e'))),
		Aliases:     []string{leader + "e"},
		Description: "Make word code (toggle `word`)",
		Usage:       "Ctrl-E",
		Category:    "Markup Shortcuts",
		Examples:    []string{"Ctrl-E - Toggle code formatting on current word"},
	})

	registry.Register(string(ctrlKey('i')), (*Editor).decorateWord, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('i'))),
		Aliases:     []string{leader + "i"},
		Description: "Make word italic (toggle *word*)",
		Usage:       "Ctrl-I",
		Category:    "Markup Shortcuts",
		Examples:    []string{"Ctrl-I - Toggle italic formatting on current word"},
	})

	// Preview commands
	registry.Register(leader+"m", (*Editor).showMarkdownPreview, CommandInfo{
		Name:        keyToDisplayName(leader + "m"),
		Description: "Show markdown preview of current note",
		Usage:       "<leader>m",
		Category:    "Preview",
		Examples:    []string{"<leader>m - Display formatted markdown preview"},
	})

	registry.Register(leader+"w", (*Editor).showWebView, CommandInfo{
		Name:        keyToDisplayName(leader + "w"),
		Description: "Show current note in web browser",
		Usage:       "<leader>w",
		Category:    "Preview",
		Examples:    []string{"<leader>w - Open note in web browser"},
	})

	// Window Management commands
	registry.Register("\x17L", (*Editor).moveOutputWindowRight, CommandInfo{
		Name:        keyToDisplayName("\x17L"),
		Description: "Move output window to the right",
		Usage:       "<C-w>L",
		Category:    "Window Management",
		Examples:    []string{"<C-w>L - Position output window to the right"},
	})

	registry.Register("\x17J", (*Editor).moveOutputWindowBelow, CommandInfo{
		Name:        keyToDisplayName("\x17J"),
		Description: "Move output window below editor",
		Usage:       "<C-w>J",
		Category:    "Window Management",
		Examples:    []string{"<C-w>J - Position output window below editor"},
	})

	registry.Register("\x17=", (*Editor).changeSplit, CommandInfo{
		Name:        keyToDisplayName("\x17="),
		Description: "Equalize split sizes",
		Usage:       "<C-w>=",
		Category:    "Window Management",
		Examples:    []string{"<C-w>= - Make editor and output windows equal size"},
	})

	registry.Register("\x17_", (*Editor).changeSplit, CommandInfo{
		Name:        keyToDisplayName("\x17_"),
		Description: "Minimize output window",
		Usage:       "<C-w>_",
		Category:    "Window Management",
		Examples:    []string{"<C-w>_ - Minimize output window to 2 lines"},
	})

	registry.Register("\x17-", (*Editor).changeSplit, CommandInfo{
		Name:        keyToDisplayName("\x17-"),
		Description: "Decrease output window height",
		Usage:       "<C-w>-",
		Category:    "Window Management",
		Examples:    []string{"<C-w>- - Decrease output window height by 1 line"},
	})

	registry.Register("\x17+", (*Editor).changeSplit, CommandInfo{
		Name:        keyToDisplayName("\x17+"),
		Description: "Increase output window height",
		Usage:       "<C-w>+",
		Category:    "Window Management",
		Examples:    []string{"<C-w>+ - Increase output window height by 1 line"},
	})

	registry.Register("\x17>", (*Editor).changeHSplit, CommandInfo{
		Name:        keyToDisplayName("\x17>"),
		Description: "Increase editor width",
		Usage:       "<C-w>>",
		Category:    "Window Management",
		Examples:    []string{"<C-w>> - Increase editor width by 1 column"},
	})

	registry.Register("\x17<", (*Editor).changeHSplit, CommandInfo{
		Name:        keyToDisplayName("\x17<"),
		Description: "Decrease editor width",
		Usage:       "<C-w><",
		Category:    "Window Management",
		Examples:    []string{"<C-w>< - Decrease editor width by 1 column"},
	})

	// Output Control commands
	registry.Register("\x0a", (*Editor).scrollOutputDown, CommandInfo{
		Name:        keyToDisplayName("\x0a"),
		Description: "Scroll output window down",
		Usage:       "Ctrl-J",
		Category:    "Output Control",
		Examples:    []string{"Ctrl-J - Scroll down in output window"},
	})

	registry.Register("\x0b", (*Editor).scrollOutputUp, CommandInfo{
		Name:        keyToDisplayName("\x0b"),
		Description: "Scroll output window up",
		Usage:       "Ctrl-K",
		Category:    "Output Control",
		Examples:    []string{"Ctrl-K - Scroll up in output window"},
	})

	// Utility commands
	registry.Register(leader+"y", (*Editor).nextStyle, CommandInfo{
		Name:        keyToDisplayName(leader + "y"),
		Description: "Cycle through available styles",
		Usage:       "<leader>y",
		Category:    "Utility",
		Examples:    []string{"<leader>y - Switch to next available style"},
	})

	registry.Register(leader+"t", (*Editor).readGoTemplate, CommandInfo{
		Name:        keyToDisplayName(leader + "t"),
		Description: "Read Go template into current note",
		Usage:       "<leader>t",
		Category:    "Utility",
		Examples:    []string{"<leader>t - Insert Go template content"},
	})

	registry.Register(leader+"sp", (*Editor).spellingCheck, CommandInfo{
		Name:        keyToDisplayName(leader + "sp"),
		Description: "Highlight misspelled words",
		Usage:       "<leader>sp",
		Category:    "Utility",
		Examples:    []string{"<leader>sp - Check spelling and highlight errors"},
	})

	registry.Register(leader+"su", (*Editor).spellSuggest, CommandInfo{
		Name:        keyToDisplayName(leader + "su"),
		Description: "Show spelling suggestions for current word",
		Usage:       "<leader>su",
		Category:    "Utility",
		Examples:    []string{"<leader>su - Get spelling suggestions"},
	})

	// System commands
	registry.Register(string(ctrlKey('z')), (*Editor).switchImplementation, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('z'))),
		Description: "Switch between Go and C vim implementations",
		Usage:       "Ctrl-Z",
		Category:    "System",
		Examples:    []string{"Ctrl-Z - Toggle vim implementation"},
	})

	// Store registry in editor for help command access
	editor.normalCommandRegistry = registry

	return registry.GetFunctionMap()
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
		app.Session.activeEditor = ae
		vim.SetCurrentBuffer(ae.vbuf)
		// There is a bug in libvim C implementation where the cursor column position is set to zero when switching buffers
		// so we set the cursor position from the stored values
		vim.SetCursorPosition(ae.fr+1, ae.fc)
		ae.ShowMessage(BR, "Cursor position: %+v", vim.GetCursorPosition())
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
	// below "if" for testing but may have use
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
	pos := vim.GetCursorPosition()
	e.ShowMessage(BR, "Before move: index: %d; length: %d e.fr: %d; e.fc %d", index, len(eds), pos[0]-1, pos[1])

	if index < len(eds)-1 {
		ae := eds[index+1]
		vim.SetCurrentBuffer(ae.vbuf)
		app.Session.activeEditor = ae
		// There is a bug in libvim C implementation where the cursor column position is set to zero when switching buffers
		// so we set the cursor position from the stored values
		vim.SetCursorPosition(ae.fr+1, ae.fc)
		ae.ShowMessage(BR, "Cursor position: %+v", vim.GetCursorPosition())
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
	//note := e.generateWWStringFromBuffer2()
	note := strings.Join(e.vbuf.Lines(), "\n")
	r, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("darkslz.json"),
		glamour.WithWordWrap(0),
	)
	note, _ = r.Render(note)
	//note = WordWrap(note, e.Screen.totaleditorcols)
	note = WordWrap(note, e.screencols)
	note = strings.TrimSpace(note)
	e.renderedNote = note
	e.mode = PREVIEW
	e.previewLineOffset = 0
	e.drawPreview()
}

func (e *Editor) showWebView(_ int) {
	if len(e.ss) == 0 {
		return
	}

	// Get current note content
	note := strings.Join(e.vbuf.Lines(), "\n")

	// Get note title from the editor
	title := e.title
	if title == "" {
		title = "Untitled Note"
	}

	// Convert to HTML
	htmlContent, err := RenderNoteAsHTML(title, note)
	if err != nil {
		e.ShowMessage(BR, "Error rendering HTML: %v", err)
		return
	}

	// Check if webview is available
	if !IsWebviewAvailable() {
		e.ShowMessage(BR, ShowWebviewNotAvailableMessage())
		// Fall back to opening in browser
		err = OpenNoteInWebview(title, htmlContent)
		if err != nil {
			e.ShowMessage(BR, "Error opening note: %v", err)
		}
		return
	}

	// Open in webview in a goroutine since it blocks
	// This will either create a new webview or update the existing one
	go func() {
		err := OpenNoteInWebview(title, htmlContent)
		if err != nil {
			// Note: Can't directly show message from goroutine
			// Could implement a channel-based message system if needed
		}
	}()

	if IsWebviewRunning() {
		e.ShowMessage(BR, "Updating webview content...")
	} else {
		e.ShowMessage(BR, "Opening note in webview...")
	}
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
	if !IsSpellCheckAvailable() {
		e.ShowMessage(BR, ShowSpellCheckNotAvailableMessage())
		return
	}

	if e.isModified() {
		e.ShowMessage(BR, "%sYou need to write the note before highlighting text%s", RED_BG, RESET)
		return
	}
	e.highlightMispelledWords()
}

func (e *Editor) spellSuggest(_ int) {
	if !IsSpellCheckAvailable() {
		e.ShowMessage(BR, ShowSpellCheckNotAvailableMessage())
		return
	}

	curPos := vim.GetCursorPosition()
	w, _, _ := GetWordAtIndex(e.ss[curPos[0]-1], curPos[1])
	//w := vim.EvaluateExpression("expand('<cword>')")

	if CheckSpelling(w) {
		e.ShowMessage(BR, "%q is spelled correctly", w)
		return
	}

	suggestions := GetSpellingSuggestions(w)
	e.ShowMessage(BR, "%q -> %s", w, strings.Join(suggestions, "|"))
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
