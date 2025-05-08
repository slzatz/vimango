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
