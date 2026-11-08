package main

import (
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/slzatz/vimango/vim"
)

// The editor intercepts no normal-mode keys: every key reaches vim, and
// editing shortcuts are vim mappings in vimango.vim (see vimrc.go). Anything
// wanting an argument or a name is an ex command; see setEditorExCmds. What is
// left here are the bodies of some of those ex commands.

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
