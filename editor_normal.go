package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/slzatz/vimango/vim"
)

// Normal mode intercepts single bytes only: no prefixes, no multi-key
// sequences, no leader. A key registered here never reaches vim, so the table
// is deliberately confined to what vim cannot know about (which editor has
// focus) plus the markdown decoration shortcuts, which libvim cannot provide
// because it registers mappings without ever expanding them. Everything else --
// anything wanting an argument, a range or a name -- is an ex command; see
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

	// Markup shortcuts -- the one-keystroke form of :bold / :italic / :code.
	// In VISUAL mode these act on the selection (see VisualModeKeyHandler).
	registry.Register(string(ctrlKey('b')), (*Editor).decorateWord, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('b'))),
		Description: "Make word bold (toggle **word**)",
		Usage:       "Ctrl-B",
		Category:    "Markup Shortcuts",
		Examples:    []string{"Ctrl-B - Toggle bold formatting on current word", ":bold - same, and works on a visual selection"},
	})

	registry.Register(string(ctrlKey('e')), (*Editor).decorateWord, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('e'))),
		Description: "Make word code (toggle `word`)",
		Usage:       "Ctrl-E",
		Category:    "Markup Shortcuts",
		Examples:    []string{"Ctrl-E - Toggle code formatting on current word", ":code - same, and works on a visual selection"},
	})

	registry.Register(string(ctrlKey('i')), (*Editor).decorateWord, CommandInfo{
		Name:        keyToDisplayName(string(ctrlKey('i'))),
		Description: "Make word italic (toggle *word*)",
		Usage:       "Ctrl-I",
		Category:    "Markup Shortcuts",
		Examples:    []string{"Ctrl-I - Toggle italic formatting on current word", ":italic - same, and works on a visual selection"},
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

// visualMarks reconstructs the last visual selection from vim's '< and '>
// marks, in the same [line col][line col] shape vim.GetVisualRange returns
// (line 1-based, col 0-based). The marks outlive the <esc> that
// ExModeKeyHandler sends to cancel vim's cmdline, which is what lets a ranged
// ex command still find its operand.
func visualMarks() ([2][2]int, bool) {
	num := func(expr string) (int, bool) {
		n, err := strconv.Atoi(vim.EvaluateExpression(expr))
		return n, err == nil
	}
	l1, ok1 := num(`line("'<")`)
	c1, ok2 := num(`col("'<")`)
	l2, ok3 := num(`line("'>")`)
	c2, ok4 := num(`col("'>")`)
	if !ok1 || !ok2 || !ok3 || !ok4 || l1 == 0 || l2 == 0 {
		return [2][2]int{}, false
	}
	return [2][2]int{{l1, c1 - 1}, {l2, c2 - 1}}, true
}

// Markdown decoration markers, keyed by the Ctrl key that applies them.
const (
	markerBold   = "**"
	markerItalic = "*"
	markerCode   = "`"
)

// markerForKey maps the Ctrl-B/I/E shortcuts to a marker.
func markerForKey(c int) string {
	switch c {
	case ctrlKey('b'), 'b':
		return markerBold
	case ctrlKey('i'), 'i':
		return markerItalic
	case ctrlKey('e'), 'e':
		return markerCode
	}
	return ""
}

// currentMarker reports which markdown decoration s already carries, if any.
func currentMarker(s string) string {
	switch {
	case strings.HasPrefix(s, markerBold):
		return markerBold
	case strings.HasPrefix(s, markerItalic):
		return markerItalic
	case strings.HasPrefix(s, markerCode):
		return markerCode
	}
	return ""
}

// decorateText wraps s in marker, or strips the decoration if s already carries
// that marker -- so applying the same style twice toggles it off. A different
// existing marker is replaced, which is how Ctrl-B over *word* yields **word**.
func decorateText(s, marker string) string {
	had := currentMarker(s)
	s = strings.Trim(s, "*`")
	if had == marker {
		return s
	}
	return marker + s + marker
}

// decorateWord is the NORMAL mode entry point (Ctrl-B/I/E): it decorates the
// word under the cursor.
func (e *Editor) decorateWord(c int) {
	e.decorateCword(markerForKey(c))
}

// decorateWordVisual is the VISUAL mode entry point (Ctrl-B/I/E): it decorates
// the current selection. The caller must have left VISUAL mode first -- see
// decorateSpan.
func (e *Editor) decorateWordVisual(c int) {
	e.decorateRange(markerForKey(c), e.highlight)
}

// decorateCword decorates the word under the cursor -- what :bold and friends
// do when given no range.
func (e *Editor) decorateCword(marker string) {
	if marker == "" || e.fr >= len(e.ss) {
		return
	}
	row := e.ss[e.fr]
	_, beg, end := GetWordAtIndex(row, e.fc)
	if beg < 0 {
		return // on whitespace or punctuation: nothing to decorate
	}
	e.decorateSpan(e.fr+1, beg, end+1, marker)
}

// decorateRange decorates a charwise selection given as [line col][line col],
// line 1-based and col 0-based -- the shape vim.GetVisualRange returns and that
// visualMarks reconstructs from vim's '< and '> marks.
func (e *Editor) decorateRange(marker string, rng [2][2]int) {
	if marker == "" {
		return
	}
	if rng[0][0] != rng[1][0] {
		e.ShowMessage(BR, "The text must all be in the same row")
		return
	}
	e.decorateSpan(rng[0][0], rng[0][1], rng[1][1]+1, marker)
}

// decorateSpan toggles marker on row lnum's bytes [beg,end).
//
// The replacement is done with vim's counted "s" rather than "ciw" or a visual
// "x": "<n>s" substitutes exactly n characters from the cursor, so the span
// vimango computed is the span vim edits. ciw was the old mechanism and it
// disagreed with expand('<cword>') whenever the cursor sat on a marker --
// <cword> skips ahead to the next word while ciw takes the punctuation run
// under the cursor -- which is why toggling a decoration back off never worked.
// vim must be in NORMAL mode: in VISUAL, "s" substitutes the selection instead.
func (e *Editor) decorateSpan(lnum, beg, end int, marker string) {
	lines := e.vbuf.Lines()
	if lnum < 1 || lnum > len(lines) {
		return
	}
	row := lines[lnum-1]
	if beg < 0 || beg >= len(row) || end <= beg {
		return
	}
	if end > len(row) {
		end = len(row) // in VISUAL the cursor can sit past the end of the row
	}

	beg, end = expandOverMarkers(row, beg, end)
	replacement := decorateText(row[beg:end], marker)

	vim.SetCursorPosition(lnum, beg)
	vim.SendInput(fmt.Sprintf("%ds%s\x1b", end-beg, replacement))

	// park the cursor on the text rather than on a trailing marker, so the
	// same command applied again finds the word and toggles the decoration off
	vim.SetCursorPosition(lnum, beg+len(currentMarker(replacement)))
}

// expandOverMarkers widens [beg,end) outward across a balanced run of markdown
// markers, so a word selected or found without its decoration still toggles.
func expandOverMarkers(row string, beg, end int) (int, int) {
	isMarker := func(b byte) bool { return b == '*' || b == '`' }
	for beg > 0 && end < len(row) && isMarker(row[beg-1]) && isMarker(row[end]) {
		beg--
		end++
	}
	return beg, end
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

// Ex-command forms of the normal-mode shortcuts. The registry wants
// func(*Editor); the normal-mode table wants func(*Editor, int).

// decorateEx is the :bold / :italic / :code body: with a range it acts on the
// visual selection, without one on the word under the cursor.
func (e *Editor) decorateEx(marker string) {
	if !e.exRange {
		e.decorateCword(marker)
		return
	}
	rng, ok := visualMarks()
	if !ok {
		e.ShowMessage(BR, "No visual selection to decorate")
		return
	}
	e.decorateRange(marker, rng)
}

func (e *Editor) boldCmd()   { e.decorateEx(markerBold) }
func (e *Editor) italicCmd() { e.decorateEx(markerItalic) }
func (e *Editor) codeCmd()   { e.decorateEx(markerCode) }

func (e *Editor) previewCmd()  { e.showMarkdownPreview(0) }
func (e *Editor) webviewCmd()  { e.showWebView(0) }
func (e *Editor) styleCmd()    { e.nextStyle(0) }
func (e *Editor) templateCmd() { e.readGoTemplate(0) }
func (e *Editor) spellCmd()    { e.spellingCheck(0) }
func (e *Editor) suggestCmd()  { e.spellSuggest(0) }
