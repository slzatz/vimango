package govim

import (
	// No imports needed here
)

// ModeNormal is the normal mode constant
const ModeNormal = 1

// ModeVisual is the visual mode constant
const ModeVisual = 2

// ModeCommand is the command mode constant (for ex commands)
const ModeCommand = 8

// ModeInsert is the insert mode constant
const ModeInsert = 16

// ModeSearch is the search mode constant
const ModeSearch = 32

// GoEngine implements the vim engine in pure Go
type GoEngine struct {
	buffers         map[int]*GoBuffer
	nextBufferId    int
	currentBuffer   *GoBuffer
	cursorRow       int
	cursorCol       int
	mode            int
	visualStart     [2]int
	visualEnd       [2]int
	visualType      int
	commandCount    int  // For motion counts like 5j, 3w, etc.
	awaitingMotion  bool // True when waiting for a motion after d, c, y, etc.
	currentCommand  string // Current command (d, c, y) waiting for motion
	buildingCount   bool   // True when we're in the process of entering a numeric prefix
	yankRegister    string // Content of the "unnamed" register for yank/put
	
	// Undo state
	inInsertUndoGroup bool // True when in insert mode to group all changes as one undo operation
	
	// Search state
	searchPattern     string  // Current search pattern
	searchDirection   int     // 1 for forward (/) and -1 for backward (?)
	searchResults     [][2]int // List of [row, col] positions matching the search
	currentSearchIdx  int     // Index of current search result
	searching         bool    // True when in search mode (typing the search pattern)
	searchBuffer      string  // Buffer for search input
}

// NewEngine creates a new vim engine
func NewEngine() *GoEngine {
	return &GoEngine{
		buffers:         make(map[int]*GoBuffer),
		nextBufferId:    1,
		mode:            ModeNormal,
		commandCount:    0,
		awaitingMotion:  false,
		currentCommand:  "",
		buildingCount:   false,
		yankRegister:    "",
		inInsertUndoGroup: false,
		searchPattern:   "",
		searchDirection: 1,  // Default to forward search
		searchResults:   make([][2]int, 0),
		currentSearchIdx: -1,
		searching:       false,
		searchBuffer:    "",
	}
}

// Init initializes the engine
func (e *GoEngine) Init(argc int) {
	// Nothing significant to do in the Go implementation
}

// BufferOpen opens a file and returns a buffer
func (e *GoEngine) BufferOpen(filename string, lnum int, flags int) Buffer {
	buf := &GoBuffer{
		id:             e.nextBufferId,
		engine:         e,
		undoStack:      make([]*UndoRecord, 0),
		redoStack:      make([]*UndoRecord, 0),
		lastSavedState: -1, // Initialize to -1 (no saved state yet)
	}
	e.nextBufferId++
	
	// Load the file (ignoring errors for now)
	buf.loadFile(filename)
	
	// Set as current
	e.currentBuffer = buf
	e.buffers[buf.id] = buf
	
	// Set cursor position
	if lnum > 0 && lnum <= buf.GetLineCount() {
		e.cursorRow = lnum
	} else {
		e.cursorRow = 1
	}
	e.cursorCol = 0
	
	return buf
}

// BufferNew creates a new empty buffer
func (e *GoEngine) BufferNew(flags int) Buffer {
	buf := &GoBuffer{
		id:             e.nextBufferId,
		engine:         e,
		lines:          []string{""},
		undoStack:      make([]*UndoRecord, 0),
		redoStack:      make([]*UndoRecord, 0),
		lastSavedState: -1, // Initialize to -1 (no saved state yet)
	}
	e.nextBufferId++
	
	e.buffers[buf.id] = buf
	return buf
}

// BufferGetCurrent returns the current buffer
func (e *GoEngine) BufferGetCurrent() Buffer {
	return e.currentBuffer
}

// BufferSetCurrent sets the current buffer
func (e *GoEngine) BufferSetCurrent(buf Buffer) {
	if goBuf, ok := buf.(*GoBuffer); ok {
		// Save the old buffer reference to check if we're changing buffers
		oldBuffer := e.currentBuffer
		
		// Set the new current buffer
		e.currentBuffer = goBuf
		
		// Ensure the buffer has at least one line (empty buffer protection)
		if len(goBuf.lines) == 0 {
			goBuf.lines = []string{""}
		}
		
		// Always reset state for any buffer change to avoid stale state
		// This is especially important for handling new notes correctly
		
		// Reset cursor position to beginning of file
		e.cursorRow = 1
		e.cursorCol = 0
		
		// Reset buffer-specific state
		e.commandCount = 0
		e.buildingCount = false
		e.awaitingMotion = false
		e.currentCommand = ""
		e.mode = ModeNormal // Always reset to normal mode
		
		// Reset visual mode state
		e.visualStart = [2]int{1, 0}
		e.visualEnd = [2]int{1, 0}
		
		// Reset search state
		e.searching = false
		e.searchBuffer = ""
		e.searchResults = nil
		e.currentSearchIdx = -1
		
		// Reset undo state
		e.inInsertUndoGroup = false
		
		// Extra debugging to verify buffer content independence
		if oldBuffer != nil && goBuf != nil {
			oldBufferLines := 0
			if oldBuffer != nil {
				oldBufferLines = len(oldBuffer.lines)
			}
			newBufferLines := len(goBuf.lines)
			
			// Perform additional validation of buffer content
			if oldBufferLines > 0 && newBufferLines > 0 {
				// If the new buffer has an empty first line and fewer lines than the old buffer,
				// there might be a reference issue. Create a completely fresh slice.
				if goBuf.lines[0] == "" && newBufferLines < oldBufferLines {
					newLines := make([]string, len(goBuf.lines))
					for i, line := range goBuf.lines {
						newLines[i] = line // Deep copy each string
					}
					goBuf.lines = newLines
				}
			}
		}
	}
}

// CursorGetLine returns the current cursor line
func (e *GoEngine) CursorGetLine() int {
	return e.cursorRow
}

// CursorGetPosition returns the cursor position as [row, col]
func (e *GoEngine) CursorGetPosition() [2]int {
	// First, validate the current cursor position to ensure it's safe
	if e.currentBuffer != nil {
		// Validate row position
		lineCount := e.currentBuffer.GetLineCount()
		if e.cursorRow < 1 {
			e.cursorRow = 1
		} else if e.cursorRow > lineCount {
			e.cursorRow = lineCount
			if e.cursorRow < 1 {
				e.cursorRow = 1
			}
		}
		
		// Validate column position
		lineLen := 0
		if e.cursorRow >= 1 && e.cursorRow <= lineCount {
			lineLen = len(e.currentBuffer.GetLine(e.cursorRow))
		}
		
		// Make sure cursor column is valid
		if e.cursorCol < 0 {
			e.cursorCol = 0
		} else if e.cursorCol > lineLen {
			e.cursorCol = lineLen
		}
	}
	
	// Return the validated cursor position
	return [2]int{e.cursorRow, e.cursorCol}
}

// CursorSetPosition sets the cursor position
func (e *GoEngine) CursorSetPosition(row, col int) {
	if e.currentBuffer == nil {
		return
	}
	
	// Ensure valid position
	if row < 1 {
		row = 1
	} else if row > e.currentBuffer.GetLineCount() {
		row = e.currentBuffer.GetLineCount()
	}
	
	lineLen := len(e.currentBuffer.GetLine(row))
	if col < 0 {
		col = 0
	} else if col > lineLen {
		col = lineLen
	}
	
	e.cursorRow = row
	e.cursorCol = col
}

// GetMode returns the current mode
func (e *GoEngine) GetMode() int {
	return e.mode
}

// GetCurrentMode returns the current mode
// This is specifically for the wrapper function GetCurrentMode()
// Takes special care to return the expected values for command/ex mode
func (e *GoEngine) GetCurrentMode() int {
	// When in ModeCommand, the application expects value 8 to trigger
	// intercepting the ':' and entering EX_COMMAND mode
	if e.mode == ModeCommand {
		return 8 // The value expected by the editor for command mode
	}
	return e.mode
}

// VisualGetRange returns the visual selection range
func (e *GoEngine) VisualGetRange() [2][2]int {
	return [2][2]int{e.visualStart, e.visualEnd}
}

// VisualGetType returns the visual selection type
func (e *GoEngine) VisualGetType() int {
	return e.visualType
}

// Execute executes a vim command
func (e *GoEngine) Execute(cmd string) {
	// Basic implementation - could be extended later for more sophisticated command parsing
	
	// Most commonly used commands
	switch {
	case cmd == "normal":
		e.mode = ModeNormal
	case cmd == "visual":
		e.mode = ModeVisual
	case cmd == "insert":
		e.mode = ModeInsert
	case cmd == "write" || cmd == "w":
		// Simulated file write (doesn't actually write yet)
		if e.currentBuffer != nil {
			e.currentBuffer.modified = false
			// Update the lastSavedState to the current position in the undo stack
			e.currentBuffer.lastSavedState = len(e.currentBuffer.undoStack)
		}
	}
}

// Eval evaluates a vim expression
func (e *GoEngine) Eval(expr string) string {
	// Minimal implementation of common expression evaluation
	// Could be extended later for more sophisticated expression parsing
	
	if expr == "mode()" {
		switch e.mode {
		case ModeNormal:
			return "n"
		case ModeInsert:
			return "i"
		case ModeVisual:
			return "v"
		case ModeCommand:
			return "c"
		case ModeSearch:
			return "s"
		default:
			return "n"
		}
	}
	
	// Default empty result for unsupported expressions
	return ""
}

// SearchGetMatchingPair finds the matching bracket for the character under the cursor
func (e *GoEngine) SearchGetMatchingPair() [2]int {
	if e.currentBuffer == nil {
		return [2]int{0, 0}
	}
	
	// Basic implementation for bracket matching
	row := e.cursorRow
	col := e.cursorCol
	line := e.currentBuffer.GetLine(row)
	
	if col >= len(line) {
		return [2]int{0, 0}
	}
	
	// Get the character under the cursor
	char := line[col]
	
	// Define matching pairs
	var matchingChar byte
	var direction int // 1 for forward search, -1 for backward search
	
	switch char {
	case '(':
		matchingChar = ')'
		direction = 1
	case ')':
		matchingChar = '('
		direction = -1
	case '[':
		matchingChar = ']'
		direction = 1
	case ']':
		matchingChar = '['
		direction = -1
	case '{':
		matchingChar = '}'
		direction = 1
	case '}':
		matchingChar = '{'
		direction = -1
	default:
		return [2]int{0, 0} // Not a bracket character
	}
	
	// Search for the matching bracket
	// For simplicity, only search in the current line for now
	count := 1 // Start with 1 for the bracket under the cursor
	
	if direction == 1 {
		// Search forward
		for i := col + 1; i < len(line); i++ {
			if line[i] == char {
				count++
			} else if line[i] == matchingChar {
				count--
				if count == 0 {
					return [2]int{row, i}
				}
			}
		}
	} else {
		// Search backward
		for i := col - 1; i >= 0; i-- {
			if line[i] == char {
				count++
			} else if line[i] == matchingChar {
				count--
				if count == 0 {
					return [2]int{row, i}
				}
			}
		}
	}
	
	// No match found
	return [2]int{0, 0}
}

// GetSearchState returns the current search state
// Returns:
// - searching: whether we're currently in search mode
// - pattern: the current search pattern
// - direction: 1 for forward, -1 for backward
// - currentIdx: index of current search result in the results list
func (e *GoEngine) GetSearchState() (bool, string, int, int) {
	return e.searching, e.searchPattern, e.searchDirection, e.currentSearchIdx
}

// GetSearchResults returns all matches for the current search
func (e *GoEngine) GetSearchResults() [][2]int {
	if e.searchResults == nil {
		return make([][2]int, 0)
	}
	return e.searchResults
}

// UndoSaveCursor saves just the cursor position for undo
// Returns true if successful, false otherwise
func (e *GoEngine) UndoSaveCursor() bool {
	if e.currentBuffer == nil {
		return false
	}
	
	// Check if there's already an undo record that can be appended to
	// For cursor-only changes, we don't always need a new record
	if len(e.currentBuffer.undoStack) > 0 {
		lastRecord := e.currentBuffer.undoStack[len(e.currentBuffer.undoStack)-1]
		// If there are no line changes yet, just update the cursor position
		if len(lastRecord.Changes) == 0 {
			lastRecord.CursorPos = [2]int{e.cursorRow, e.cursorCol}
			return true
		}
	}
	
	// Create a new undo record with just the cursor position
	record := &UndoRecord{
		Changes:     make(map[int]string),
		CursorPos:   [2]int{e.cursorRow, e.cursorCol},
		Description: "cursor movement",
	}
	
	// Add to undo stack
	e.currentBuffer.undoStack = append(e.currentBuffer.undoStack, record)
	
	// Clear redo stack when making a new change
	if len(e.currentBuffer.redoStack) > 0 {
		e.currentBuffer.redoStack = nil
	}
	
	return true
}

// UndoSaveRegion saves the current state of lines for later undo
// Returns true if successful, false otherwise
func (e *GoEngine) UndoSaveRegion(startLine, endLine int) bool {
	if e.currentBuffer == nil {
		return false
	}
	
	// Create a new undo record
	record := &UndoRecord{
		Changes:     make(map[int]string),
		CursorPos:   [2]int{e.cursorRow, e.cursorCol},
		Description: "text change",
	}
	
	// Save the specified lines (1-based indexing)
	for lineNum := startLine; lineNum <= endLine; lineNum++ {
		if lineNum >= 1 && lineNum <= e.currentBuffer.GetLineCount() {
			record.Changes[lineNum] = e.currentBuffer.GetLine(lineNum)
		}
	}
	
	// Add to undo stack
	e.currentBuffer.undoStack = append(e.currentBuffer.undoStack, record)
	
	// Clear redo stack when making a new change
	if len(e.currentBuffer.redoStack) > 0 {
		e.currentBuffer.redoStack = nil
	}
	
	return true
}

// UndoSync creates a synchronization point for undo operations
// This ensures the next change will be a separate undo operation
// The force parameter determines whether to always create a new sync point
func (e *GoEngine) UndoSync(force bool) {
	// Currently a no-op in our implementation since each UndoSaveRegion
	// already creates a distinct undo record. We could extend this later
	// to consolidate sequential changes if desired.
}

// Undo performs an undo operation, restoring the buffer and cursor to a previous state
// Returns true if the undo was successful, false otherwise
func (e *GoEngine) Undo() bool {
	if e.currentBuffer == nil || len(e.currentBuffer.undoStack) == 0 {
		// Nothing to undo
		return false
	}
	
	// Get the last undo record
	lastIdx := len(e.currentBuffer.undoStack) - 1
	record := e.currentBuffer.undoStack[lastIdx]
	
	// Create a redo record with current state of the changed lines
	redoRecord := &UndoRecord{
		Changes:       make(map[int]string),
		CursorPos:     [2]int{e.cursorRow, e.cursorCol},
		Description:   "redo " + record.Description,
		CommandType:   record.CommandType,
		LineOperation: record.LineOperation,
	}
	
	// Special handling for 'o' and 'O' commands which add a line
	// For these commands, we need to handle line removal properly
	if record.CommandType == "o" || record.CommandType == "O" {
		// For 'o' command, we need to save the current state then remove the added line
		lineCount := e.currentBuffer.GetLineCount()
		
		// Store the current state for redo
		for i := 1; i <= lineCount; i++ {
			redoRecord.Changes[i] = e.currentBuffer.GetLine(i)
		}
		
		// Temporarily disable undo recording
		oldUndoGroupState := e.inInsertUndoGroup
		e.inInsertUndoGroup = true
		
		// For 'o' command, we need to remove the line that was added
		// and restore the other lines to their previous state
		if record.CommandType == "o" {
			// The line after the cursor was added, so remove it
			newRow := record.CursorPos[0] + 1
			if newRow <= lineCount {
				// Remove the added line by restoring the buffer to its previous state
				// We create a new array excluding the line that was added
				var newLines []string
				for i := 1; i <= lineCount; i++ {
					if i != newRow {
						// Keep all lines except the added one
						if i <= len(record.Changes) {
							// Use the original content for lines that existed before
							newLines = append(newLines, record.Changes[i])
						} else {
							// This shouldn't happen if the undo record is properly created
							newLines = append(newLines, e.currentBuffer.GetLine(i))
						}
					}
				}
				
				// Replace the entire buffer with these lines
				if len(newLines) > 0 {
					e.currentBuffer.SetLines(0, -1, newLines)
				} else {
					// Ensure we have at least one line
					e.currentBuffer.SetLines(0, -1, []string{""})
				}
			}
		} else if record.CommandType == "O" {
			// The line at the cursor was added, so remove it
			newRow := record.CursorPos[0]
			if newRow <= lineCount {
				// Remove the added line by restoring the buffer to its previous state
				var newLines []string
				for i := 1; i <= lineCount; i++ {
					if i != newRow {
						// Keep all lines except the added one
						if i <= len(record.Changes) {
							newLines = append(newLines, record.Changes[i])
						} else {
							newLines = append(newLines, e.currentBuffer.GetLine(i))
						}
					}
				}
				
				// Replace the entire buffer with these lines
				if len(newLines) > 0 {
					e.currentBuffer.SetLines(0, -1, newLines)
				} else {
					// Ensure we have at least one line
					e.currentBuffer.SetLines(0, -1, []string{""})
				}
			}
		}
		
		// Restore undo state
		e.inInsertUndoGroup = oldUndoGroupState
	} else {
		// Standard undo behavior for other commands
		
		// Save current state for redo
		for lineNum := range record.Changes {
			if lineNum <= e.currentBuffer.GetLineCount() {
				redoRecord.Changes[lineNum] = e.currentBuffer.GetLine(lineNum)
			}
		}
		
		// Apply the undo changes
		// Temporarily disable undo recording to avoid circular undo events
		oldUndoGroupState := e.inInsertUndoGroup
		e.inInsertUndoGroup = true  // This prevents SetLines from creating new undo records
		
		// For each line in the undo record, restore it to its previous state
		for lineNum, content := range record.Changes {
			if lineNum >= 1 && lineNum <= e.currentBuffer.GetLineCount() {
				e.currentBuffer.SetLines(lineNum-1, lineNum, []string{content})
			}
		}
		
		// Restore previous undo state
		e.inInsertUndoGroup = oldUndoGroupState
	}
	
	// Make sure to update the buffer's last tick
	e.currentBuffer.lastTick++
	
	// Only mark the buffer as modified if we're not back at the last saved state
	if lastIdx == e.currentBuffer.lastSavedState {
		e.currentBuffer.modified = false
	} else {
		e.currentBuffer.modified = true
	}
	
	// Always restore cursor position based on undo record
	e.cursorRow = record.CursorPos[0]
	e.cursorCol = record.CursorPos[1]
	
	// Ensure cursor position is valid for the current buffer state
	e.validateCursorPosition()
	
	// Move record from undo to redo stack
	e.currentBuffer.undoStack = e.currentBuffer.undoStack[:lastIdx]
	e.currentBuffer.redoStack = append(e.currentBuffer.redoStack, redoRecord)
	
	return true
}

// Redo performs a redo operation, reapplying a previously undone change
// Returns true if the redo was successful, false otherwise
func (e *GoEngine) Redo() bool {
	if e.currentBuffer == nil || len(e.currentBuffer.redoStack) == 0 {
		// Nothing to redo
		return false
	}
	
	// Get the last redo record
	lastIdx := len(e.currentBuffer.redoStack) - 1
	record := e.currentBuffer.redoStack[lastIdx]
	
	// Create an undo record with current state of the changed lines
	undoRecord := &UndoRecord{
		Changes:       make(map[int]string),
		CursorPos:     [2]int{e.cursorRow, e.cursorCol},
		Description:   "undo " + record.Description,
		CommandType:   record.CommandType,
		LineOperation: record.LineOperation,
	}
	
	// Special handling for 'o' and 'O' commands - they need special treatment during redo
	if record.CommandType == "o" || record.CommandType == "O" {
		// For 'o' or 'O' commands, we need to save the current state
		lineCount := e.currentBuffer.GetLineCount()
		
		// Store the current state for undo
		for i := 1; i <= lineCount; i++ {
			undoRecord.Changes[i] = e.currentBuffer.GetLine(i)
		}
		
		// Temporarily disable undo recording
		oldUndoGroupState := e.inInsertUndoGroup
		e.inInsertUndoGroup = true
		
		// For redoing 'o' or 'O', we need to restore the buffer state from the redo record
		// This includes the line that was added
		
		// Create a completely new buffer with all lines from the redo record
		var newLines []string
		for i := 1; i <= len(record.Changes); i++ {
			if content, exists := record.Changes[i]; exists {
				newLines = append(newLines, content)
			}
		}
		
		// Replace the entire buffer with these lines
		if len(newLines) > 0 {
			e.currentBuffer.SetLines(0, -1, newLines)
		} else {
			// Ensure we have at least one line
			e.currentBuffer.SetLines(0, -1, []string{""})
		}
		
		// Restore undo state
		e.inInsertUndoGroup = oldUndoGroupState
	} else {
		// Standard redo behavior for other commands
		
		// Save current state for undo
		for lineNum := range record.Changes {
			if lineNum <= e.currentBuffer.GetLineCount() {
				undoRecord.Changes[lineNum] = e.currentBuffer.GetLine(lineNum)
			}
		}
		
		// Apply the redo changes
		// Temporarily disable undo recording to avoid circular undo events
		oldUndoGroupState := e.inInsertUndoGroup
		e.inInsertUndoGroup = true  // This prevents SetLines from creating new undo records
		
		// For each line in the redo record, restore it to its state before undo
		for lineNum, content := range record.Changes {
			if lineNum >= 1 && lineNum <= e.currentBuffer.GetLineCount() {
				e.currentBuffer.SetLines(lineNum-1, lineNum, []string{content})
			}
		}
		
		// Restore previous undo state
		e.inInsertUndoGroup = oldUndoGroupState
	}
	
	// Make sure to update the buffer's last tick
	e.currentBuffer.lastTick++
	
	// Check if we're back at the saved state
	if len(e.currentBuffer.undoStack) == e.currentBuffer.lastSavedState {
		e.currentBuffer.modified = false
	} else {
		e.currentBuffer.modified = true
	}
	
	// Restore cursor position based on redo record
	e.cursorRow = record.CursorPos[0]
	e.cursorCol = record.CursorPos[1]
	
	// Ensure cursor position is valid for the current buffer state
	e.validateCursorPosition()
	
	// Move record from redo to undo stack
	e.currentBuffer.redoStack = e.currentBuffer.redoStack[:lastIdx]
	e.currentBuffer.undoStack = append(e.currentBuffer.undoStack, undoRecord)
	
	return true
}

// validateCursorPosition ensures the cursor is in a valid position
// after operations like undo/redo that might change buffer contents
func (e *GoEngine) validateCursorPosition() {
	if e.currentBuffer == nil {
		return
	}
	
	// Check row bounds
	lineCount := e.currentBuffer.GetLineCount()
	if e.cursorRow < 1 {
		e.cursorRow = 1
	} else if e.cursorRow > lineCount {
		e.cursorRow = lineCount
	}
	
	// Check column bounds
	currentLine := e.currentBuffer.GetLine(e.cursorRow)
	if e.cursorCol < 0 {
		e.cursorCol = 0
	} else if e.cursorCol > len(currentLine) {
		e.cursorCol = len(currentLine)
	}
}

// startInsertUndoGroup begins a new insert mode undo group
// This makes vim treat all changes in insert mode as a single undo operation
// commandType can be "o", "O", or "" (general insert mode)
func (e *GoEngine) startInsertUndoGroup(commandType string) {
	if e.currentBuffer == nil {
		return
	}
	
	// Mark that we're in an insert undo group
	e.inInsertUndoGroup = true
	
	// Create a comprehensive undo record containing the entire buffer state
	record := &UndoRecord{
		Changes:       make(map[int]string),
		CursorPos:     [2]int{e.cursorRow, e.cursorCol},
		Description:   "insert mode",
		CommandType:   commandType,
		LineOperation: commandType == "o" || commandType == "O", // These commands operate on whole lines
	}
	
	// For 'o' and 'O' commands, we need to save the state BEFORE the line is added
	// This is important to ensure that undo will properly restore the state
	if commandType == "o" || commandType == "O" {
		// Make a clean copy of the buffer state
		// It's important to get a snapshot of the buffer BEFORE the additional line is added
		// For 'o' we need to save the cursor row and the cursor row - 1
		// For 'O' we need to save the cursor row + 1 and the cursor row
		// But for simplicity, just save the entire buffer
		for i := 1; i <= e.currentBuffer.GetLineCount(); i++ {
			record.Changes[i] = e.currentBuffer.GetLine(i)
		}
	} else {
		// For normal insert operations, save the current buffer state
		for i := 1; i <= e.currentBuffer.GetLineCount(); i++ {
			record.Changes[i] = e.currentBuffer.GetLine(i)
		}
	}
	
	// Add to undo stack
	e.currentBuffer.undoStack = append(e.currentBuffer.undoStack, record)
	
	// Clear redo stack when making a new change
	if len(e.currentBuffer.redoStack) > 0 {
		e.currentBuffer.redoStack = nil
	}
}