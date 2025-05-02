package govim

import (
	"strings"
)

// ModeNormal is the normal mode constant
const ModeNormal = 1

// ModeVisual is the visual mode constant
const ModeVisual = 2

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
	yankRegister    string // Content of the "unnamed" register for yank/put
	
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
		yankRegister:    "",
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
		id:     e.nextBufferId,
		engine: e,
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
		id:     e.nextBufferId,
		engine: e,
		lines:  []string{""},
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
		e.currentBuffer = goBuf
	}
}

// CursorGetLine returns the current cursor line
func (e *GoEngine) CursorGetLine() int {
	return e.cursorRow
}

// CursorGetPosition returns the cursor position as [row, col]
func (e *GoEngine) CursorGetPosition() [2]int {
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

// Input processes input (basic motion commands)
func (e *GoEngine) Input(s string) {
	// Handle motion with count (e.g., "5j", "3w")
	// First, check if we're in command count building mode
	if e.mode == ModeNormal && s >= "0" && s <= "9" && !e.awaitingMotion {
		// Convert the digit to its numeric value
		digit := int(s[0] - '0')
		
		// If the count is 0, it's a special case as a motion
		if digit == 0 && e.commandCount == 0 {
			if handler, ok := motionHandlers["0"]; ok {
				handler(e)
			}
			return
		}
		
		// Otherwise, build the count
		e.commandCount = e.commandCount*10 + digit
		return
	}
	
	// Apply counts to motions when applicable
	if e.mode == ModeNormal && e.commandCount > 0 && !e.awaitingMotion {
		if handler, ok := motionHandlers[s]; ok {
			// Apply the motion the specified number of times
			for i := 0; i < e.commandCount; i++ {
				handler(e)
			}
			e.commandCount = 0
			return
		}
	}
	
	// Handle command + motion operations (like d, c, y)
	if e.awaitingMotion {
		if s == e.currentCommand {
			// Double command (dd, cc, yy) operates on the entire line
			e.handleLineCommand(e.currentCommand)
			e.resetCommandState()
			return
		}
		
		// Handle motion after command (e.g., "dw", "c$")
		if handler, ok := motionHandlers[s]; ok {
			// Save current position as start of operation
			startPos := [2]int{e.cursorRow, e.cursorCol}
			
			// Apply the motion
			handler(e)
			
			// End position after motion
			endPos := [2]int{e.cursorRow, e.cursorCol}
			
			// Execute the command on the range
			e.executeCommandOnRange(e.currentCommand, startPos, endPos)
			
			// Reset command state
			e.resetCommandState()
			return
		}
		
		// If we get here, it was an invalid motion after a command
		e.resetCommandState()
		return
	}
	
	if e.mode == ModeNormal {
		if handler, ok := motionHandlers[s]; ok {
			handler(e)
			return
		}
		
		// Handle other normal mode commands
		switch s {
		case "i":
			e.mode = ModeInsert
		case "v":
			e.mode = ModeVisual
			e.visualStart = [2]int{e.cursorRow, e.cursorCol}
			e.visualEnd = [2]int{e.cursorRow, e.cursorCol}
			e.visualType = 'v' // character-wise visual mode
		case "gg":
			// Move to first line
			e.cursorRow = 1
			// Move to first non-blank character
			if e.currentBuffer != nil {
				moveToFirstNonBlank(e)
			}
		case "d":
			// Start delete operation, awaiting motion
			e.awaitingMotion = true
			e.currentCommand = "d"
			return
		case "c":
			// Start change operation, awaiting motion
			e.awaitingMotion = true
			e.currentCommand = "c"
			return
		case "y":
			// Start yank operation, awaiting motion
			e.awaitingMotion = true
			e.currentCommand = "y"
			return
		case "p":
			// Put text after cursor
			e.putAfter()
			return
		case "P":
			// Put text before cursor
			e.putBefore()
			return
		case "/":
			// Start forward search
			e.startSearch(1)
			return
		case "?":
			// Start backward search
			e.startSearch(-1)
			return
		case "n":
			// Repeat search in same direction
			e.findNextMatch(e.searchDirection)
			return
		case "N":
			// Repeat search in opposite direction
			e.findNextMatch(-e.searchDirection)
			return
		case "x":
			// Delete character under cursor
			if e.currentBuffer != nil {
				line := e.currentBuffer.GetLine(e.cursorRow)
				if e.cursorCol < len(line) {
					// Store deleted character in register for potential paste operations
					e.yankRegister = string(line[e.cursorCol])
					
					// Create new line with the character removed
					newLine := line[:e.cursorCol] + line[e.cursorCol+1:]
					
					// Update the line in the buffer
					e.currentBuffer.SetLines(e.cursorRow-1, e.cursorRow, []string{newLine})
					
					// If we're at the end of the line now, move cursor back if possible
					if e.cursorCol >= len(newLine) && e.cursorCol > 0 {
						e.cursorCol--
					}
				}
			}
			return
		}
	} else if e.mode == ModeInsert {
		// Handle insert mode
		if s == "\x1b" { // ESC key
			e.mode = ModeNormal
			return
		}
		
		// Insert the character at current position
		if e.currentBuffer != nil {
			line := e.currentBuffer.GetLine(e.cursorRow)
			newLine := ""
			if e.cursorCol == 0 {
				newLine = s + line
			} else if e.cursorCol >= len(line) {
				newLine = line + s
			} else {
				newLine = line[:e.cursorCol] + s + line[e.cursorCol:]
			}
			
			// Update the line
			e.currentBuffer.lines[e.cursorRow-1] = newLine
			e.currentBuffer.markModified()
			
			// Move cursor forward
			e.cursorCol += len(s)
		}
	} else if e.mode == ModeVisual {
		// Apply counts to motions in visual mode too
		if e.commandCount > 0 && s >= "0" && s <= "9" {
			// Build the count
			digit := int(s[0] - '0')
			e.commandCount = e.commandCount*10 + digit
			return
		}
		
		// Apply count to motion
		if e.commandCount > 0 {
			if handler, ok := motionHandlers[s]; ok {
				// Apply the motion the specified number of times
				for i := 0; i < e.commandCount; i++ {
					handler(e)
				}
				e.visualEnd = [2]int{e.cursorRow, e.cursorCol}
				e.commandCount = 0
				return
			}
		}
		
		if handler, ok := motionHandlers[s]; ok {
			handler(e)
			e.visualEnd = [2]int{e.cursorRow, e.cursorCol}
			return
		}
		
		// Handle visual mode commands
		switch s {
		case "\x1b": // ESC key
			e.mode = ModeNormal
		}
	} else if e.mode == ModeSearch {
		// Handle search mode
		if s == "\x1b" { // ESC key
			// Cancel search
			e.mode = ModeNormal
			e.searching = false
			e.searchBuffer = ""
		} else if s == "\r" { // Enter key
			// Complete search
			if e.searchBuffer != "" {
				// Set the search pattern
				e.searchPattern = e.searchBuffer
				
				// Execute the search
				e.executeSearch()
				
				// Return to normal mode
				e.mode = ModeNormal
				e.searching = false
			} else {
				// Empty search - repeat last search if there was one
				if e.searchPattern != "" {
					e.executeSearch()
				}
				e.mode = ModeNormal
				e.searching = false
			}
		} else if s == "\b" { // Backspace key
			// Remove last character from search buffer
			if len(e.searchBuffer) > 0 {
				e.searchBuffer = e.searchBuffer[:len(e.searchBuffer)-1]
			}
		} else {
			// Add character to search buffer
			e.searchBuffer += s
		}
	}
}

// Key processes a key with terminal codes replaced
func (e *GoEngine) Key(s string) {
	// For now, map some common key codes
	switch s {
	case "<ESC>":
		e.Input("\x1b")
	case "<LEFT>":
		moveLeft(e)
	case "<RIGHT>":
		moveRight(e)
	case "<UP>":
		moveUp(e)
	case "<DOWN>":
		moveDown(e)
	case "<CR>", "<ENTER>":
		e.Input("\r")
	case "<BS>", "<BACKSPACE>":
		e.Input("\b")
	default:
		// Try to handle literally
		e.Input(s)
	}
}

// Execute runs a vim ex command
func (e *GoEngine) Execute(cmd string) {
	// Parse and execute ex commands
	cmd = strings.TrimSpace(cmd)
	
	// Handle some basic ex commands
	if cmd == "w" || strings.HasPrefix(cmd, "w ") {
		// Mark buffer as saved (not modified)
		if e.currentBuffer != nil {
			e.currentBuffer.modified = false
		}
	} else if cmd == "q" || cmd == "q!" {
		// Quit would be handled by the main application
	} else if cmd == "set number" {
		// Would set line numbering
	} else if cmd == "set nonumber" {
		// Would unset line numbering
	}
}

// GetMode returns the current mode
func (e *GoEngine) GetMode() int {
	return e.mode
}

// VisualGetRange returns the visual selection range
func (e *GoEngine) VisualGetRange() [2][2]int {
	return [2][2]int{
		{e.visualStart[0], e.visualStart[1]},
		{e.visualEnd[0], e.visualEnd[1]},
	}
}

// VisualGetType returns the visual mode type
func (e *GoEngine) VisualGetType() int {
	return e.visualType
}

// GetSearchState returns the current search state information
func (e *GoEngine) GetSearchState() (bool, string, int, int) {
	// Returns: isSearching, searchBuffer, matchCount, currentMatchIndex
	return e.searching, e.searchBuffer, len(e.searchResults), e.currentSearchIdx
}

// GetSearchResults returns all matches for the current search
func (e *GoEngine) GetSearchResults() [][2]int {
	return e.searchResults
}

// Eval evaluates a vim expression
func (e *GoEngine) Eval(expr string) string {
	// Placeholder for vim expression evaluation
	return ""
}

// resetCommandState resets the command state for operations like d, c, y
func (e *GoEngine) resetCommandState() {
	e.awaitingMotion = false
	e.currentCommand = ""
}

// handleLineCommand handles line operations like dd, cc, yy
func (e *GoEngine) handleLineCommand(cmd string) {
	if e.currentBuffer == nil {
		return
	}
	
	switch cmd {
	case "d":
		// Delete current line
		e.deleteRange(
			[2]int{e.cursorRow, 0},
			[2]int{e.cursorRow, len(e.currentBuffer.GetLine(e.cursorRow))},
			true,
		)
	case "c":
		// Change current line (delete and enter insert mode)
		e.deleteRange(
			[2]int{e.cursorRow, 0},
			[2]int{e.cursorRow, len(e.currentBuffer.GetLine(e.cursorRow))},
			true,
		)
		e.mode = ModeInsert
	case "y":
		// Yank current line
		e.yankRange(
			[2]int{e.cursorRow, 0},
			[2]int{e.cursorRow, len(e.currentBuffer.GetLine(e.cursorRow))},
			true,
		)
	}
}

// executeCommandOnRange applies a command to a range of text
func (e *GoEngine) executeCommandOnRange(cmd string, start, end [2]int) {
	// Ensure start comes before end
	if start[0] > end[0] || (start[0] == end[0] && start[1] > end[1]) {
		start, end = end, start
	}
	
	switch cmd {
	case "d":
		e.deleteRange(start, end, false)
	case "c":
		e.deleteRange(start, end, false)
		e.mode = ModeInsert
	case "y":
		e.yankRange(start, end, false)
	}
}

// deleteRange deletes text in the given range
func (e *GoEngine) deleteRange(start, end [2]int, wholeLine bool) {
	if e.currentBuffer == nil {
		return
	}
	
	// First yank the text being deleted
	e.yankRange(start, end, wholeLine)
	
	if wholeLine {
		// Delete the entire line
		lines := e.currentBuffer.Lines()
		lineIndex := start[0] - 1 // Convert to 0-based
		
		// Remove the line
		if lineIndex < len(lines) {
			e.currentBuffer.lines = append(lines[:lineIndex], lines[lineIndex+1:]...)
			e.currentBuffer.markModified()
			
			// If we deleted the last line, move cursor up
			if lineIndex >= len(e.currentBuffer.lines) {
				e.cursorRow = len(e.currentBuffer.lines)
				if e.cursorRow == 0 {
					// Buffer is now empty, add an empty line
					e.currentBuffer.lines = []string{""}
					e.cursorRow = 1
				}
			}
			
			// Position cursor at first non-blank character of the line
			e.cursorCol = 0
			if e.cursorRow <= len(e.currentBuffer.lines) {
				moveToFirstNonBlank(e)
			}
		}
	} else if start[0] == end[0] {
		// Delete within a single line
		lineIndex := start[0] - 1 // Convert to 0-based
		line := e.currentBuffer.GetLine(start[0])
		
		// Create new line with deleted portion removed
		newLine := line[:start[1]] + line[end[1]:]
		e.currentBuffer.lines[lineIndex] = newLine
		e.currentBuffer.markModified()
		
		// Position cursor at the deletion point
		e.cursorCol = start[1]
	} else {
		// Delete across multiple lines
		startLine := start[0] - 1 // Convert to 0-based
		endLine := end[0] - 1
		
		// Get the parts we want to keep
		startLineContent := e.currentBuffer.GetLine(start[0])[:start[1]]
		endLineContent := e.currentBuffer.GetLine(end[0])[end[1]:]
		
		// Create the merged line
		newLine := startLineContent + endLineContent
		
		// Keep the merged line and remove the others
		e.currentBuffer.lines[startLine] = newLine
		e.currentBuffer.lines = append(
			e.currentBuffer.lines[:startLine+1],
			e.currentBuffer.lines[endLine+1:]...,
		)
		
		e.currentBuffer.markModified()
		
		// Position cursor at the deletion start
		e.cursorRow = start[0]
		e.cursorCol = start[1]
	}
}

// yankRange copies text in the given range to the yank register
func (e *GoEngine) yankRange(start, end [2]int, wholeLine bool) {
	if e.currentBuffer == nil {
		return
	}
	
	var yankedText string
	
	if wholeLine {
		// Yank the entire line
		line := e.currentBuffer.GetLine(start[0])
		yankedText = line + "\n"
	} else if start[0] == end[0] {
		// Yank within a single line
		line := e.currentBuffer.GetLine(start[0])
		if end[1] <= len(line) && start[1] <= end[1] {
			yankedText = line[start[1]:end[1]]
		}
	} else {
		// Yank across multiple lines
		var sb strings.Builder
		
		// First line
		firstLine := e.currentBuffer.GetLine(start[0])
		if start[1] < len(firstLine) {
			sb.WriteString(firstLine[start[1]:])
		}
		sb.WriteString("\n")
		
		// Middle lines (if any)
		for i := start[0] + 1; i < end[0]; i++ {
			sb.WriteString(e.currentBuffer.GetLine(i))
			sb.WriteString("\n")
		}
		
		// Last line
		lastLine := e.currentBuffer.GetLine(end[0])
		if end[1] <= len(lastLine) {
			sb.WriteString(lastLine[:end[1]])
		}
		
		yankedText = sb.String()
	}
	
	// Store in yank register
	e.yankRegister = yankedText
}

// putAfter puts yanked text after the cursor
func (e *GoEngine) putAfter() {
	if e.currentBuffer == nil || e.yankRegister == "" {
		return
	}
	
	// Check if the register contains a whole line (ending with newline)
	if e.yankRegister[len(e.yankRegister)-1] == '\n' {
		// Line-wise put: add the line after the current line
		lineIndex := e.cursorRow // 1-based
		lines := strings.Split(e.yankRegister, "\n")
		
		// Remove the last empty entry from the split if it exists
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		
		// Insert the lines after the current line
		newLines := make([]string, 0, len(e.currentBuffer.lines)+len(lines))
		newLines = append(newLines, e.currentBuffer.lines[:lineIndex]...)
		newLines = append(newLines, lines...)
		newLines = append(newLines, e.currentBuffer.lines[lineIndex:]...)
		
		e.currentBuffer.lines = newLines
		e.currentBuffer.markModified()
		
		// Position cursor at the first non-blank character of the first inserted line
		e.cursorRow = lineIndex + 1
		e.cursorCol = 0
		moveToFirstNonBlank(e)
	} else {
		// Character-wise put: put after the cursor
		lineIndex := e.cursorRow - 1 // Convert to 0-based
		line := e.currentBuffer.GetLine(e.cursorRow)
		col := e.cursorCol
		
		// Insert the yanked text after the current position
		if col >= len(line) {
			// If cursor is at the end of the line, append
			e.currentBuffer.lines[lineIndex] += e.yankRegister
		} else {
			// Otherwise insert
			e.currentBuffer.lines[lineIndex] = line[:col+1] + e.yankRegister + line[col+1:]
		}
		
		e.currentBuffer.markModified()
		
		// Position cursor at the end of the inserted text
		e.cursorCol = col + len(e.yankRegister)
	}
}

// putBefore puts yanked text before the cursor
func (e *GoEngine) putBefore() {
	if e.currentBuffer == nil || e.yankRegister == "" {
		return
	}
	
	// Check if the register contains a whole line (ending with newline)
	if e.yankRegister[len(e.yankRegister)-1] == '\n' {
		// Line-wise put: add the line before the current line
		lineIndex := e.cursorRow - 1 // 0-based
		lines := strings.Split(e.yankRegister, "\n")
		
		// Remove the last empty entry from the split if it exists
		if len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		
		// Insert the lines before the current line
		newLines := make([]string, 0, len(e.currentBuffer.lines)+len(lines))
		newLines = append(newLines, e.currentBuffer.lines[:lineIndex]...)
		newLines = append(newLines, lines...)
		newLines = append(newLines, e.currentBuffer.lines[lineIndex:]...)
		
		e.currentBuffer.lines = newLines
		e.currentBuffer.markModified()
		
		// Position cursor at the first non-blank character of the first inserted line
		e.cursorRow = lineIndex + 1
		e.cursorCol = 0
		moveToFirstNonBlank(e)
	} else {
		// Character-wise put: put before the cursor
		lineIndex := e.cursorRow - 1 // Convert to 0-based
		line := e.currentBuffer.GetLine(e.cursorRow)
		col := e.cursorCol
		
		// Insert the yanked text before the current position
		e.currentBuffer.lines[lineIndex] = line[:col] + e.yankRegister + line[col:]
		e.currentBuffer.markModified()
		
		// Position cursor at the end of the inserted text
		e.cursorCol = col + len(e.yankRegister) - 1
		if e.cursorCol < 0 {
			e.cursorCol = 0
		}
	}
}

// startSearch begins a search operation
func (e *GoEngine) startSearch(direction int) {
	e.mode = ModeSearch
	e.searching = true
	e.searchDirection = direction
	e.searchBuffer = ""
}

// executeSearch performs the search with the current pattern
func (e *GoEngine) executeSearch() {
	if e.currentBuffer == nil || e.searchPattern == "" {
		return
	}
	
	// Find all matches in the buffer
	e.searchResults = e.findAllMatches(e.searchPattern)
	
	// If no matches found, return
	if len(e.searchResults) == 0 {
		e.currentSearchIdx = -1
		return
	}
	
	// For backward search, initialize to the last match position
	if e.searchDirection < 0 && len(e.searchResults) > 0 {
		// Start from the end for backward search
		currentPos := [2]int{e.cursorRow, e.cursorCol}
		
		// Find the last match before current position
		foundIdx := -1
		for i := len(e.searchResults) - 1; i >= 0; i-- {
			match := e.searchResults[i]
			if match[0] < currentPos[0] || (match[0] == currentPos[0] && match[1] < currentPos[1]) {
				foundIdx = i
				break
			}
		}
		
		// If no match found before current position, wrap around to the last match
		if foundIdx == -1 {
			foundIdx = len(e.searchResults) - 1
		}
		
		// Set the cursor to this match
		e.currentSearchIdx = foundIdx
		match := e.searchResults[e.currentSearchIdx]
		e.cursorRow = match[0]
		e.cursorCol = match[1]
	} else {
		// Forward search - find first match after cursor
		e.findNextMatch(1)
	}
}

// findAllMatches finds all occurrences of the pattern in the buffer
func (e *GoEngine) findAllMatches(pattern string) [][2]int {
	if e.currentBuffer == nil {
		return nil
	}
	
	var matches [][2]int
	lineCount := e.currentBuffer.GetLineCount()
	
	// Search each line for the pattern
	for i := 1; i <= lineCount; i++ {
		line := e.currentBuffer.GetLine(i)
		idx := 0
		for {
			// Find the pattern in the current line, starting from idx
			pos := strings.Index(line[idx:], pattern)
			if pos == -1 {
				break
			}
			
			// Record the match position (absolute position in line)
			absPos := idx + pos
			matches = append(matches, [2]int{i, absPos})
			
			// Move past this match for the next search
			idx = absPos + len(pattern)
			if idx >= len(line) {
				break
			}
		}
	}
	
	return matches
}

// findNextMatch moves to the next match in the specified direction
func (e *GoEngine) findNextMatch(direction int) {
	if e.currentBuffer == nil || len(e.searchResults) == 0 {
		return
	}
	
	currentPos := [2]int{e.cursorRow, e.cursorCol}
	
	// If we have a previous search index, use it as a starting point
	if e.currentSearchIdx >= 0 && e.currentSearchIdx < len(e.searchResults) {
		if direction > 0 {
			// Forward search
			e.currentSearchIdx = (e.currentSearchIdx + 1) % len(e.searchResults)
		} else {
			// Backward search
			e.currentSearchIdx--
			if e.currentSearchIdx < 0 {
				e.currentSearchIdx = len(e.searchResults) - 1
			}
		}
	} else {
		// No previous search index, find the nearest match in the given direction
		if direction > 0 {
			// Find the first match after the current position
			e.currentSearchIdx = -1
			for i, match := range e.searchResults {
				if match[0] > currentPos[0] || (match[0] == currentPos[0] && match[1] > currentPos[1]) {
					e.currentSearchIdx = i
					break
				}
			}
			
			// If no match found after current position, wrap around
			if e.currentSearchIdx == -1 {
				e.currentSearchIdx = 0
			}
		} else {
			// Find the last match before the current position
			e.currentSearchIdx = -1
			for i := len(e.searchResults) - 1; i >= 0; i-- {
				match := e.searchResults[i]
				if match[0] < currentPos[0] || (match[0] == currentPos[0] && match[1] < currentPos[1]) {
					e.currentSearchIdx = i
					break
				}
			}
			
			// If no match found before current position, wrap around
			if e.currentSearchIdx == -1 {
				e.currentSearchIdx = len(e.searchResults) - 1
			}
		}
	}
	
	// Move cursor to the match
	match := e.searchResults[e.currentSearchIdx]
	e.cursorRow = match[0]
	e.cursorCol = match[1]
}

// SearchGetMatchingPair finds matching brackets
func (e *GoEngine) SearchGetMatchingPair() [2]int {
	if e.currentBuffer == nil {
		return [2]int{0, 0}
	}
	
	// Simple implementation for bracket matching
	line := e.currentBuffer.GetLine(e.cursorRow)
	if e.cursorCol >= len(line) {
		return [2]int{0, 0}
	}
	
	// Get the character under the cursor
	char := line[e.cursorCol]
	var match byte
	var direction int
	
	// Determine the matching character and search direction
	switch char {
	case '(':
		match = ')'
		direction = 1
	case '[':
		match = ']'
		direction = 1
	case '{':
		match = '}'
		direction = 1
	case ')':
		match = '('
		direction = -1
	case ']':
		match = '['
		direction = -1
	case '}':
		match = '{'
		direction = -1
	default:
		return [2]int{0, 0}
	}
	
	// Implement bracket matching logic
	// This is a simplified version - a full implementation would handle nesting
	
	// For now, just search the current line
	if direction > 0 {
		// Search forward
		for i := e.cursorCol + 1; i < len(line); i++ {
			if line[i] == match {
				return [2]int{e.cursorRow, i}
			}
		}
	} else {
		// Search backward
		for i := e.cursorCol - 1; i >= 0; i-- {
			if line[i] == match {
				return [2]int{e.cursorRow, i}
			}
		}
	}
	
	return [2]int{0, 0}
}