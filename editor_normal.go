package main

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/slzatz/vimango/vim"
)

// Normal mode intercepts single bytes only: no prefixes, no multi-key
// sequences, no leader. A key registered here never reaches vim, so the table
// is confined to what vim cannot know about: which editor has focus. Editing
// shortcuts (bold/italic/code, lists) are vim mappings in vimango.vim -- see
// vimrc.go. Anything wanting an argument or a name is an ex command; see
// setEditorExCmds.
func (a *App) setEditorNormalCmds(editor *Editor) map[string]func(*Editor, int) {
	registry := NewCommandRegistry[func(*Editor, int)]()

	// Editor selection
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

	// Store registry in editor for help command access
	editor.normalCommandRegistry = registry

	return registry.GetFunctionMap()
}

func (e *Editor) moveLeft(_ int) {
	// below "if" really for testing
	if e.isModified() {
		e.ShowMessage(BR, "Note you left has been modified")
	}

	if len(e.Session.Editors) == 1 {
		if e.Session.editorOnly {
			e.ShowMessage(BR, "No organizer in editor-only mode")
			return
		}

		if e.Screen.divider < 10 {
			e.Screen.edPct = 80
			app.moveDividerPct(80)
		}
		e.Session.editorMode = false
		vim.SetCurrentBuffer(app.Organizer.vbuf)
		app.Organizer.displayNote()
		app.Organizer.mode = NORMAL
		app.returnCursor()
		return
	}

	index := 0
	for i, ed := range e.Session.Editors {
		if ed == e {
			index = i
			break
		}
	}

	e.ShowMessage(BL, "index: %d; length: %d", index, len(e.Session.Editors))

	if index > 0 {
		ae := e.Session.Editors[index-1]
		app.Session.activeEditor = ae
		vim.SetCurrentBuffer(ae.vbuf)
		// There is a bug in libvim C implementation where the cursor column position is set to zero when switching buffers
		// so we set the cursor position from the stored values
		vim.SetCursorPosition(ae.fr+1, ae.fc)
		ae.ShowMessage(BR, "Cursor position: %+v", vim.GetCursorPosition())
		return
	} else {
		if e.Session.editorOnly {
			e.ShowMessage(BR, "No organizer in editor-only mode")
			return
		}

		if e.Screen.divider < 10 {
			e.Screen.edPct = 80
			app.moveDividerPct(80)
		}
		e.Session.editorMode = false
		vim.SetCurrentBuffer(app.Organizer.vbuf)
		app.Organizer.displayNote()
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

	index := 0
	for i, z := range e.Session.Editors {
		if z == e {
			index = i
			break
		}
	}
	pos := vim.GetCursorPosition()
	e.ShowMessage(BR, "Before move: index: %d; length: %d e.fr: %d; e.fc %d", index, len(e.Session.Editors), pos[0]-1, pos[1])

	if index < len(e.Session.Editors)-1 {
		ae := e.Session.Editors[index+1]
		vim.SetCurrentBuffer(ae.vbuf)
		app.Session.activeEditor = ae
		// There is a bug in libvim C implementation where the cursor column position is set to zero when switching buffers
		// so we set the cursor position from the stored values
		vim.SetCursorPosition(ae.fr+1, ae.fc)
		ae.ShowMessage(BR, "Cursor position: %+v", vim.GetCursorPosition())
	}

	return
}

// splitExRange peels a leading ex range off a command line, returning the range
// (empty if there is none) and the remainder. vim writes "'<,'>" itself when
// ':' is pressed from VISUAL mode; "%" is the only other range vimango is
// likely to meet.
func splitExRange(line string) (rng, rest string) {
	const visual = "'<,'>"
	switch {
	case strings.HasPrefix(line, visual):
		return visual, line[len(visual):]
	case strings.HasPrefix(line, "%"):
		return "%", line[1:]
	}
	return "", line
}

func (e *Editor) showMarkdownPreview(_ int) {
	if len(e.ss) == 0 {
		return
	}
	//note := e.generateWWStringFromBuffer2()
	note := strings.Join(e.vbuf.Lines(), "\n")
	r, _ := glamour.NewTermRenderer(
		glamour.WithStylePath(getGlamourStylePath()),
		glamour.WithWordWrap(0),
	)
	note, _ = r.Render(note)
	// Decode any Kitty text sizing markers (OSC 66)
	note = ansi.DecodeKittyTextSizeMarkers(note)
	//note = WordWrap(note, e.Screen.totaleditorcols)
	note = WordWrap(note, e.screencols, 0)
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
	htmlContent, err := RenderNoteAsHTML(title, note, true, false)
	if err != nil {
		e.ShowMessage(BR, "Error rendering HTML: %v", err)
		return
	}

	// Check if webview is available
	if !IsWebviewAvailable() {
		e.ShowMessage(BR, "%s", ShowWebviewNotAvailableMessage())
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
		e.ShowMessage(BR, "%s", ShowSpellCheckNotAvailableMessage())
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
		e.ShowMessage(BR, "%s", ShowSpellCheckNotAvailableMessage())
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

// Ex-command forms of the normal-mode shortcuts. The registry wants
// func(*Editor); the normal-mode table wants func(*Editor, int).

func (e *Editor) previewCmd()  { e.showMarkdownPreview(0) }
func (e *Editor) webviewCmd()  { e.showWebView(0) }
func (e *Editor) styleCmd()    { e.nextStyle(0) }
func (e *Editor) templateCmd() { e.readGoTemplate(0) }
func (e *Editor) spellCmd()    { e.spellingCheck(0) }
func (e *Editor) suggestCmd()  { e.spellSuggest(0) }
