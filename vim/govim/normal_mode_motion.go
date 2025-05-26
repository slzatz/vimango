package govim

// BasicMotions implements the basic h,j,k,l movement commands
type motionCommand func(e *GoEngine, count int) bool

// motionHandlers maps characters to their motion handlers
var motionHandlers = map[string]motionCommand{
	"h":     saveAndMoveLeft,
	"j":     saveAndMoveDown,
	"k":     saveAndMoveUp,
	"l":     saveAndMoveRight,
	"0":     saveAndMoveToLineStart,
	"$":     saveAndMoveToLineEnd,
	"^":     saveAndMoveToFirstNonBlank,
	"w":     saveAndMoveWordForward,
	"b":     saveAndMoveWordBackward,
	"e":     saveAndMoveWordEnd,
	"G":     saveAndMoveToLastLine,
	"g":     saveAndMoveToFirstLine, // Changed from "gg" to "g" - we'll handle the second 'g' in Input()
	"%":     saveAndMoveToMatchingBracket,
	"u":     performUndo,
	"<C-r>": performRedo,
	".":     repeatLastEdit,
}

// moveLeft moves the cursor one or more characters to the left
func moveLeft(e *GoEngine, count int) bool {
	if count <= 0 {
		count = 1 // Default to 1 if count is not specified
	}

	moved := false
	for i := 0; i < count; i++ {
		if e.currentBuffer.cursorCol > 0 {
			e.currentBuffer.cursorCol--
			moved = true
		} else if e.currentBuffer.cursorRow > 1 && e.mode == ModeInsert {
			// In insert mode, allow going to previous line at beginning of line
			e.currentBuffer.cursorRow--
			if e.currentBuffer != nil {
				line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
				e.currentBuffer.cursorCol = len(line)
			}
			moved = true
			break // Stop after jumping to previous line
		} else {
			break // Can't move further
		}
	}

	// Update visual selection if in visual mode
	if moved && e.mode == ModeVisual {
		e.visualEnd = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
	}

	return moved
}

// moveRight moves the cursor one or more characters to the right
func moveRight(e *GoEngine, count int) bool {
	if e.currentBuffer == nil {
		return false
	}

	if count <= 0 {
		count = 1 // Default to 1 if count is not specified
	}

	line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
	moved := false

	// Different behavior based on mode
	if e.mode == ModeInsert {
		// In insert mode we can go to the end of the line
		for i := 0; i < count && e.currentBuffer.cursorCol < len(line); i++ {
			e.currentBuffer.cursorCol++
			moved = true
		}
	} else {
		// In normal mode, we can only go to the last character of the line
		for i := 0; i < count && len(line) > 0 && e.currentBuffer.cursorCol < len(line)-1; i++ {
			e.currentBuffer.cursorCol++
			moved = true
		}
	}

	// Update visual selection if in visual mode
	if moved && e.mode == ModeVisual {
		e.visualEnd = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
	}

	return moved
}

// moveDown moves the cursor one or more lines down
func moveDown(e *GoEngine, count int) bool {
	if e.currentBuffer == nil {
		return false
	}

	if count <= 0 {
		count = 1 // Default to 1 if count is not specified
	}

	// Save the desired column position
	desiredCol := e.currentBuffer.cursorCol
	moved := false

	// Move down by the specified count
	for i := 0; i < count && e.currentBuffer.cursorRow < e.currentBuffer.GetLineCount(); i++ {
		e.currentBuffer.cursorRow++
		moved = true
	}

	if moved {
		// Adjust column if the new line is shorter
		line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

		if e.mode == ModeInsert {
			// In insert mode, we can position at the end of the line
			if desiredCol >= len(line) {
				e.currentBuffer.cursorCol = len(line)
			} else {
				e.currentBuffer.cursorCol = desiredCol
			}
		} else {
			// In normal mode, we can only position on an actual character
			if len(line) == 0 {
				e.currentBuffer.cursorCol = 0
			} else if desiredCol >= len(line) {
				e.currentBuffer.cursorCol = len(line) - 1
			} else {
				e.currentBuffer.cursorCol = desiredCol
			}
		}

		// Update visual selection if in visual mode
		if e.mode == ModeVisual {
			e.visualEnd = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
		}
	}

	return moved
}

// moveUp moves the cursor one or more lines up
func moveUp(e *GoEngine, count int) bool {
	if e.currentBuffer == nil {
		return false
	}

	if count <= 0 {
		count = 1 // Default to 1 if count is not specified
	}

	// Save the desired column position
	desiredCol := e.currentBuffer.cursorCol
	moved := false

	// Move up by the specified count
	for i := 0; i < count && e.currentBuffer.cursorRow > 1; i++ {
		e.currentBuffer.cursorRow--
		moved = true
	}

	if moved {
		// Adjust column if the new line is shorter
		line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

		if e.mode == ModeInsert {
			// In insert mode, we can position at the end of the line
			if desiredCol >= len(line) {
				e.currentBuffer.cursorCol = len(line)
			} else {
				e.currentBuffer.cursorCol = desiredCol
			}
		} else {
			// In normal mode, we can only position on an actual character
			if len(line) == 0 {
				e.currentBuffer.cursorCol = 0
			} else if desiredCol >= len(line) {
				e.currentBuffer.cursorCol = len(line) - 1
			} else {
				e.currentBuffer.cursorCol = desiredCol
			}
		}

		// Update visual selection if in visual mode
		if e.mode == ModeVisual {
			e.visualEnd = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
		}
	}

	return moved
}

// moveToLineStart moves to the first character of the line (count is ignored)
func moveToLineStart(e *GoEngine, count int) bool {
	if e.currentBuffer.cursorCol != 0 {
		e.currentBuffer.cursorCol = 0

		// Update visual selection if in visual mode
		if e.mode == ModeVisual {
			e.visualEnd = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
		}

		return true
	}
	return false
}

// moveToLineEnd moves to the end of the line (count is ignored)
func moveToLineEnd(e *GoEngine, count int) bool {
	if e.currentBuffer == nil {
		return false
	}

	line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
	if len(line) > 0 {
		// In vim, $ moves to the last character of the line
		lastPos := len(line) - 1
		if e.currentBuffer.cursorCol != lastPos {
			e.currentBuffer.cursorCol = lastPos

			// Update visual selection if in visual mode
			if e.mode == ModeVisual {
				e.visualEnd = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
			}

			return true
		}
	}
	return false
}

// moveWordForward moves to the start of the next word
func moveWordForward(e *GoEngine, count int) bool {
	if e.currentBuffer == nil {
		return false
	}

	if count <= 0 {
		count = 1 // Default to 1 if count is not specified
	}

	moved := false
	for i := 0; i < count; i++ {
		if !moveWordForwardOnce(e) {
			break
		}
		moved = true
	}

	// Update visual selection if in visual mode
	if moved && e.mode == ModeVisual {
		e.visualEnd = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
	}

	return moved
}

// moveWordForwardOnce moves to the start of the next word (helper function)
func moveWordForwardOnce(e *GoEngine) bool {
	line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

	// If we're at the end of the line, move to the next line
	if e.currentBuffer.cursorCol >= len(line)-1 {
		if e.currentBuffer.cursorRow < e.currentBuffer.GetLineCount() {
			e.currentBuffer.cursorRow++
			e.currentBuffer.cursorCol = 0
			return true
		}
		return false
	}

	// Find the next word start
	inWord := isWordChar(line[e.currentBuffer.cursorCol])
	start := e.currentBuffer.cursorCol + 1

	// Skip current word
	for start < len(line) && isWordChar(line[start]) == inWord {
		start++
	}

	// Skip whitespace
	for start < len(line) && isWhitespace(line[start]) {
		start++
	}

	if start < len(line) {
		e.currentBuffer.cursorCol = start
		return true
	} else if e.currentBuffer.cursorRow < e.currentBuffer.GetLineCount() {
		// Move to the start of the next line
		e.currentBuffer.cursorRow++
		e.currentBuffer.cursorCol = 0
		return true
	}

	return false
}

// moveWordBackward moves to the start of the previous word
func moveWordBackward(e *GoEngine, count int) bool {
	if count <= 0 {
		count = 1 // Default to 1 if count is not specified
	}

	moved := false
	for i := 0; i < count; i++ {
		if !moveWordBackwardOnce(e) {
			break
		}
		moved = true
	}

	// Update visual selection if in visual mode
	if moved && e.mode == ModeVisual {
		e.visualEnd = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
	}

	return moved
}

// moveWordBackwardOnce moves to the start of the previous word (helper function)
func moveWordBackwardOnce(e *GoEngine) bool {
	if e.currentBuffer == nil {
		return false
	}

	// If we're at the start of the line, move to the end of the previous line
	if e.currentBuffer.cursorCol == 0 {
		if e.currentBuffer.cursorRow > 1 {
			e.currentBuffer.cursorRow--
			line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
			e.currentBuffer.cursorCol = len(line) - 1
			return true
		}
		return false
	}

	line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

	// Start from the character before current position
	pos := e.currentBuffer.cursorCol - 1

	// Skip whitespace backwards
	for pos > 0 && isWhitespace(line[pos]) {
		pos--
	}

	// If we're in the middle of a word, go to the start of the current word
	if pos > 0 && isWordChar(line[pos]) && isWordChar(line[pos-1]) {
		// We're in the middle of a word, go to its start
		for pos > 0 && isWordChar(line[pos-1]) {
			pos--
		}
	} else if pos > 0 {
		// We're either at the start of a word or on a non-word character

		// If we're on a non-word character, skip backwards over all non-word characters
		if !isWordChar(line[pos]) {
			for pos > 0 && !isWordChar(line[pos-1]) {
				pos--
			}
		}

		// Now find the start of the previous word if we're not already at the start of a word
		if pos > 0 {
			// Skip backwards to a word character
			if !isWordChar(line[pos]) {
				for pos > 0 && !isWordChar(line[pos-1]) {
					pos--
				}
			}

			// If we found a word character, go to the start of that word
			if pos > 0 && isWordChar(line[pos-1]) {
				// Go back to start of the word
				for pos > 0 && isWordChar(line[pos-1]) {
					pos--
				}
			}
		}
	}

	if e.currentBuffer.cursorCol != pos {
		e.currentBuffer.cursorCol = pos
		return true
	}

	return false
}

// moveToLastLine moves to the last line of the buffer
func moveToLastLine(e *GoEngine, count int) bool {
	if e.currentBuffer == nil {
		return false
	}

	lastLine := e.currentBuffer.GetLineCount()
	if e.currentBuffer.cursorRow != lastLine {
		e.currentBuffer.cursorRow = lastLine
		line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
		if e.currentBuffer.cursorCol > len(line) {
			e.currentBuffer.cursorCol = len(line)
		}

		// Update visual selection if in visual mode
		if e.mode == ModeVisual {
			e.visualEnd = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
		}

		return true
	}
	return false
}

// moveToFirstLine moves to the first line of the buffer or to a specific line if count is provided
func moveToFirstLine(e *GoEngine, count int) bool {
	if e.currentBuffer == nil {
		return false
	}

	lineCount := e.currentBuffer.GetLineCount()
	targetRow := 1 // Default to first line

	// If count is provided, go to that specific line number
	if count > 1 {
		targetRow = count
		if targetRow > lineCount {
			targetRow = lineCount // Clamp to last line
		}
	}

	if e.currentBuffer.cursorRow != targetRow {
		e.currentBuffer.cursorRow = targetRow
		line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

		// Move to the first non-blank character on the line
		for i := 0; i < len(line); i++ {
			if !isWhitespace(line[i]) {
				e.currentBuffer.cursorCol = i

				// Update visual selection if in visual mode
				if e.mode == ModeVisual {
					e.visualEnd = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
				}

				return true
			}
		}

		// If the line is all whitespace or empty, move to the beginning
		e.currentBuffer.cursorCol = 0

		// Update visual selection if in visual mode
		if e.mode == ModeVisual {
			e.visualEnd = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
		}

		return true
	}
	return false
}

// isWordChar checks if a character is part of a word (alphanumeric or underscore)
func isWordChar(c byte) bool {
	return (c >= 'a' && c <= 'z') ||
		(c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') ||
		c == '_'
}

// isWhitespace checks if a character is whitespace
func isWhitespace(c byte) bool {
	return c == ' ' || c == '\t'
}

// moveToFirstNonBlank moves to the first non-whitespace character of the line
func moveToFirstNonBlank(e *GoEngine, count int) bool {
	if e.currentBuffer == nil {
		return false
	}

	line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
	for i := 0; i < len(line); i++ {
		if !isWhitespace(line[i]) {
			if e.currentBuffer.cursorCol != i {
				e.currentBuffer.cursorCol = i

				// Update visual selection if in visual mode
				if e.mode == ModeVisual {
					e.visualEnd = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
				}

				return true
			}
			return false
		}
	}

	// If the line is all whitespace, move to the start
	if e.currentBuffer.cursorCol != 0 {
		e.currentBuffer.cursorCol = 0

		// Update visual selection if in visual mode
		if e.mode == ModeVisual {
			e.visualEnd = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
		}

		return true
	}
	return false
}

// moveWordEnd moves to the end of the current or next word
func moveWordEnd(e *GoEngine, count int) bool {
	if count <= 0 {
		count = 1 // Default to 1 if count is not specified
	}

	moved := false
	for i := 0; i < count; i++ {
		if !moveWordEndOnce(e) {
			break
		}
		moved = true
	}

	// Update visual selection if in visual mode
	if moved && e.mode == ModeVisual {
		e.visualEnd = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
	}

	return moved
}

// moveWordEndOnce moves to the end of the current or next word (helper function)
func moveWordEndOnce(e *GoEngine) bool {
	if e.currentBuffer == nil {
		return false
	}

	line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

	// If we're at the end of the line, move to the next line
	if e.currentBuffer.cursorCol >= len(line)-1 {
		if e.currentBuffer.cursorRow < e.currentBuffer.GetLineCount() {
			e.currentBuffer.cursorRow++
			e.currentBuffer.cursorCol = 0
			return moveWordEndOnce(e) // Recursively find word end in next line
		}
		return false
	}

	// Start from the current position
	pos := e.currentBuffer.cursorCol

	// If we're at a word character, find the end of the current word
	if isWordChar(line[pos]) {
		// Move to the end of the current word
		for pos < len(line)-1 && isWordChar(line[pos+1]) {
			pos++
		}

		// If we're already at the end of a word, find the next one
		if pos == e.currentBuffer.cursorCol && pos < len(line)-1 {
			pos++
			// Skip non-word characters
			for pos < len(line)-1 && !isWordChar(line[pos]) {
				pos++
			}
			// Find the end of this next word
			for pos < len(line)-1 && isWordChar(line[pos+1]) {
				pos++
			}
		}
	} else {
		// We're not on a word character, so find the next word
		pos++
		// Skip any remaining non-word characters
		for pos < len(line) && !isWordChar(line[pos]) {
			pos++
		}
		// Find the end of this word
		if pos < len(line) {
			for pos < len(line)-1 && isWordChar(line[pos+1]) {
				pos++
			}
		}
	}

	// Set the new cursor position
	if pos < len(line) && pos != e.currentBuffer.cursorCol {
		e.currentBuffer.cursorCol = pos
		return true
	}

	// If we're at the end of the line, move to the last character
	if pos >= len(line) && e.currentBuffer.cursorCol != len(line)-1 {
		e.currentBuffer.cursorCol = len(line) - 1
		return true
	}

	return false
}

// moveToMatchingBracket moves to the matching bracket (%, bracket matching)
func moveToMatchingBracket(e *GoEngine, count int) bool {
	if e.currentBuffer == nil {
		return false
	}

	// Use the SearchGetMatchingPair function to find the matching bracket
	matchPos := e.SearchGetMatchingPair()

	// If there's no match (returns 0,0), check if we need to look for a bracket first
	if matchPos[0] == 0 && matchPos[1] == 0 {
		// We might need to search for a bracket character on the line
		line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

		// Don't attempt to search if the line is empty
		if len(line) == 0 {
			return false
		}

		// Look for bracket characters to the right of the cursor on the same line
		for col := e.currentBuffer.cursorCol; col < len(line); col++ {
			if isBracketChar(line[col]) {
				// Move the cursor to the bracket and try again
				e.currentBuffer.cursorCol = col

				// Now find the matching bracket
				matchPos = e.SearchGetMatchingPair()
				break
			}
		}

		// If still no match, look to the left
		if matchPos[0] == 0 && matchPos[1] == 0 {
			for col := e.currentBuffer.cursorCol - 1; col >= 0; col-- {
				if isBracketChar(line[col]) {
					// Move the cursor to the bracket and try again
					e.currentBuffer.cursorCol = col

					// Now find the matching bracket
					matchPos = e.SearchGetMatchingPair()
					break
				}
			}
		}
	}

	// If we found a matching bracket, move the cursor to it
	if matchPos[0] > 0 {
		e.currentBuffer.cursorRow = matchPos[0]
		e.currentBuffer.cursorCol = matchPos[1]

		// Update visual selection if in visual mode
		if e.mode == ModeVisual {
			e.visualEnd = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
		}

		return true
	}

	return false
}

// isBracketChar checks if a character is a bracket character
func isBracketChar(c byte) bool {
	return c == '(' || c == ')' || c == '[' || c == ']' || c == '{' || c == '}'
}

// Wrappers for motion functions that save cursor position for undo
func saveAndMoveLeft(e *GoEngine, count int) bool {
	e.UndoSaveCursor()
	return moveLeft(e, count)
}

func saveAndMoveRight(e *GoEngine, count int) bool {
	e.UndoSaveCursor()
	return moveRight(e, count)
}

func saveAndMoveUp(e *GoEngine, count int) bool {
	e.UndoSaveCursor()
	return moveUp(e, count)
}

func saveAndMoveDown(e *GoEngine, count int) bool {
	e.UndoSaveCursor()
	return moveDown(e, count)
}

func saveAndMoveToLineStart(e *GoEngine, count int) bool {
	e.UndoSaveCursor()
	return moveToLineStart(e, count)
}

func saveAndMoveToLineEnd(e *GoEngine, count int) bool {
	e.UndoSaveCursor()
	return moveToLineEnd(e, count)
}

func saveAndMoveToFirstNonBlank(e *GoEngine, count int) bool {
	e.UndoSaveCursor()
	return moveToFirstNonBlank(e, count)
}

func saveAndMoveWordForward(e *GoEngine, count int) bool {
	e.UndoSaveCursor()
	return moveWordForward(e, count)
}

func saveAndMoveWordBackward(e *GoEngine, count int) bool {
	e.UndoSaveCursor()
	return moveWordBackward(e, count)
}

func saveAndMoveWordEnd(e *GoEngine, count int) bool {
	e.UndoSaveCursor()
	return moveWordEnd(e, count)
}

func saveAndMoveToLastLine(e *GoEngine, count int) bool {
	e.UndoSaveCursor()
	return moveToLastLine(e, count)
}

func saveAndMoveToFirstLine(e *GoEngine, count int) bool {
	e.UndoSaveCursor()
	return moveToFirstLine(e, count)
}

func saveAndMoveToMatchingBracket(e *GoEngine, count int) bool {
	e.UndoSaveCursor()
	return moveToMatchingBracket(e, count)
}

// performUndo executes the undo operation
func performUndo(e *GoEngine, count int) bool {
	if count <= 0 {
		count = 1 // Default to 1 if count is not specified
	}

	// Perform undo the specified number of times
	success := false
	for i := 0; i < count; i++ {
		if e.Undo() {
			success = true
		} else {
			break // Stop if we can't undo further
		}
	}

	return success
}

// performRedo executes the redo operation
func performRedo(e *GoEngine, count int) bool {
	if count <= 0 {
		count = 1 // Default to 1 if count is not specified
	}

	// Perform redo the specified number of times
	success := false
	for i := 0; i < count; i++ {
		if e.Redo() {
			success = true
		} else {
			break // Stop if we can't redo further
		}
	}

	return success
}

// repeatLastEdit repeats the last edit operation (the dot command)
func repeatLastEdit(e *GoEngine, count int) bool {
	if e.currentBuffer == nil || e.lastEditCommand == "" {
		return false // No buffer or no previous edit to repeat
	}

	// Use provided count if specified, otherwise default to 1 for dot command
	effectiveCount := count
	if effectiveCount == 0 {
		effectiveCount = 1  // Default to 1 repetition for simple dot command
	}

	// Track if any operation was successful
	success := false

	// Execute based on the last edit command
	switch e.lastEditCommand {
	case "x":
		// Repeat character deletion
		line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
		if e.currentBuffer.cursorCol < len(line) {
			// Save the current state for undo before changing anything
			e.UndoSaveRegion(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow)

			// Determine how many characters we can actually delete
			charsToDelete := effectiveCount
			if e.currentBuffer.cursorCol+charsToDelete > len(line) {
				charsToDelete = len(line) - e.currentBuffer.cursorCol
			}

			// Delete the characters under and after the cursor
			newLine := ""
			if e.currentBuffer.cursorCol > 0 {
				newLine = line[:e.currentBuffer.cursorCol]
			}
			if e.currentBuffer.cursorCol+charsToDelete < len(line) {
				newLine += line[e.currentBuffer.cursorCol+charsToDelete:]
			}
			e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{newLine})

			// Adjust cursor if at end of line
			if len(newLine) > 0 && e.currentBuffer.cursorCol >= len(newLine) {
				e.currentBuffer.cursorCol = len(newLine) - 1
			}

			success = true
		}

	case "~":
		// Repeat case toggle
		line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
		if e.currentBuffer.cursorCol < len(line) {
			// Save the current state for undo before changing anything
			e.UndoSaveRegion(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow)

			// Determine how many characters we can actually change
			charsToChange := effectiveCount
			if e.currentBuffer.cursorCol+charsToChange > len(line) {
				charsToChange = len(line) - e.currentBuffer.cursorCol
			}

			// Create the new line with case changes
			newLine := ""
			if e.currentBuffer.cursorCol > 0 {
				newLine = line[:e.currentBuffer.cursorCol]
			}

			// Change the case of each character
			for i := 0; i < charsToChange; i++ {
				pos := e.currentBuffer.cursorCol + i
				if pos < len(line) {
					char := line[pos]
					if char >= 'a' && char <= 'z' {
						// Convert lowercase to uppercase
						newLine += string(char - 32)
					} else if char >= 'A' && char <= 'Z' {
						// Convert uppercase to lowercase
						newLine += string(char + 32)
					} else {
						// Non-alphabetic character remains unchanged
						newLine += string(char)
					}
				}
			}

			// Add the rest of the line
			if e.currentBuffer.cursorCol+charsToChange < len(line) {
				newLine += line[e.currentBuffer.cursorCol+charsToChange:]
			}

			// Update the line
			e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{newLine})

			// Move cursor forward by the number of characters changed
			e.currentBuffer.cursorCol += charsToChange

			// Ensure cursor doesn't go past end of line
			if e.currentBuffer.cursorCol >= len(newLine) {
				e.currentBuffer.cursorCol = len(newLine) - 1
				if e.currentBuffer.cursorCol < 0 {
					e.currentBuffer.cursorCol = 0
				}
			}

			success = true
		}

	case "r":
		// Repeat character replacement
		if e.lastEditText == "" {
			return false // No replacement character stored
		}
		
		line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
		if e.currentBuffer.cursorCol < len(line) {
			// Save the current state for undo before changing anything
			e.UndoSaveRegion(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow)

			// Determine how many characters we can actually replace
			charsToReplace := effectiveCount
			if e.currentBuffer.cursorCol+charsToReplace > len(line) {
				charsToReplace = len(line) - e.currentBuffer.cursorCol
			}

			// Create the new line with replacements
			newLine := ""
			if e.currentBuffer.cursorCol > 0 {
				newLine = line[:e.currentBuffer.cursorCol]
			}

			// Replace each character with the stored replacement character
			for i := 0; i < charsToReplace; i++ {
				newLine += e.lastEditText
			}

			// Add the rest of the line
			if e.currentBuffer.cursorCol+charsToReplace < len(line) {
				newLine += line[e.currentBuffer.cursorCol+charsToReplace:]
			}

			// Update the line
			e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{newLine})

			// Move cursor forward by the number of characters replaced (but not past end)
			e.currentBuffer.cursorCol += charsToReplace - 1
			if e.currentBuffer.cursorCol >= len(newLine) && len(newLine) > 0 {
				e.currentBuffer.cursorCol = len(newLine) - 1
			}
			if e.currentBuffer.cursorCol < 0 {
				e.currentBuffer.cursorCol = 0
			}

			success = true
		}

	case "s":
		// Repeat substitute command (delete characters and insert text)
		if e.lastEditText == "" {
			// If no text was captured, treat it like just deletion
		}
		
		line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
		if e.currentBuffer.cursorCol < len(line) {
			// Save the current state for undo before changing anything
			e.UndoSaveRegion(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow)

			// Determine how many characters we can actually delete
			charsToDelete := effectiveCount
			if e.currentBuffer.cursorCol+charsToDelete > len(line) {
				charsToDelete = len(line) - e.currentBuffer.cursorCol
			}

			// Delete the characters under and after the cursor
			newLine := ""
			if e.currentBuffer.cursorCol > 0 {
				newLine = line[:e.currentBuffer.cursorCol]
			}
			
			// Insert the captured text
			newLine += e.lastEditText
			
			// Add the rest of the line (after the deleted characters)
			if e.currentBuffer.cursorCol+charsToDelete < len(line) {
				newLine += line[e.currentBuffer.cursorCol+charsToDelete:]
			}

			// Update the line
			e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{newLine})

			// Position cursor at the end of the inserted text
			if len(e.lastEditText) > 0 {
				e.currentBuffer.cursorCol += len(e.lastEditText) - 1
			}
			
			// Ensure cursor doesn't go past end of line
			if e.currentBuffer.cursorCol >= len(newLine) && len(newLine) > 0 {
				e.currentBuffer.cursorCol = len(newLine) - 1
			}
			if e.currentBuffer.cursorCol < 0 {
				e.currentBuffer.cursorCol = 0
			}

			success = true
		}

	case "dw":
		// Repeat delete word operation
		e.UndoSaveRegion(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow)
		
		// Repeat the original delete word operation (with its original count) effectiveCount times
		for i := 0; i < effectiveCount; i++ {
			for j := 0; j < e.lastEditCount; j++ {
				e.deleteWordRaw()
			}
		}
		success = true

	case "d$":
		// Repeat delete to end of line operation
		// For d$, we multiply the counts: original count * dot repetition count
		totalCount := e.lastEditCount * effectiveCount
		e.deleteToEndOfLineWithCount(totalCount)
		success = true

	case "d0":
		// Repeat delete to start of line operation  
		// For d0, we multiply the counts: original count * dot repetition count
		totalCount := e.lastEditCount * effectiveCount
		e.deleteToStartOfLineWithCount(totalCount)
		success = true

	case "db":
		// Repeat delete backward word operation
		e.UndoSaveRegion(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow)
		
		// Repeat the original delete backward word operation (with its original count) effectiveCount times
		for i := 0; i < effectiveCount; i++ {
			for j := 0; j < e.lastEditCount; j++ {
				e.deleteBackwardWordRaw()
			}
		}
		success = true

	case "de":
		// Repeat delete to word end operation
		e.UndoSaveRegion(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow)
		
		// Repeat the original delete to word end operation (with its original count) effectiveCount times
		for i := 0; i < effectiveCount; i++ {
			for j := 0; j < e.lastEditCount; j++ {
				e.deleteToWordEndRaw()
			}
		}
		success = true

	case "dd":
		// Repeat delete lines operation
		// For dd, we multiply the counts: original count * dot repetition count  
		totalCount := e.lastEditCount * effectiveCount
		e.deleteLines(totalCount)
		success = true
		
	case "i":
		// Repeat insert command - insert the same text at current position
		if e.lastEditText != "" {
			// Save for undo
			e.UndoSaveRegion(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow)
			
			// Repeat the text insertion count times
			for i := 0; i < effectiveCount; i++ {
				line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
				newLine := ""
				if e.currentBuffer.cursorCol > 0 {
					newLine = line[:e.currentBuffer.cursorCol]
				}
				newLine += e.lastEditText
				if e.currentBuffer.cursorCol < len(line) {
					newLine += line[e.currentBuffer.cursorCol:]
				}
				e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{newLine})
				
				// Move cursor to end of inserted text
				e.currentBuffer.cursorCol += len(e.lastEditText)
			}
			success = true
		}
		
	case "o":
		// Repeat open line below command - create new line with same text
		if e.lastEditText != "" {
			// Save for undo
			e.UndoSaveRegion(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow)
			
			// Repeat the line creation count times
			for i := 0; i < effectiveCount; i++ {
				e.currentBuffer.SetLines(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow, []string{e.lastEditText})
				e.currentBuffer.cursorRow++
				e.currentBuffer.cursorCol = len(e.lastEditText)
			}
			success = true
		}
		
	case "O":
		// Repeat open line above command - create new line with same text
		if e.lastEditText != "" {
			// Save for undo
			e.UndoSaveRegion(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow-1)
			
			// Repeat the line creation count times
			for i := 0; i < effectiveCount; i++ {
				e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow-1, []string{e.lastEditText})
				e.currentBuffer.cursorCol = len(e.lastEditText)
			}
			success = true
		}
		
	case ">>":
		// Repeat line indent command
		
		// Determine how many times to repeat and how many lines to affect
		var repeatCount, lineCount int
		
		if effectiveCount == 0 {
			// Simple dot command (.) - use original behavior
			repeatCount = 1
			lineCount = e.lastEditCount
			if lineCount == 0 {
				lineCount = 1
			}
		} else {
			// Dot command with count (e.g., 2.) - repeat the operation
			repeatCount = effectiveCount
			lineCount = e.lastEditCount
			if lineCount == 0 {
				lineCount = 1
			}
		}

		// Save for undo
		e.UndoSaveRegion(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow+lineCount-1)

		// Apply indentation: repeatCount times, each time affecting lineCount lines
		for rep := 0; rep < repeatCount; rep++ {
			for i := 0; i < lineCount && (e.currentBuffer.cursorRow+i) <= e.currentBuffer.GetLineCount(); i++ {
				lineNum := e.currentBuffer.cursorRow + i
				line := e.currentBuffer.GetLine(lineNum)
				newLine := e.indentLine(line)
				e.currentBuffer.SetLines(lineNum-1, lineNum, []string{newLine})
			}
		}

		// Move cursor to first non-blank character of the current line
		line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
		e.currentBuffer.cursorCol = 0
		for i := 0; i < len(line); i++ {
			if line[i] != ' ' && line[i] != '\t' {
				e.currentBuffer.cursorCol = i
				break
			}
		}
		success = true
		
	case "<<":
		// Repeat line dedent command
		// For dot command, count represents number of repetitions, not lines
		repeatCount := effectiveCount
		if repeatCount == 0 {
			repeatCount = 1
		}

		// Get the original line count from the last edit
		lineCount := e.lastEditCount
		if lineCount == 0 {
			lineCount = 1
		}

		// Save for undo
		e.UndoSaveRegion(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow+lineCount-1)

		// Repeat the dedentation operation repeatCount times
		for rep := 0; rep < repeatCount; rep++ {
			// Apply dedentation to lineCount lines starting from current line
			for i := 0; i < lineCount && (e.currentBuffer.cursorRow+i) <= e.currentBuffer.GetLineCount(); i++ {
				lineNum := e.currentBuffer.cursorRow + i
				line := e.currentBuffer.GetLine(lineNum)
				newLine := e.dedentLine(line)
				e.currentBuffer.SetLines(lineNum-1, lineNum, []string{newLine})
			}
		}

		// Move cursor to first non-blank character of the current line
		line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
		e.currentBuffer.cursorCol = 0
		for i := 0; i < len(line); i++ {
			if line[i] != ' ' && line[i] != '\t' {
				e.currentBuffer.cursorCol = i
				break
			}
		}
		success = true
		
	case "visual_indent":
		// Repeat visual line indent - apply one indentation to the specified number of lines
		lineCount := e.lastEditCount
		if lineCount == 0 {
			lineCount = 1
		}
		
		// For visual operations, effectiveCount represents how many times to repeat the entire operation
		repeatCount := effectiveCount
		if repeatCount == 0 {
			repeatCount = 1
		}

		// Save for undo
		e.UndoSaveRegion(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow+(lineCount*repeatCount)-1)

		// Apply indentation to lineCount lines, repeatCount times
		for rep := 0; rep < repeatCount; rep++ {
			for i := 0; i < lineCount && (e.currentBuffer.cursorRow+i) <= e.currentBuffer.GetLineCount(); i++ {
				lineNum := e.currentBuffer.cursorRow + i
				line := e.currentBuffer.GetLine(lineNum)
				newLine := e.indentLine(line)
				e.currentBuffer.SetLines(lineNum-1, lineNum, []string{newLine})
			}
		}

		// Move cursor to first non-blank character of the current line
		line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
		e.currentBuffer.cursorCol = 0
		for i := 0; i < len(line); i++ {
			if line[i] != ' ' && line[i] != '\t' {
				e.currentBuffer.cursorCol = i
				break
			}
		}
		success = true
		
	case "visual_dedent":
		// Repeat visual line dedent - apply one dedentation to the specified number of lines
		lineCount := e.lastEditCount
		if lineCount == 0 {
			lineCount = 1
		}
		
		// If user specified a count with dot command, repeat the entire operation that many times
		repeatCount := effectiveCount
		if repeatCount == 0 {
			repeatCount = 1
		}

		// Save for undo
		e.UndoSaveRegion(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow+lineCount-1)

		// Apply the visual dedentation operation repeatCount times
		for rep := 0; rep < repeatCount; rep++ {
			for i := 0; i < lineCount && (e.currentBuffer.cursorRow+i) <= e.currentBuffer.GetLineCount(); i++ {
				lineNum := e.currentBuffer.cursorRow + i
				line := e.currentBuffer.GetLine(lineNum)
				newLine := e.dedentLine(line)
				e.currentBuffer.SetLines(lineNum-1, lineNum, []string{newLine})
			}
		}

		// Move cursor to first non-blank character of the current line
		line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
		e.currentBuffer.cursorCol = 0
		for i := 0; i < len(line); i++ {
			if line[i] != ' ' && line[i] != '\t' {
				e.currentBuffer.cursorCol = i
				break
			}
		}
		success = true
		
	case "cw":
		// Repeat change word command (delete word and insert text)
		// Note: lastEditText can be empty if user did cw+ESC without typing anything
		
		// Save for undo
		e.UndoSaveRegion(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow)
		
		// Repeat the change operation count times
		for i := 0; i < effectiveCount; i++ {
			// Delete the word at cursor using change-specific deletion (preserves whitespace)
			// Use the original count that was used with the cw command (raw version to avoid double undo save)
			e.changeWordDeleteRaw(e.lastEditCount)
			
			// Insert the captured text at cursor position
			line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
			newLine := ""
			if e.currentBuffer.cursorCol > 0 {
				newLine = line[:e.currentBuffer.cursorCol]
			}
			newLine += e.lastEditText
			if e.currentBuffer.cursorCol < len(line) {
				newLine += line[e.currentBuffer.cursorCol:]
			}
			e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{newLine})
			
			// Position cursor after the inserted text for the next iteration
			e.currentBuffer.cursorCol += len(e.lastEditText)
			
			// Adjust cursor to not go past line end
			if e.currentBuffer.cursorCol > len(newLine) {
				e.currentBuffer.cursorCol = len(newLine)
			}
			
			// For multiple repetitions, move to the next word for the next iteration
			// For the final position (after all iterations), move back to last char of inserted text
			if i < effectiveCount-1 {
				// Not the last iteration - move to next word start
				line = e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
				// Skip any whitespace to get to next word
				for e.currentBuffer.cursorCol < len(line) && !isWordChar(line[e.currentBuffer.cursorCol]) {
					e.currentBuffer.cursorCol++
				}
			} else {
				// Last iteration - position cursor at last character of inserted text (vim behavior)
				if e.currentBuffer.cursorCol > 0 {
					e.currentBuffer.cursorCol--
				}
			}
		}
		success = true
		
	case "c$":
		// Repeat change to end of line (with original count)
		for i := 0; i < effectiveCount; i++ {
			// Delete to end of line using the original count
			e.deleteToEndOfLineWithCount(e.lastEditCount)
			
			// Insert the recorded text
			if e.lastEditText != "" {
				line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
				newLine := ""
				if e.currentBuffer.cursorCol > 0 {
					newLine = line[:e.currentBuffer.cursorCol]
				}
				newLine += e.lastEditText
				e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{newLine})
				
				// Position cursor after inserted text
				e.currentBuffer.cursorCol += len(e.lastEditText)
				if e.currentBuffer.cursorCol > 0 {
					e.currentBuffer.cursorCol--
				}
			}
		}
		success = true
		
	case "c0":
		// Repeat change to start of line
		for i := 0; i < effectiveCount; i++ {
			// Delete to start of line
			e.deleteToStartOfLine()
			
			// Insert the recorded text
			if e.lastEditText != "" {
				line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
				newLine := e.lastEditText + line
				e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{newLine})
				
				// Position cursor after inserted text
				e.currentBuffer.cursorCol = len(e.lastEditText)
				if e.currentBuffer.cursorCol > 0 {
					e.currentBuffer.cursorCol--
				}
			}
		}
		success = true
		
	case "cb":
		// Repeat change backward word
		for i := 0; i < effectiveCount; i++ {
			// Delete backward word
			e.deleteBackwardWord()
			
			// Insert the recorded text
			if e.lastEditText != "" {
				line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
				newLine := ""
				if e.currentBuffer.cursorCol > 0 {
					newLine = line[:e.currentBuffer.cursorCol]
				}
				newLine += e.lastEditText
				if e.currentBuffer.cursorCol < len(line) {
					newLine += line[e.currentBuffer.cursorCol:]
				}
				e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{newLine})
				
				// Position cursor after inserted text
				e.currentBuffer.cursorCol += len(e.lastEditText)
				if e.currentBuffer.cursorCol > 0 {
					e.currentBuffer.cursorCol--
				}
			}
		}
		success = true
		
	case "ce":
		// Repeat change to word end
		for i := 0; i < effectiveCount; i++ {
			// Delete to word end
			e.deleteToWordEnd()
			
			// Insert the recorded text
			if e.lastEditText != "" {
				line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
				newLine := ""
				if e.currentBuffer.cursorCol > 0 {
					newLine = line[:e.currentBuffer.cursorCol]
				}
				newLine += e.lastEditText
				if e.currentBuffer.cursorCol < len(line) {
					newLine += line[e.currentBuffer.cursorCol:]
				}
				e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{newLine})
				
				// Position cursor after inserted text
				e.currentBuffer.cursorCol += len(e.lastEditText)
				if e.currentBuffer.cursorCol > 0 {
					e.currentBuffer.cursorCol--
				}
			}
		}
		success = true
		
	case "cc":
		// Repeat change line
		for i := 0; i < effectiveCount; i++ {
			// Delete the entire line (use original count)
			e.deleteLines(e.lastEditCount)
			
			// Insert the recorded text as a new line
			if e.lastEditText != "" {
				e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow-1, []string{e.lastEditText})
				
				// Position cursor after inserted text
				e.currentBuffer.cursorCol = len(e.lastEditText)
				if e.currentBuffer.cursorCol > 0 {
					e.currentBuffer.cursorCol--
				}
			} else {
				// If no text was inserted, create an empty line
				e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow-1, []string{""})
				e.currentBuffer.cursorCol = 0
			}
		}
		success = true
		
	// Add cases for other commands as they are implemented
	}

	return success
}
