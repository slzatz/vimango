//go:build exclude
package main

import (
	"strings"
	
	"github.com/slzatz/vimango/vim"
)

// EditorCommand defines a command function that can be executed by the editor
type EditorCommand func(*Editor, *AppContext) bool

// CommandResult represents the result of a command execution
type CommandResult struct {
	TextChanged bool
	ModeChanged bool
	Message     string
}

// toggleLinenumbers toggles the display of line numbers
func (e *Editor) toggleLinenumbers() {
	e.numberLines = !e.numberLines
	if !e.numberLines {
		e.left_margin_offset = 0
	} else {
		e.left_margin_offset = LEFT_MARGIN_OFFSET
	}
}

// toggleSyntaxHighlighting toggles syntax highlighting for the editor
func (e *Editor) toggleSyntaxHighlighting() {
	e.highlightSyntax = !e.highlightSyntax
}

// We'll revert to using the global implementation for now
// Our context-based version will be in a follow-up refactoring

// WriteNote saves the current editor content to the database
func (e *Editor) WriteNote(app *AppContext) {
	text := e.bufferToString()
	
	// Handle code files separately (maintaining compatibility)
	if taskFolder(e.id) == "code" {
		go updateCodeFile(e.id, text)
	}
	
	// Update the note using our new method
	e.UpdateNote(app)
	
	// Update UI
	e.drawStatusBar()
	app.Session.showEdMessage("isModified = %t", e.isModified())
}

// UpdateNote updates a note in the database
func (e *Editor) UpdateNote(app *AppContext) {
	if len(e.ss) == 0 {
		return
	}
	
	// Get the text from the buffer
	text := e.bufferToString()
	if e.id == -1 {
		return
	}
	
	// Update the note in the database using the DBContext
	app.DBCtx.UpdateNoteWrapper(e.id, text)
	
	// Update the buffer tick to track changes
	e.bufferTick = vim.BufferGetLastChangedTick(e.vbuf)
	
	// Update modified flag
	e.modified = vim.BufferGetModified(e.vbuf)
}

// ReadNote reads a note from the database into the editor
func (e *Editor) ReadNote(app *AppContext, id int) {
	if id == -1 {
		return // id given to new and unsaved entries
	}
	
	// Get the note from the database using the DBContext
	note := app.DBCtx.ReadNoteIntoString(id)
	
	// Split the note into lines
	e.ss = strings.Split(note, "\n")
	
	// Set up the vim buffer
	e.vbuf = vim.BufferNew(0)
	vim.BufferSetCurrent(e.vbuf)
	vim.BufferSetLines(e.vbuf, 0, -1, e.ss, len(e.ss))
	
	// Reset cursor and view position
	e.fr, e.fc, e.cy, e.cx = 0, 0, 0, 0
	e.lineOffset, e.firstVisibleRow = 0, 0
	
	// Set the entry ID and update buffer tick
	e.id = id
	e.bufferTick = vim.BufferGetLastChangedTick(e.vbuf)
	
	// Reset modified state
	e.modified = false
}

// ProcessKey handles key input for the editor
// Returns a bool indicating whether text was changed (needs redraw)
func (e *Editor) ProcessKey(app *AppContext, key int) bool {
	// For now, we'll only handle a few special key combinations
	// and delegate everything else to the global function
	
	// Only handle Ctrl-S in this method for now
  // This was never a method in the application but a Claude hallucination
//	if key == ctrlKey('s') {
//		e.WriteNote(app)
//		return false
//	}
	
	// Let the global function handle everything else
	return editorProcessKey(key)
}
