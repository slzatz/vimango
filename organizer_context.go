//go:build exclude
package main

import (
	"fmt"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/slzatz/vimango/vim"
)

/*
The Organizer context implementation provides a context-based approach to the organizer functionality.
This is part of the ongoing refactoring effort to move global state into appropriate context objects.

Future work:
1. Fully implement the placeholder methods with the actual logic from the global functions
2. Update other files that call the global functions to use these context-based methods
3. Consider moving organizer display methods to this file as well
4. Gradually deprecate the global organizerProcessKey function once all calls are moved to the context
5. Consider creating methods that work directly with the DBContext instead of calling global DB functions
*/

// ProcessKey handles key input for the organizer
// This is a context-based replacement for the global organizerProcessKey function
func (o *Organizer) ProcessKey(app *AppContext, c int) {
	session := app.Session

	if c == '\x1b' {
		session.ShowOrgMessage("")
		o.command = ""
		vim.Key("<esc>")
		o.last_mode = o.mode
		o.mode = NORMAL
		pos := vim.CursorGetPosition()
		o.fc = pos[1]
		o.fr = pos[0] - 1
		tabCompletion.idx = 0
		return
	}

	switch o.mode {
	case NORMAL:
		o.processNormalModeKey(app, c)
	case FIND:
		o.processFindModeKey(app, c)
	case COMMAND_LINE:
		o.processCommandLineModeKey(app, c)
	case PREVIEW_SYNC_LOG:
		o.processPreviewSyncLogModeKey(app, c)
	case INSERT_OUTPUT, INSERT_NEW_ROW:
		o.processInsertModeKey(app, c)
	}
}

// processNormalModeKey handles keys in normal mode
func (o *Organizer) processNormalModeKey(app *AppContext, c int) {
	session := app.Session

	// Check for command like 'dd' from n_lookup
	if _, err := strconv.Atoi(string(c)); err != nil {
		o.command += string(c)
	}

	// Check if we have a matching command in n_lookup
	if cmd, found := n_lookup[o.command]; found {
		// This will execute global functions for now, but should be
		// migrated to context-based methods in the future
		cmd()
		o.command = ""
		vim.Key("<esc>")
		return
	}

	// Special case for control-l from previous mode
	if c == ctrlKey('l') && o.last_mode == ADD_CHANGE_FILTER {
		o.mode = ADD_CHANGE_FILTER
		session.EraseRightScreen()
	}

	// Handle enter key in NORMAL mode
	if c == '\r' {
		o.command = ""
		row := &o.rows[o.fr]
		if row.dirty {
			o.writeTitle()
			vim.Key("<esc>")
			row.dirty = false
			o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
			return
		}

		// If not row.dirty and in a container view, open the entries with that container
		var tid int
		switch o.view {
		case TASK:
			return
		case CONTEXT:
			o.taskview = BY_CONTEXT
			tid, _ = contextExists(row.title)
		case FOLDER:
			o.taskview = BY_FOLDER
			tid, _ = folderExists(row.title)
		case KEYWORD:
			o.taskview = BY_KEYWORD
			tid, _ = keywordExists(row.title)
		}

		// If it's a new context|folder|keyword we can't filter tasks by it
		if tid < 1 {
			session.ShowOrgMessage("You need to sync before you can use %q", row.title)
			return
		}
		o.filter = row.title
		session.ShowOrgMessage("'%s' will be opened", o.filter)

		o.clearMarkedEntries()
		o.view = TASK
		o.fc, o.fr, o.rowoff = 0, 0, 0
		o.rows = app.DBCtx.FilterEntries(o.taskview, o.filter, o.show_deleted, o.sort, o.sortPriority, MAX)
		if len(o.rows) == 0 {
			o.insertRow(0, "", true, false, false, BASE_DATE)
			o.rows[0].dirty = false
			session.ShowOrgMessage("No results were returned")
		}
		session.imagePreview = false
		o.readRowsIntoBuffer()
		vim.CursorSetPosition(1, 0)
		o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
		o.drawPreview()
		return
	}

	// Special case for handling keys that should be ignored
	// In normal mode, don't want leader, 'O', 'V', etc. passed to vim
	if c == int([]byte(leader)[0]) || c == 'O' || c == 'V' || c == ctrlKey('v') || c == 'o' || c == 'J' {
		if c != int([]byte(leader)[0]) {
			session.ShowOrgMessage("Ascii %d has no effect in Organizer NORMAL mode", c)
		}
		return
	}

	// Handle specific commands based on key
	switch c {
	case ctrlKey('q'):
		session.QuitApp()

	case ':':
		o.enterCommandLineMode(app)

	case '/':
		o.enterFindMode(app)

	case 'e':
		// Edit entry
		id := o.getId()
		if id == -1 {
			session.ShowOrgMessage("Can't edit item with an id of -1")
			return
		}
		o.editEntry(app, id)

	case '*':
		// Toggle star
		o.toggleStar(app)

	case 'x':
		// Toggle completed
		o.toggleCompleted(app)

	case 'D':
		// Delete/archive entry
		o.deleteEntry(app)

	case 'R':
		// Restore entry
		o.restoreEntry(app)

	case 'p':
		// Preview entry
		o.previewEntry(app)

	case ' ':
		// Mark entry
		o.markEntry(app)

	case 'c':
		// Change title
		o.changeTitle(app)

	case 'i':
		// Insert new row
		o.insertNewRow(app)

	case 'o':
		// New output
		o.newOutput(app)

	case 'r':
		// Reload entries
		o.reloadEntries(app)

	case 'f':
		// Toggle filter
		o.toggleFilter(app)

	case 'C':
		// Toggle context/folder view
		o.toggleContextFolderView(app)

	case 'a':
		// Change context
		o.changeContext(app)

	case 'A':
		// Change folder
		o.changeFolder(app)

	case 'y':
		// Yank entry
		o.yankEntry(app)

	case '\t':
		// Preview with markdown
		o.previewMarkdown(app)

	case ctrlKey('l'):
		// Switch to editor mode
		o.switchToEditorMode(app)

	case ctrlKey('n'):
		// Draw preview with images
		o.drawPreviewWithImages(app)

	default:
		// Send the keystroke to vim for standard keys
		if z, found := termcodes[c]; found {
			vim.Key(z)
			session.ShowEdMessage("%s", z)
		} else {
			vim.Input(string(rune(c)))
		}

		// Update cursor position and handle row changes
		pos := vim.CursorGetPosition()
		o.fc = pos[1]

		// If moved to a new row, draw task note preview or container info
		if o.fr != pos[0]-1 {
			o.fr = pos[0] - 1
			o.fc = 0
			vim.CursorSetPosition(o.fr+1, 0)
			o.altRowoff = 0
			if o.view == TASK {
				o.drawPreview()
			} else {
				session.DisplayContainerInfo(app)
			}
		}

		// Update row title and dirty status
		s := vim.BufferLines(o.vbuf)[o.fr]
		o.rows[o.fr].title = s
		row := &o.rows[o.fr]
		tick := vim.BufferGetLastChangedTick(o.vbuf)
		if tick > o.bufferTick {
			row.dirty = true
			o.bufferTick = tick
		}

		// Handle vim mode
		mode := vim.GetMode()
		// OP_PENDING like 4da
		if mode == 4 {
			return
		}
		o.command = ""
	}
}

// processFindModeKey handles keys in find mode
func (o *Organizer) processFindModeKey(app *AppContext, c int) {
	session := app.Session

	switch c {
	case '\r': // Enter key - perform search
		if len(o.command_line) == 0 {
			o.mode = NORMAL
			return
		}

		o.title_search_string = o.command_line
		o.command_line = ""
		o.mode = NORMAL

		// Search for string in titles
		o.fr, o.fc = 0, 0
		o.findNextWord()

	case ctrlKey('h'), 127: // backspace/delete
		if len(o.command_line) > 0 {
			o.command_line = o.command_line[:len(o.command_line)-1]
		}

	default:
		if c >= 32 && c < 127 { // Printable ASCII
			o.command_line += string(rune(c))
		}
	}

	session.ShowOrgMessage("/%s", o.command_line)
}

// processCommandLineModeKey handles keys in command line mode
func (o *Organizer) processCommandLineModeKey(app *AppContext, c int) {
	session := app.Session

	switch c {
	case '\r': // Enter key - execute command
		// Save and restore mode
		last_mode := o.last_mode
		o.mode = last_mode

		// Execute the command
		o.executeCommandLine(app)

	case '\t': // Tab - command completion
		// Check if there's a command to complete
		if o.command_line == "" {
			return
		}

		// Try to complete the command
		if tabCompletion.list == nil {
			tabCompletion.idx = 0
			tabCompletion.list = []string{}

			// Get list of possible commands based on command line
			cli := o.command_line
			for cmd := range ex_lookup {
				if strings.HasPrefix(cmd, cli) {
					tabCompletion.list = append(tabCompletion.list, cmd)
				}
			}

			// Sort the list for consistent results
			sort.Strings(tabCompletion.list)
		}

		// If we have completions, cycle through them
		if len(tabCompletion.list) > 0 {
			o.command_line = tabCompletion.list[tabCompletion.idx]
			tabCompletion.idx = (tabCompletion.idx + 1) % len(tabCompletion.list)
			session.ShowOrgMessage(":%s", o.command_line)
		}

	case ctrlKey('h'), 127: // backspace/delete
		if len(o.command_line) > 0 {
			o.command_line = o.command_line[:len(o.command_line)-1]
			session.ShowOrgMessage(":%s", o.command_line)
		}

		// Reset command completion
		tabCompletion.idx = 0
		tabCompletion.list = nil

	default:
		if c >= 32 && c < 127 { // Printable ASCII
			o.command_line += string(rune(c))
			session.ShowOrgMessage(":%s", o.command_line)

			// Reset command completion
			tabCompletion.idx = 0
			tabCompletion.list = nil
		}
	}
}

// processPreviewSyncLogModeKey handles keys in preview/sync/log mode
func (o *Organizer) processPreviewSyncLogModeKey(app *AppContext, c int) {
	session := app.Session

	switch c {
	case ':':
		o.enterCommandLineMode(app)

	case ctrlKey('j'), ctrlKey('n'):
		// Scroll down
		o.altRowoff++
		o.drawPreview()

	case ctrlKey('k'), ctrlKey('p'):
		// Scroll up
		if o.altRowoff > 0 {
			o.altRowoff--
		}
		o.drawPreview()

	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		// In LINKS mode, handle numeric selections for links
		if o.mode == LINKS {
			num := c - '0'
			// This would call a function to open the link by number
			session.ShowOrgMessage("Selected link %d", num)
		}

	case 'q':
		// Quit preview mode
		o.mode = NORMAL
		session.ShowOrgMessage("")
	}
}

// processInsertModeKey handles keys in insert modes
func (o *Organizer) processInsertModeKey(app *AppContext, c int) {
	session := app.Session

	if c == '\r' {
		// Enter key - finish editing
		o.writeTitle()
		vim.Key("<esc>")
		o.mode = NORMAL
		row := &o.rows[o.fr]
		row.dirty = false
		o.bufferTick = vim.BufferGetLastChangedTick(o.vbuf)
		o.command = ""
		session.ShowOrgMessage("")
		return
	}

	// Send characters to vim
	if z, found := termcodes[c]; found {
		vim.Key(z)
	} else {
		vim.Input(string(rune(c)))
	}

	// Update the title from the buffer
	s := vim.BufferLines(o.vbuf)[o.fr]
	o.rows[o.fr].title = s

	// Get and manage cursor position
	pos := vim.CursorGetPosition()
	o.fc = pos[1]

	// Prevent row from changing in INSERT mode
	// For instance, when an up or down arrow is pressed
	if o.fr != pos[0]-1 {
		vim.CursorSetPosition(o.fr+1, o.fc)
	}

	// Keep track of changes for dirty flag
	row := &o.rows[o.fr]
	tick := vim.BufferGetLastChangedTick(o.vbuf)
	if tick > o.bufferTick {
		row.dirty = true
		o.bufferTick = tick
	}
}

// Helper functions for key handling

// enterCommandLineMode switches to command line mode
func (o *Organizer) enterCommandLineMode(app *AppContext) {
	session := app.Session

	o.mode = COMMAND_LINE
	o.command_line = ""
	session.ShowOrgMessage(":")
}

// enterFindMode switches to find mode
func (o *Organizer) enterFindMode(app *AppContext) {
	session := app.Session

	o.mode = FIND
	o.command_line = ""
	session.ShowOrgMessage("/")
}

// savePosition saves the current cursor position
func (o *Organizer) savePosition() {
	// Placeholder for saving position when editing
}

// editEntry opens the selected entry for editing
func (o *Organizer) editEntry(app *AppContext, id int) {
	session := app.Session
	editor := app.Editor

	// Switch to editor mode
	session.editorMode = true

	// Read the note into the editor
	editor.ReadNote(app, id)

	// Update displays
	editor.drawText()
	editor.drawStatusBar()
	session.ReturnCursor(app)
}

// switchToEditorMode switches to editor mode
func (o *Organizer) switchToEditorMode(app *AppContext) {
	session := app.Session

	if len(app.Windows) == 0 {
		session.ShowOrgMessage("There are no active editors")
		return
	}
	session.EraseRightScreen()
	session.DrawRightScreen(app)
	session.editorMode = true
	vim.BufferSetCurrent(app.Editor.vbuf)
}

// toggleStar toggles the star status of an entry
func (o *Organizer) toggleStar(app *AppContext) {
	session := app.Session

	if len(o.rows) == 0 {
		return
	}

	row := &o.rows[o.fr]
	row.star = !row.star

	// Call to database function - this should eventually use DBContext
	starTask(row.id, row.star)
	session.ShowOrgMessage("Toggled star for task %d", row.id)
}

// toggleCompleted toggles completion status of an entry
func (o *Organizer) toggleCompleted(app *AppContext) {
	session := app.Session

	if len(o.rows) == 0 {
		return
	}

	row := &o.rows[o.fr]
	row.archived = !row.archived

	// Call to database function - this should eventually use DBContext
	archiveTask(row.id, row.archived)
	if row.archived {
		session.ShowOrgMessage("Completed task %d", row.id)
	} else {
		session.ShowOrgMessage("Uncompleted task %d", row.id)
	}
}

// deleteEntry marks an entry as deleted
func (o *Organizer) deleteEntry(app *AppContext) {
	session := app.Session

	if len(o.rows) == 0 {
		return
	}

	row := &o.rows[o.fr]
	row.deleted = true

	// Call to database function - this should eventually use DBContext
	deleteTask(row.id)
	session.ShowOrgMessage("Deleted task %d", row.id)
}

// restoreEntry restores a deleted entry
func (o *Organizer) restoreEntry(app *AppContext) {
	session := app.Session

	if len(o.rows) == 0 {
		return
	}

	row := &o.rows[o.fr]
	row.deleted = false

	// Call to database function - this should eventually use DBContext
	undeleteTask(row.id)
	session.ShowOrgMessage("Restored task %d", row.id)
}

// previewEntry shows a preview of the current entry
func (o *Organizer) previewEntry(app *AppContext) {
	if len(o.rows) == 0 {
		return
	}

	o.drawPreview()
}

// drawPreviewWithImages shows a preview with images
func (o *Organizer) drawPreviewWithImages(app *AppContext) {
	session := app.Session

	session.EraseRightScreen()
	o.drawPreviewWithImages()
	session.imagePreview = true
}

// markEntry marks/unmarks an entry
func (o *Organizer) markEntry(app *AppContext) {
	session := app.Session

	if o.view != TASK {
		session.ShowOrgMessage("You can only mark tasks")
		return
	}

	if len(o.rows) == 0 {
		return
	}

	// Toggle the mark for this entry
	if _, found := o.marked_entries[o.rows[o.fr].id]; found {
		delete(o.marked_entries, o.rows[o.fr].id)
	} else {
		o.marked_entries[o.rows[o.fr].id] = struct{}{}
	}
	session.ShowOrgMessage("Toggle mark for item %d", o.rows[o.fr].id)
}

// moveCursor handles cursor movement keys
func (o *Organizer) moveCursor(app *AppContext, key int) {
	// This would implement cursor movement similar to the global function
	vim.Input(string(rune(key)))
}

// navigateToPosition handles H, M, L navigation
func (o *Organizer) navigateToPosition(app *AppContext, key int) {
	// Implementation of navigating to top/middle/bottom of screen
	vim.Input(string(rune(key)))
}

// navigateHorizontally handles left/right navigation
func (o *Organizer) navigateHorizontally(app *AppContext, key int) {
	// Implementation of horizontal navigation
	vim.Input(string(rune(key)))
}

// pageNavigation handles page up/down and half-page navigation
func (o *Organizer) pageNavigation(app *AppContext, key int) {
	// Implementation of page navigation
	vim.Input(string(rune(key)))
}

// changeSortOrder changes the sort order of entries
func (o *Organizer) changeSortOrder(app *AppContext, key int) {
	// Implementation of changing sort order
	// This would be derived from the global key handler
}

// toggleStar toggles the star status of an entry
func (o *Organizer) toggleStar(app *AppContext) {
	// Implementation of toggling star
	// This would be derived from the global key handler
}

// toggleCompleted toggles completion status
func (o *Organizer) toggleCompleted(app *AppContext) {
	// Implementation of toggling completion
	// This would be derived from the global key handler
}

// deleteEntry marks an entry as deleted
func (o *Organizer) deleteEntry(app *AppContext) {
	// Implementation of deleting an entry
	// This would be derived from the global key handler
}

// restoreEntry restores a deleted entry
func (o *Organizer) restoreEntry(app *AppContext) {
	// Implementation of restoring an entry
	// This would be derived from the global key handler
}

// previewEntry shows a preview of the current entry
func (o *Organizer) previewEntry(app *AppContext) {
	// Implementation of showing a preview
	o.drawPreview()
}

// markEntry marks/unmarks an entry
func (o *Organizer) markEntry(app *AppContext) {
	// Implementation of marking an entry
	// This would be derived from the global key handler
}

// changeTitle changes the title of an entry
func (o *Organizer) changeTitle(app *AppContext) {
	// Implementation of changing a title
	// This would be derived from the global key handler
}

// insertNewRow inserts a new row
func (o *Organizer) insertNewRow(app *AppContext) {
	// Implementation of inserting a new row
	// This would be derived from the global key handler
}

// newOutput creates a new output window
func (o *Organizer) newOutput(app *AppContext) {
	// Implementation of creating a new output
	// This would be derived from the global key handler
}

// reloadEntries reloads the entries
func (o *Organizer) reloadEntries(app *AppContext) {
	// Implementation of reloading entries
	// This would be derived from the global key handler
}

// toggleFilter toggles filter options
func (o *Organizer) toggleFilter(app *AppContext) {
	// Implementation of toggling filters
	// This would be derived from the global key handler
}

// toggleContextFolderView toggles between context and folder views
func (o *Organizer) toggleContextFolderView(app *AppContext) {
	// Implementation of toggling views
	// This would be derived from the global key handler
}

// changeContext changes the context
func (o *Organizer) changeContext(app *AppContext) {
	// Implementation of changing context
	// This would be derived from the global key handler
}

// changeFolder changes the folder
func (o *Organizer) changeFolder(app *AppContext) {
	// Implementation of changing folder
	// This would be derived from the global key handler
}

// yankEntry copies an entry
func (o *Organizer) yankEntry(app *AppContext) {
	// Implementation of yanking an entry
	// This would be derived from the global key handler
}

// previewMarkdown shows a markdown preview
func (o *Organizer) previewMarkdown(app *AppContext) {
	// Implementation of showing markdown preview
	// This would be derived from the global key handler
}

// quickEdit performs a quick edit
func (o *Organizer) quickEdit(app *AppContext) {
	// Implementation of quick edit
	// This would be derived from the global key handler
}

// findNextWord finds the next occurrence of search string
func (o *Organizer) findNextWord() {
	if o.title_search_string == "" {
		return
	}

	if len(o.rows) == 0 {
		return
	}

	searchTerm := strings.ToLower(o.title_search_string)
	startRow := o.fr

	// Start from next row
	o.fr++
	if o.fr >= len(o.rows) {
		o.fr = 0 // Wrap around
	}

	// Search all rows, wrapping around if necessary
	for i := 0; i < len(o.rows); i++ {
		title := strings.ToLower(o.rows[o.fr].title)
		if strings.Contains(title, searchTerm) {
			// Found a match
			vim.CursorSetPosition(o.fr+1, 0)
			pos := vim.CursorGetPosition()
			o.fc = pos[1]
			return
		}

		// Move to next row
		o.fr++
		if o.fr >= len(o.rows) {
			o.fr = 0 // Wrap around
		}

		// If we're back to where we started, no match found
		if o.fr == startRow {
			break
		}
	}

	// If no match found, stay at the original position
	o.fr = startRow
	vim.CursorSetPosition(o.fr+1, 0)
}

// clearMarkedEntries clears all marked entries
func (o *Organizer) clearMarkedEntries() {
	o.marked_entries = make(map[int]struct{})
}

// writeTitle updates the title in the database
func (o *Organizer) writeTitle() {
	if len(o.rows) == 0 {
		return
	}

	pos := vim.CursorGetPosition()
	o.fr = pos[0] - 1
	o.fc = pos[1]

	// Get the latest title from the buffer
	s := vim.BufferLines(o.vbuf)[o.fr]
	o.rows[o.fr].title = s

	// If ID is valid, update the database
	if o.rows[o.fr].id > 0 {
		row := &o.rows[o.fr]

		// Differentiate between different container types
		switch o.view {
		case TASK:
			updateTaskTitle(row.id, row.title)
		case CONTEXT:
			updateContextTitle(row.id, row.title)
		case FOLDER:
			updateFolderTitle(row.id, row.title)
		case KEYWORD:
			updateKeywordTitle(row.id, row.title)
		}
	}
}

// executeCommandLine executes a command from the command line
func (o *Organizer) executeCommandLine(app *AppContext) {
	session := app.Session
	cmdLine := o.command_line
	o.command_line = ""

	if cmdLine == "" {
		return
	}

	// Handle commands with arguments
	parts := strings.SplitN(cmdLine, " ", 2)
	cmd := parts[0]
	var arg string
	if len(parts) > 1 {
		arg = parts[1]
	}

	// Look for the command in the ex_lookup map
	if fn, found := ex_lookup[cmd]; found {
		// Execute the command
		fn(arg)
		return
	}

	// If command not found, show an error
	session.ShowOrgMessage("Unknown command: %s", cmd)
}

// enterFindMode switches to find mode
func (o *Organizer) enterFindMode(app *AppContext) {
	session := app.Session

	o.mode = FIND
	o.command_line = ""
	session.ShowOrgMessage("/")
}

// getId gets the ID of the current entry
func (o *Organizer) getId() int {
	if len(o.rows) == 0 || o.fr >= len(o.rows) {
		return -1
	}
	return o.rows[o.fr].id
}

// Note: This is a framework for the Organizer context that would need
// to be filled out with the actual implementations from the global functions.
// Each method is currently a placeholder that would need to be expanded with
// the logic from the corresponding global function.
