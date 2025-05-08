package govim

import (
	"io/ioutil"
	"strings"
)

// UndoRecord represents a single undoable change
type UndoRecord struct {
	Changes       map[int]string // Map of line numbers to their previous content (1-based)
	CursorPos     [2]int         // Cursor position before the change [row, col]
	Description   string         // Optional description of the change
	CommandType   string         // The type of command that created this undo record (e.g., "o", "O", "general")
	LineOperation bool           // Whether this operation added or removed entire lines
}

// GoBuffer is the Go implementation of a vim buffer
type GoBuffer struct {
	id             int
	lines          []string
	name           string
	modified       bool
	engine         *GoEngine
	lastTick       int
	undoStack      []*UndoRecord // Stack of undo records (newest at the end)
	redoStack      []*UndoRecord // Stack of redo records (newest at the end)
	lastSavedState int           // Index in the undo stack when buffer was last saved (-1 if never)
	cursorRow      int           // Current cursor row position for this buffer
	cursorCol      int           // Current cursor column position for this buffer
}

// GetID returns the buffer ID
func (b *GoBuffer) GetID() int {
	return b.id
}

// GetLine returns the line at the given line number (1-based indexing)
func (b *GoBuffer) GetLine(lnum int) string {
	if lnum < 1 || lnum > len(b.lines) {
		return ""
	}
	return b.lines[lnum-1]
}

// GetLineB returns the line as a byte slice
func (b *GoBuffer) GetLineB(lnum int) []byte {
	return []byte(b.GetLine(lnum))
}

// Lines returns all lines in the buffer
func (b *GoBuffer) Lines() []string {
	result := make([]string, len(b.lines))
	copy(result, b.lines)
	return result
}

// LinesB returns all lines as byte slices
func (b *GoBuffer) LinesB() [][]byte {
	result := make([][]byte, len(b.lines))
	for i, line := range b.lines {
		result[i] = []byte(line)
	}
	return result
}

// GetLineCount returns the number of lines in the buffer
func (b *GoBuffer) GetLineCount() int {
	return len(b.lines)
}

// SetCurrent makes this buffer the current buffer
func (b *GoBuffer) SetCurrent() {
	b.engine.currentBuffer = b ////////////////////////////////////////////////////////////
	//changes but doesn't fix the issue with the transition from editing a note to returning to organizer
	//b.engine.BufferSetCurrent(b) govim/wrapper.go
}

// IsModified returns whether the buffer has been modified
func (b *GoBuffer) IsModified() bool {
	return b.modified
}

// GetLastChangedTick returns the last changed tick
func (b *GoBuffer) GetLastChangedTick() int {
	return b.lastTick
}

// GetCursorRow returns the buffer's saved cursor row position
func (b *GoBuffer) GetCursorRow() int {
	return b.cursorRow
}

// GetCursorCol returns the buffer's saved cursor column position
func (b *GoBuffer) GetCursorCol() int {
	return b.cursorCol
}

// SetCursorPosition sets the buffer's saved cursor position
func (b *GoBuffer) SetCursorPosition(row, col int) {
	b.cursorRow = row
	b.cursorCol = col
}

// SetLines sets all lines in the buffer
func (b *GoBuffer) SetLines(start, end int, lines []string) {
	// Add panic recovery with detailed logging
	/*
		defer func() {
			if r := recover(); r != nil {
				// Log the panic with detailed information
				println("PANIC in GoBuffer.SetLines:", r)
				println("Buffer ID:", b.id)
				println("Start:", start)
				println("End:", end)
				println("Buffer line count:", len(b.lines))
				println("Lines length:", len(lines))

				// Try to recover by doing a safer operation
				try := func() (ok bool) {
					// Extra safety - recover from panics in the recovery code itself
					defer func() {
						if r := recover(); r != nil {
							println("Recovery failed:", r)
							ok = false
						}
					}()

					// Make sure we have valid lines
					if b.lines == nil {
						b.lines = []string{""}
					}

					// Safest possible operation: append an empty line if needed
					if len(b.lines) == 0 {
						b.lines = []string{""}
						ok = true
						return
					}

					// For simple single-line updates (like from 'x' command), try direct update
					if len(lines) == 1 && start >= 0 && start < len(b.lines) {
						// Copy existing lines for safety
						allLines := make([]string, len(b.lines))
						copy(allLines, b.lines)

						// Replace just the one line
						allLines[start] = lines[0]

						// Set the lines directly
						b.lines = allLines
						b.markModified()
						ok = true
					}

					return
				}

				// Try the recovery
				if recovered := try(); recovered {
					println("Recovery succeeded")
				} else {
					println("Could not recover - buffer may be in inconsistent state")
				}
			}
		}()
	*/
	// Validate inputs to prevent panics
	if b.lines == nil {
		b.lines = []string{""}
	}

	// Save state for undo before making changes
	// Convert from 0-based to 1-based line numbering for undo save
	if b.engine != nil && !b.engine.inInsertUndoGroup {
		// Only save undo state if we're not in an insert group
		// This is because all insert mode changes are treated as a single undo operation
		if start == 0 && end == -1 {
			// For complete buffer replacement, save all lines
			b.engine.UndoSaveRegion(1, len(b.lines))
		} else {
			// For partial replacements, save the affected region
			// Need to add 1 because UndoSaveRegion expects 1-based line numbers
			startLine := start + 1
			endLine := end + 1
			if endLine > len(b.lines) {
				endLine = len(b.lines)
			}
			if startLine <= endLine {
				b.engine.UndoSaveRegion(startLine, endLine)
			}
		}
	}

	// Create a completely new buffer and return - this is the most radical approach
	// to ensure we get a complete replacement for the special case
	if start == 0 && end == -1 {
		// Make a clean copy of the input lines
		cleanLines := make([]string, len(lines))
		for i, line := range lines {
			cleanLines[i] = line // Deep copy each string
		}

		// Make sure we have at least an empty line
		if len(cleanLines) == 0 {
			cleanLines = []string{""}
		}

		// COMPLETELY replace the lines array - create a new slice to break all references
		b.lines = make([]string, len(cleanLines))
		copy(b.lines, cleanLines)
		b.markModified()

		return
	}

	// For all other cases (partial replacements), continue with normal logic

	// Ensure we have a valid lines array to insert
	if lines == nil {
		lines = []string{}
	}

	// Bounds check for start and end
	if start < 0 {
		start = 0
	}

	// For partial replacements, continue with the standard logic
	if end == -1 {
		end = len(b.lines)
	}

	// Ensure start <= end <= len(b.lines)
	if start > len(b.lines) {
		start = len(b.lines)
	}

	if end > len(b.lines) {
		end = len(b.lines)
	}

	if start > end {
		start = end
	}

	// Calculate number of lines to replace
	count := end - start

	// Calculate the capacity needed for the new lines array
	newCapacity := len(b.lines) - count + len(lines)
	if newCapacity < 1 {
		newCapacity = 1 // Ensure at least one line
	}

	// Create new lines array with replaced content
	newLines := make([]string, 0, newCapacity)

	// Add lines before the replacement
	if start > 0 && start <= len(b.lines) {
		newLines = append(newLines, b.lines[:start]...)
	}

	// Add the new lines - make a deep copy
	for _, line := range lines {
		newLines = append(newLines, line)
	}

	// Add lines after the replacement
	if end < len(b.lines) {
		newLines = append(newLines, b.lines[end:]...)
	}

	// Ensure we always have at least one line
	if len(newLines) == 0 {
		newLines = []string{""}
	}

	// Update the buffer's lines - ensure we break any reference to the old lines
	b.lines = newLines
	b.markModified()
}

// Mark the buffer as modified and increment the change tick
func (b *GoBuffer) markModified() {
	// Set modified flag to trigger UI updates
	b.modified = true

	// Increment last tick to indicate change
	b.lastTick++
}

// Load a file into the buffer
func (b *GoBuffer) loadFile(filename string) error {
	// Add panic recovery for any unexpected issues
	defer func() {
		if r := recover(); r != nil {
			println("PANIC in GoBuffer.loadFile:", r)
			println("Filename:", filename)

			// Ensure we at least have a minimally valid buffer state
			if b.lines == nil || len(b.lines) == 0 {
				b.lines = []string{""}
			}
			b.name = filename
			b.modified = false
		}
	}()

	// Try to read the file
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		// File couldn't be read, but don't fail catastrophically
		// Set up an empty buffer with the right name
		b.name = filename
		b.lines = []string{""}
		b.modified = false
		b.undoStack = make([]*UndoRecord, 0)
		b.redoStack = make([]*UndoRecord, 0)
		b.lastSavedState = -1
		return err
	}

	// Save the filename
	b.name = filename

	// Split the content by newlines
	contentStr := string(content)

	// Check if the file contains CR+LF line endings (Windows)
	if strings.Contains(contentStr, "\r\n") {
		// Handle Windows line endings
		b.lines = strings.Split(contentStr, "\r\n")
	} else {
		// Handle Unix line endings
		b.lines = strings.Split(contentStr, "\n")
	}

	// Make sure we have at least one line
	if len(b.lines) == 0 {
		b.lines = []string{""}
	}

	// Initialize undo/redo stacks
	b.undoStack = make([]*UndoRecord, 0)
	b.redoStack = make([]*UndoRecord, 0)
	b.lastSavedState = -1

	// Reset the modified flag
	b.modified = false

	return nil
}
