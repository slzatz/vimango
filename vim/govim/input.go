package govim

import (
	"strings"
)

// Helper function to check if a character is a digit
func isDigit(s string) bool {
	if len(s) != 1 {
		return false
	}
	return s[0] >= '0' && s[0] <= '9'
}

// updateVisualSelection updates the end of the visual selection to match the current cursor position
func (e *GoEngine) updateVisualSelection() {
	if e.mode == ModeVisual && e.currentBuffer != nil {
		e.visualEnd = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
	}
}

// enterVisualMode initializes visual mode with the specified visual type
func (e *GoEngine) enterVisualMode(visualType int) {
	if e.currentBuffer == nil {
		return
	}

	e.mode = ModeVisual
	e.visualStart = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol} // Set start of selection to cursor position
	e.visualEnd = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}   // Set end of selection to cursor position
	e.visualType = visualType                                                    // Visual type (0 = char, 1 = line, 2 = block)
}

// exitVisualMode cleans up state when exiting visual mode
func (e *GoEngine) exitVisualMode() {
	if e.currentBuffer == nil {
		return
	}

	// When exiting visual mode, position cursor at visual start
	// (where we were when we entered visual mode)
	e.currentBuffer.cursorRow = e.visualStart[0]
	e.currentBuffer.cursorCol = e.visualStart[1]
	
	// For line-wise visual mode, if we're at the start of a line, move to first non-blank character
	if e.visualType == 1 && e.currentBuffer.cursorCol == 0 {
		line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
		for i := 0; i < len(line); i++ {
			if line[i] != ' ' && line[i] != '\t' {
				e.currentBuffer.cursorCol = i
				break
			}
		}
	}
	
	e.mode = ModeNormal
}

// visualOperation performs a visual mode operation and switches to the appropriate mode
func (e *GoEngine) visualOperation(op string) {
	switch op {
	case "y":
		e.yankVisualSelection()
		e.mode = ModeNormal // Return to normal mode after yanking
	case "d", "x":
		e.deleteVisualSelection()
		e.mode = ModeNormal // Return to normal mode after deleting
	case "c":
		e.deleteVisualSelection()
		e.mode = ModeInsert // Enter insert mode after deleting
	case "~":
		e.changeCaseVisualSelection()
		e.mode = ModeNormal // Return to normal mode after changing case
	case ">":
		e.indentVisualSelection()
		e.mode = ModeNormal // Return to normal mode after indenting
	case "<":
		e.dedentVisualSelection()
		e.mode = ModeNormal // Return to normal mode after dedenting
	}
}

// Input processes input (basic motion commands)
func (e *GoEngine) Input(s string) {
	// Handle escape key for all modes
	if s == "\x1b" { // ESC key
		// Remember what mode we're coming from
		prevMode := e.mode

		// Update general state
		e.searching = false
		e.searchBuffer = ""
		e.awaitingMotion = false
		e.currentCommand = ""
		e.commandCount = 0
		e.buildingCount = false
		e.awaitingReplace = false

		if prevMode == ModeInsert {
			// Reset the insert undo group flag
			e.inInsertUndoGroup = false

			// Sync to create a clear undo point
			e.UndoSync(true)

			// Set mode to normal
			e.mode = ModeNormal

			// Fix cursor position for insert mode exit
			if e.currentBuffer != nil {
				line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
				lineLen := len(line)

				// Standard Vim behavior: When leaving insert mode with ESC,
				// the cursor should always move back one position if possible

				// First ensure cursor is not past end of line
				if e.currentBuffer.cursorCol > lineLen {
					e.currentBuffer.cursorCol = lineLen
				}

				// Then apply the "move back one character" rule:
				// If at the end of a non-empty line, move back to the last character
				if lineLen > 0 && e.currentBuffer.cursorCol == lineLen {
					e.currentBuffer.cursorCol = lineLen - 1
				} else if e.currentBuffer.cursorCol > 0 {
					// If not at the end but not at the beginning, move back
					e.currentBuffer.cursorCol--
				}

				// Check if this was an s-triggered insert mode and capture the inserted text
				if e.sCommandActive {
					// Calculate the inserted text from the starting position to current position (before cursor moved back)
					line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
					
					// The actual end position before cursor was moved back
					actualEndCol := e.currentBuffer.cursorCol + 1
					
					// Extract the inserted text
					insertedText := ""
					if e.sCommandStartCol < len(line) && e.sCommandStartCol <= actualEndCol {
						if actualEndCol <= len(line) {
							insertedText = line[e.sCommandStartCol:actualEndCol]
						} else {
							insertedText = line[e.sCommandStartCol:]
						}
					}
					
					// Record the s command with the captured text
					e.recordEditCommandWithText("s", e.sCommandCount, insertedText)
					
					// Reset s command tracking
					e.sCommandActive = false
					e.sCommandCount = 0
					e.sCommandStartCol = 0
				}
				
				// Check if this was an insert command (i, o, O) and capture the inserted text
				if e.insertCommandActive {
					var insertedText string
					
					switch e.insertCommandType {
					case "i":
						// For 'i' command, capture text from the original position
						line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
						actualEndCol := e.currentBuffer.cursorCol + 1
						startCol := e.insertCommandPos[1]
						
						if startCol < len(line) && startCol <= actualEndCol {
							if actualEndCol <= len(line) {
								insertedText = line[startCol:actualEndCol]
							} else {
								insertedText = line[startCol:]
							}
						}
						
					case "o":
						// For 'o' command, capture the entire line that was created
						if e.currentBuffer.cursorRow > e.insertCommandPos[0] {
							insertedText = e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
						}
						
					case "O":
						// For 'O' command, capture the entire line that was created
						if e.currentBuffer.cursorRow >= e.insertCommandPos[0] {
							insertedText = e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
						}
					}
					
					// Record the insert command with the captured text
					e.recordEditCommandWithText(e.insertCommandType, e.insertCommandCount, insertedText)
					
					// Reset insert command tracking
					e.insertCommandActive = false
					e.insertCommandType = ""
					e.insertCommandCount = 0
					e.insertCommandPos = [2]int{0, 0}
				}

				// Check if this was a change command and capture the inserted text
				if e.changeCommandActive {
					// For change commands, we need to be more careful about text capture
					// because the text between startCol and cursor might be existing text, not inserted text
					
					// Calculate the inserted text from the starting position to current position (before cursor moved back)
					line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
					
					// The actual end position before cursor was moved back
					actualEndCol := e.currentBuffer.cursorCol + 1
					
					// Extract the inserted text from the change starting position
					insertedText := ""
					startCol := e.changeCommandPos[1]
					
					// Special case: if cursor is still at the starting position (or moved back to it due to ESC positioning),
					// and we haven't typed anything, then there's no inserted text
					if e.currentBuffer.cursorCol == startCol {
						// No text was inserted - cursor is at the same position where change started
						insertedText = ""
					} else if startCol < len(line) && startCol < actualEndCol {
						if actualEndCol <= len(line) {
							insertedText = line[startCol:actualEndCol]
						} else {
							insertedText = line[startCol:]
						}
					}
					
					// Record the change command with the captured text
					e.recordEditCommandWithText(e.changeCommandType, e.changeCommandCount, insertedText)
					
					// Reset change command tracking
					e.changeCommandActive = false
					e.changeCommandType = ""
					e.changeCommandCount = 0
					e.changeCommandPos = [2]int{0, 0}
				}
			}
		} else if prevMode == ModeVisual {
			// When exiting visual mode, position cursor at visual start
			e.exitVisualMode()
		} else {
			// For other modes like normal or command
			e.mode = ModeNormal

			// Ensure cursor is not past the end of the line
			if e.currentBuffer != nil {
				line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
				lineLen := len(line)

				// Make sure cursor is not past the end of the line
				if e.currentBuffer.cursorCol >= lineLen {
					if lineLen > 0 {
						e.currentBuffer.cursorCol = lineLen - 1
					} else {
						e.currentBuffer.cursorCol = 0
					}
				}
			}
		}
		return
	}

	// Handle visual mode commands
	if e.mode == ModeVisual {
		// First check for operations on the visual selection
		switch s {
		case "y": // yank selection
			e.visualOperation("y")
			return
		case "d", "x": // delete selection
			e.visualOperation("d")
			return
		case "c": // change selection (delete and enter insert mode)
			e.visualOperation("c")
			return
		case "~": // change case of selection
			e.visualOperation("~")
			return
		case ">": // indent selection (only for line-wise visual mode)
			if e.visualType == 1 { // Only for line-wise visual mode (V)
				e.visualOperation(">")
			}
			return
		case "<": // dedent selection (only for line-wise visual mode)
			if e.visualType == 1 { // Only for line-wise visual mode (V)
				e.visualOperation("<")
			}
			return
		}

		// Motion commands in visual mode - use the normal mode handlers
		if handler, exists := motionHandlers[s]; exists {
			handler(e, e.commandCount) // Execute the motion command with count
			// Reset count after executing the command
			e.commandCount = 0
			e.buildingCount = false

			// Ensure visualEnd is updated after any motion
			e.updateVisualSelection()
			return
		}
	}

	// Handle normal mode commands
	if e.mode == ModeNormal {
		// Check if we're waiting for a replacement character after 'r'
		if e.awaitingReplace {
			if e.currentBuffer != nil {
				// Get the count (default to 1 if no count specified)
				count := e.commandCount
				if count == 0 {
					count = 1
				}

				// Get the current line
				line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

				// Make sure the cursor is on a valid position
				if e.currentBuffer.cursorCol < len(line) {
					// Save for undo
					e.UndoSaveRegion(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow)

					// Determine how many characters we can actually replace
					charsToReplace := count
					if e.currentBuffer.cursorCol+charsToReplace > len(line) {
						charsToReplace = len(line) - e.currentBuffer.cursorCol
					}

					// Create the new line with replacements
					newLine := ""
					if e.currentBuffer.cursorCol > 0 {
						newLine = line[:e.currentBuffer.cursorCol]
					}

					// Replace the specified number of characters
					for i := 0; i < charsToReplace; i++ {
						newLine += s
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

					// Record this edit for the dot command
					e.recordEditCommandWithText("r", count, s)
				}

				// Exit replace mode and reset count
				e.awaitingReplace = false
				e.commandCount = 0
				e.buildingCount = false
				return
			}
		}

		// First check for numeric prefix
		if isDigit(s) && (e.buildingCount || s != "0" || (e.currentCommand != "" && !e.awaitingMotion)) {
			// Treat digits as part of a count if any of these are true:
			// 1. We're already building a count (e.buildingCount is true)
			// 2. The digit is not "0" (so "0" alone is a motion command, not a count)
			// 3. We have a current command but are NOT awaiting motion (prevent "0" in "c0" from being treated as count)
			digit := int(s[0] - '0')

			if e.buildingCount {
				// Continue building the count
				e.commandCount = e.commandCount*10 + digit
			} else {
				// Start a new count
				e.commandCount = digit
				e.buildingCount = true
			}
			return
		}

		// Special handling for "gg" command
		if s == "g" && e.currentCommand == "" {
			// First 'g' pressed - store state
			e.currentCommand = "g"
			return
		} else if e.currentCommand == "g" {
			if s == "g" {
				// Second 'g' pressed - execute "gg" command
				if handler, exists := motionHandlers["g"]; exists {
					count := e.commandCount
					if count == 0 {
						count = 1 // If no count specified, default to line 1 for "gg"
					}
					handler(e, count) // Use the 'g' handler for the "gg" command
					e.currentCommand = ""
					e.commandCount = 0
					e.buildingCount = false
					return
				}
			}
			// Any other key after 'g' - reset state
			e.currentCommand = ""
		}

		// Special case for verb+motion combinations if we're in awaiting motion state
		if len(s) == 1 && e.awaitingMotion {
			switch e.currentCommand {
			case "d":
				switch s {
				case "w":
					e.deleteWord()
					e.recordEditCommand("dw", 1)
					e.awaitingMotion = false
					e.currentCommand = ""
					return
				case "$":
					e.deleteToEndOfLine()
					e.recordEditCommand("d$", 1)
					e.awaitingMotion = false
					e.currentCommand = ""
					return
				case "0":
					e.deleteToStartOfLine()
					e.recordEditCommand("d0", 1)
					e.awaitingMotion = false
					e.currentCommand = ""
					return
				case "b":
					e.deleteBackwardWord()
					e.recordEditCommand("db", 1)
					e.awaitingMotion = false
					e.currentCommand = ""
					return
				case "e":
					e.deleteToWordEnd()
					e.recordEditCommand("de", 1)
					e.awaitingMotion = false
					e.currentCommand = ""
					return
				case "d":
					// Special case for "dd" - delete entire line
					e.deleteLines(1)
					e.recordEditCommand("dd", 1)
					e.awaitingMotion = false
					e.currentCommand = ""
					return
				}
			case "c":
				switch s {
				case "w":
					count := e.commandCount
					if count == 0 {
						count = 1
					}
					e.changeWord(count)
					e.awaitingMotion = false
					e.currentCommand = ""
					return
				case "$":
					e.changeToEndOfLine()
					e.awaitingMotion = false
					e.currentCommand = ""
					return
				case "0":
					e.changeToStartOfLine()
					e.awaitingMotion = false
					e.currentCommand = ""
					return
				case "b":
					e.changeBackwardWord()
					e.awaitingMotion = false
					e.currentCommand = ""
					return
				case "e":
					e.changeToWordEnd()
					e.awaitingMotion = false
					e.currentCommand = ""
					return
				case "c":
					// Special case for "cc" - change entire line
					e.deleteLines(1)
					e.mode = ModeInsert
					e.awaitingMotion = false
					e.currentCommand = ""
					return
				}
			case "y":
				switch s {
				case "w":
					e.yankWord()
					e.awaitingMotion = false
					e.currentCommand = ""
					return
				case "$":
					e.yankToEndOfLine()
					e.awaitingMotion = false
					e.currentCommand = ""
					return
				case "0":
					e.yankToStartOfLine()
					e.awaitingMotion = false
					e.currentCommand = ""
					return
				case "b":
					e.yankBackwardWord()
					e.awaitingMotion = false
					e.currentCommand = ""
					return
				case "e":
					e.yankToWordEnd()
					e.awaitingMotion = false
					e.currentCommand = ""
					return
				case "y":
					// Special case for "yy" - yank entire line
					line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
					e.yankRegister = line + "\n"
					e.yankRegisterType = 1 // Mark as line-wise yank
					e.awaitingMotion = false
					e.currentCommand = ""
					return
				}
			}
		}

		// Special case for colon which needs to be handled differently
		if s == ":" {
			e.mode = ModeCommand
			return
		}

		// Handle mode changes
		switch s {
		case "r": // replace character under cursor
			e.awaitingReplace = true
			return
		case "i": // insert mode
			// Set up tracking for insert command
			e.insertCommandActive = true
			e.insertCommandType = "i"
			e.insertCommandCount = e.commandCount
			if e.insertCommandCount == 0 {
				e.insertCommandCount = 1
			}
			e.insertCommandPos = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
			
			e.mode = ModeInsert
			e.startInsertUndoGroup("") // Start a new insert undo group (regular insert)
			
			// Reset count after execution
			e.commandCount = 0
			e.buildingCount = false
			return
		case "v": // character-wise visual mode
			e.enterVisualMode(0) // 0 for character-wise visual mode
			return
		case "V": // line-wise visual mode
			e.enterVisualMode(1) // 1 for line-wise visual mode
			return
		case "a": // append (insert after cursor)
			if e.currentBuffer != nil {
				line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
				// Move cursor one position right if not at end of line
				if len(line) > 0 && e.currentBuffer.cursorCol < len(line) {
					e.currentBuffer.cursorCol++
				}
				e.mode = ModeInsert
				e.startInsertUndoGroup("a") // Start a new insert undo group (append)
			}
			return
		case "A": // append at end of line
			if e.currentBuffer != nil {
				line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
				// Move cursor to end of line
				e.currentBuffer.cursorCol = len(line)
				e.mode = ModeInsert
				e.startInsertUndoGroup("A") // Start a new insert undo group (append at end)
			}
			return
		}

		// Handle all movement commands using the motion handlers
		if handler, exists := motionHandlers[s]; exists {
			handler(e, e.commandCount) // Execute the motion command with count

			// Reset count and buildingCount after executing the command
			e.commandCount = 0
			e.buildingCount = false
			return
		}

		// Handle char deletion with x
		if s == "x" && e.currentBuffer != nil {
			count := e.commandCount
			if count == 0 {
				count = 1 // Default to 1 if no count specified
			}

			line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
			if e.currentBuffer.cursorCol < len(line) {
				// Save the current state for undo before changing anything
				e.UndoSaveRegion(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow)

				// Determine how many characters we can actually delete
				charsToDelete := count
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

				// Record this edit for the dot command
				e.recordEditCommand("x", count)
			}

			// Reset count after execution
			e.commandCount = 0
			e.buildingCount = false
			return
		}

		// Handle the tilde (~) command to change case
		if s == "~" && e.currentBuffer != nil {
			count := e.commandCount
			if count == 0 {
				count = 1 // Default to 1 if no count specified
			}

			line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
			if e.currentBuffer.cursorCol < len(line) {
				// Save the current state for undo before changing anything
				e.UndoSaveRegion(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow)

				// Determine how many characters we can actually change
				charsToChange := count
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

				// Record this edit for the dot command
				e.recordEditCommand("~", count)
			}

			// Reset count after execution
			e.commandCount = 0
			e.buildingCount = false
			return
		}

		// Handle 's' command (substitute) - delete character(s) and enter insert mode
		if s == "s" && e.currentBuffer != nil {
			count := e.commandCount
			if count == 0 {
				count = 1 // Default to 1 if no count specified
			}

			line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
			if e.currentBuffer.cursorCol < len(line) {
				// Save the current state for undo before changing anything
				e.UndoSaveRegion(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow)

				// Determine how many characters we can actually delete
				charsToDelete := count
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

				// Set up tracking for s command before entering insert mode
				e.sCommandActive = true
				e.sCommandCount = count
				e.sCommandStartCol = e.currentBuffer.cursorCol

				// Enter insert mode
				e.mode = ModeInsert
			} else if len(line) == 0 {
				// If the line is empty, set up tracking and enter insert mode
				e.sCommandActive = true
				e.sCommandCount = count
				e.sCommandStartCol = e.currentBuffer.cursorCol
				e.mode = ModeInsert
			}

			// Reset count after execution
			e.commandCount = 0
			e.buildingCount = false
			return
		}

		// Handle line deletion with dd
		if s == "d" {
			e.awaitingMotion = true
			e.currentCommand = "d"
			return
		}

		// Handle line yank with yy
		if s == "y" {
			e.awaitingMotion = true
			e.currentCommand = "y"
			return
		}

		// Handle change command
		if s == "c" {
			e.awaitingMotion = true
			e.currentCommand = "c"
			return
		}

		// Handle indent command
		if s == ">" && !e.awaitingMotion {
			e.awaitingMotion = true
			e.currentCommand = ">"
			return
		}

		// Handle dedent command
		if s == "<" && !e.awaitingMotion {
			e.awaitingMotion = true
			e.currentCommand = "<"
			return
		}

		// Handle new line commands
// Handle paste with p
		if s == "p" && e.currentBuffer != nil && e.yankRegister != "" {
			// Paste after cursor position
			line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

			// Handle differently based on the type of yanked content
			if e.yankRegisterType == 1 || (e.yankRegisterType == 0 && strings.Contains(e.yankRegister, "\n")) {
				// Line-wise paste - insert after current line
				
				// Split the content by newlines and filter out any empty trailing element
				parts := strings.Split(e.yankRegister, "\n")
				
				// The last element will be empty if the string ends with a newline
				// which it does with line-wise yanks
				lines := make([]string, 0, len(parts))
				for _, part := range parts {
					if part != "" || len(lines) == 0 {
						lines = append(lines, part)
					}
				}
				
				// Insert the lines after current line
				e.currentBuffer.SetLines(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow, lines)
				
				// Move cursor to the first non-whitespace character of the first pasted line
				e.currentBuffer.cursorRow++
				e.currentBuffer.cursorCol = 0
				
				// If no lines were actually added, don't adjust the cursor
				if len(lines) > 0 {
					// Move to first non-blank character in the line
					line = e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
					for i := 0; i < len(line); i++ {
						if line[i] != ' ' && line[i] != '\t' {
							e.currentBuffer.cursorCol = i
							break
						}
					}
				}
			} else {
				// Character paste - insert after current position
				newLine := ""
				if e.currentBuffer.cursorCol < len(line) {
					newLine = line[:e.currentBuffer.cursorCol+1] + e.yankRegister + line[e.currentBuffer.cursorCol+1:]
				} else {
					newLine = line + e.yankRegister
				}
				e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{newLine})
				e.currentBuffer.cursorCol += len(e.yankRegister)
			}
			return
		}

		// Handle new line commands
		if s == "o" && e.currentBuffer != nil {
			// Set up tracking for o command
			e.insertCommandActive = true
			e.insertCommandType = "o"
			e.insertCommandCount = e.commandCount
			if e.insertCommandCount == 0 {
				e.insertCommandCount = 1
			}
			e.insertCommandPos = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
			
			// Start a new insert undo group BEFORE adding the new line
			// This ensures we capture the buffer state before modification
			e.startInsertUndoGroup("o") // Start a new insert undo group for 'o' command

			// Open a new line below current line and enter insert mode
			e.currentBuffer.SetLines(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow, []string{""})
			e.currentBuffer.cursorRow++
			e.currentBuffer.cursorCol = 0
			e.mode = ModeInsert
			
			// Reset count after execution
			e.commandCount = 0
			e.buildingCount = false
			return
		}

		if s == "O" && e.currentBuffer != nil {
			// Set up tracking for O command
			e.insertCommandActive = true
			e.insertCommandType = "O"
			e.insertCommandCount = e.commandCount
			if e.insertCommandCount == 0 {
				e.insertCommandCount = 1
			}
			e.insertCommandPos = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
			
			// Start a new insert undo group BEFORE adding the new line
			// This ensures we capture the buffer state before modification
			e.startInsertUndoGroup("O") // Start a new insert undo group for 'O' command

			// Open a new line above current line and enter insert mode
			e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow-1, []string{""})
			e.currentBuffer.cursorCol = 0
			e.mode = ModeInsert
			
			// Reset count after execution
			e.commandCount = 0
			e.buildingCount = false
			return
		}

		// Handle awaiting motion state (for d, y, c, etc.)
		if e.awaitingMotion {
			// We need to remember the original position for some operations
			// (variables are used lower in the function)

			// First, check if we're in a command+motion sequence
			// We need to check for specific double-letter commands before checking for general motions
			if e.currentCommand == "d" && s == "d" ||
				e.currentCommand == "y" && s == "y" ||
				e.currentCommand == "c" && s == "c" ||
				e.currentCommand == ">" && s == ">" ||
				e.currentCommand == "<" && s == "<" {
				// Double-letter commands (dd, yy, cc)

				// Get line count
				lineCount := e.currentBuffer.GetLineCount()

				// Get the target line
				line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

				// Store in register for any command
				e.yankRegister = line + "\n"

				if e.currentCommand == "d" || e.currentCommand == "c" {
					// For delete or change, modify buffer
					if lineCount > 1 {
						// If this is the last line, move cursor up
						if e.currentBuffer.cursorRow == lineCount {
							e.currentBuffer.cursorRow--
						}

						// Remove the line for delete
						if e.currentCommand == "d" {
							newLines := make([]string, 0)
							for i := 1; i <= lineCount; i++ {
								if i != e.currentBuffer.cursorRow {
									newLines = append(newLines, e.currentBuffer.GetLine(i))
								}
							}

							e.currentBuffer.SetLines(0, lineCount, newLines)

							// Adjust cursor position
							if e.currentBuffer.cursorRow > len(newLines) {
								e.currentBuffer.cursorRow = len(newLines)
							}

							// Move to first non-blank character
							line = e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
							e.currentBuffer.cursorCol = 0
							for i := 0; i < len(line); i++ {
								if line[i] != ' ' && line[i] != '\t' {
									e.currentBuffer.cursorCol = i
									break
								}
							}
						} else if e.currentCommand == "c" {
							// For change, replace with empty line and enter insert mode
							e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{""})
							e.currentBuffer.cursorCol = 0
							e.mode = ModeInsert
						}
					} else {
						// Handling the only line
						if e.currentCommand == "d" {
							e.currentBuffer.SetLines(0, 1, []string{""})
							e.currentBuffer.cursorCol = 0
						} else if e.currentCommand == "c" {
							e.currentBuffer.SetLines(0, 1, []string{""})
							e.currentBuffer.cursorCol = 0
							e.mode = ModeInsert
						}
					}
				} else if e.currentCommand == ">" || e.currentCommand == "<" {
					// Handle indentation commands (>> and <<)
					// Get the count (default to 1 if no count specified)
					count := e.commandCount
					if count == 0 {
						count = 1
					}

					// Save for undo
					e.UndoSaveRegion(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow+count-1)

					// Apply indentation/dedentation to count lines starting from current line
					for i := 0; i < count && (e.currentBuffer.cursorRow+i) <= e.currentBuffer.GetLineCount(); i++ {
						lineNum := e.currentBuffer.cursorRow + i
						line := e.currentBuffer.GetLine(lineNum)
						
						var newLine string
						if e.currentCommand == ">" {
							// Indent the line
							newLine = e.indentLine(line)
						} else {
							// Dedent the line
							newLine = e.dedentLine(line)
						}
						
						// Update the line in the buffer
						e.currentBuffer.SetLines(lineNum-1, lineNum, []string{newLine})
					}

					// Record for dot command
					e.recordEditCommand(e.currentCommand + e.currentCommand, count) // ">>" or "<<"

					// Move cursor to first non-blank character of the current line
					line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
					e.currentBuffer.cursorCol = 0
					for i := 0; i < len(line); i++ {
						if line[i] != ' ' && line[i] != '\t' {
							e.currentBuffer.cursorCol = i
							break
						}
					}
				}

				// Reset awaiting motion state
				e.awaitingMotion = false
				e.currentCommand = ""
				e.commandCount = 0
				e.buildingCount = false
				return
			}

			// Check if this is a motion command in our motion handlers map
			// This is the key part that handles dw, cw, d$, etc.
			if handler, exists := motionHandlers[s]; exists {
				// For w, e, b, $, 0, etc.
				// First, remember current position
				startRow := e.currentBuffer.cursorRow
				startCol := e.currentBuffer.cursorCol
				startLine := e.currentBuffer.GetLine(startRow)

				// Special handling for cw - in Vim, "cw" behaves like "ce"
				// It changes to the end of the current word, not to the beginning of the next
				if e.currentCommand == "c" && s == "w" {
					// For cw, we want to move to the end of the current word
					// So we use moveWordEnd instead of moveWordForward
					if endHandler, exists := motionHandlers["e"]; exists {
						endHandler(e, 1)
					} else {
						// Fallback to the normal handler
						handler(e, 1)
					}
				} else {
					// Regular motion
					handler(e, 1)
				}

				// Get the ending position
				endRow := e.currentBuffer.cursorRow
				endCol := e.currentBuffer.cursorCol

				// For same-line operations
				if startRow == endRow {
					line := startLine

					// Ensure we don't go past end of line
					if endCol > len(line) {
						endCol = len(line)
					}

					// Make sure startCol is valid
					if startCol > len(line) {
						startCol = len(line)
					}

					// Ensure start is before end
					if startCol <= endCol {
						// Get the text to operate on
						textToYank := line[startCol:endCol]
						e.yankRegister = textToYank

						// For delete or change commands
						if e.currentCommand == "d" || e.currentCommand == "c" {
							// Create new line
							newLine := line[:startCol] + line[endCol:]
							e.currentBuffer.SetLines(startRow-1, startRow, []string{newLine})

							// Reset cursor to start position
							e.currentBuffer.cursorRow = startRow
							e.currentBuffer.cursorCol = startCol

							// For change, enter insert mode
							if e.currentCommand == "c" {
								e.mode = ModeInsert
							}
						} else if e.currentCommand == "y" {
							// For yank, just reset cursor
							e.currentBuffer.cursorRow = startRow
							e.currentBuffer.cursorCol = startCol
						}
					}
				} else {
					// Multi-line operation (simplified)
					firstLine := e.currentBuffer.GetLine(startRow)
					lastLine := e.currentBuffer.GetLine(endRow)

					// Get text to operate on (simplified)
					yankedText := ""
					if startCol < len(firstLine) {
						yankedText = firstLine[startCol:]
					}
					yankedText += "\n" // Add newline between lines
					if endCol < len(lastLine) {
						yankedText += lastLine[:endCol]
					}
					e.yankRegister = yankedText

					// For delete/change operations
					if e.currentCommand == "d" || e.currentCommand == "c" {
						// Create joined line
						newLine := ""
						if startCol < len(firstLine) {
							newLine = firstLine[:startCol]
						}
						if endCol < len(lastLine) {
							newLine += lastLine[endCol:]
						}

						// Replace the lines
						e.currentBuffer.SetLines(startRow-1, endRow, []string{newLine})

						// Reset cursor
						e.currentBuffer.cursorRow = startRow
						e.currentBuffer.cursorCol = startCol

						// Enter insert mode for change
						if e.currentCommand == "c" {
							e.mode = ModeInsert
						}
					} else if e.currentCommand == "y" {
						// Just reset cursor for yank
						e.currentBuffer.cursorRow = startRow
						e.currentBuffer.cursorCol = startCol
					}
				}
			} else {
				// Special cases for specific motions
				switch s {
				case "$":
					// To end of line
					line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
					if e.currentBuffer.cursorCol < len(line) {
						// Extract text from cursor to end of line
						textToYank := line[e.currentBuffer.cursorCol:]
						e.yankRegister = textToYank

						if e.currentCommand == "d" || e.currentCommand == "c" {
							// Delete to end of line
							newLine := line[:e.currentBuffer.cursorCol]
							e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{newLine})

							// For change, enter insert mode
							if e.currentCommand == "c" {
								e.mode = ModeInsert
							}
						}
					}
				}
			}

			// Reset awaiting motion state
			e.awaitingMotion = false
			e.currentCommand = ""
			return
		}
	}

	// Handle insert mode
	if e.mode == ModeInsert && e.currentBuffer != nil {
		// Handle newline in insert mode
		if len(s) == 1 && (s[0] == '\r' || s[0] == '\n') {
			// The equivalent of pressing Enter in insert mode
			line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

			// Extract leading whitespace (indentation)
			indentation := ""
			for i := 0; i < len(line); i++ {
				if line[i] == ' ' || line[i] == '\t' {
					indentation += string(line[i])
				} else {
					break
				}
			}

			// Handle additional indentation for lines ending with opening braces
			if e.currentBuffer.cursorCol > 0 && e.currentBuffer.cursorCol <= len(line) {
				// Check if there's an opening brace right before the cursor
				// or if the line ends with an opening brace
				checkPos := e.currentBuffer.cursorCol - 1
				if checkPos >= 0 && checkPos < len(line) {
					if line[checkPos] == '{' {
						// Add one level of indentation (4 spaces or a tab based on preference)
						// Here we'll use 4 spaces for simplicity
						indentation += "    " // Four spaces for one level of indentation
					}
				}
			}

			// Split the line at cursor position
			left := line[:e.currentBuffer.cursorCol]
			right := ""
			if e.currentBuffer.cursorCol < len(line) {
				right = line[e.currentBuffer.cursorCol:]
			}

			// Update current line and add a new indented line
			e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{left})
			e.currentBuffer.SetLines(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow, []string{indentation + right})

			// Move cursor to beginning of content on new line (after indentation)
			e.currentBuffer.cursorRow++
			e.currentBuffer.cursorCol = len(indentation)
			return
		}

		// Basic character insertion for insert mode
		if len(s) == 1 && s[0] >= 32 && s[0] <= 126 { // printable ASCII
			line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

			// Insert the character at cursor position
			if e.currentBuffer.cursorCol <= len(line) {
				newLine := line[:e.currentBuffer.cursorCol] + s + line[e.currentBuffer.cursorCol:]
				e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{newLine})
				e.currentBuffer.cursorCol++
			}
		}
		return
	}
}

// Helper methods for verb+motion commands

// deleteWord deletes from cursor to next word
func (e *GoEngine) deleteWord() {
	if e.currentBuffer == nil {
		return
	}

	// Remember current position
	startRow := e.currentBuffer.cursorRow
	startCol := e.currentBuffer.cursorCol

	// Find the destination by using moveWordForward
	moveWordForward(e, 1)

	// Get destination position
	endRow := e.currentBuffer.cursorRow
	endCol := e.currentBuffer.cursorCol

	// Perform deletion
	if startRow == endRow {
		// Same line deletion
		line := e.currentBuffer.GetLine(startRow)

		// Ensure valid indices
		if startCol > len(line) {
			startCol = len(line)
		}
		if endCol > len(line) {
			endCol = len(line)
		}

		// Get the text to yank (store in register)
		e.yankRegister = line[startCol:endCol]

		// Create new line with deletion
		newLine := line[:startCol] + line[endCol:]
		e.currentBuffer.SetLines(startRow-1, startRow, []string{newLine})

		// Reset cursor to start position
		e.currentBuffer.cursorRow = startRow
		e.currentBuffer.cursorCol = startCol
	} else {
		// Multi-line deletion
		startLine := e.currentBuffer.GetLine(startRow)
		endLine := e.currentBuffer.GetLine(endRow)

		// Ensure valid indices
		if startCol > len(startLine) {
			startCol = len(startLine)
		}
		if endCol > len(endLine) {
			endCol = len(endLine)
		}

		// Get text to yank
		e.yankRegister = startLine[startCol:] + "\n" + endLine[:endCol]

		// Create new joined line
		newLine := startLine[:startCol] + endLine[endCol:]

		// Replace the lines
		e.currentBuffer.SetLines(startRow-1, endRow, []string{newLine})

		// Reset cursor to start position
		e.currentBuffer.cursorRow = startRow
		e.currentBuffer.cursorCol = startCol
	}
}

// deleteToEndOfLine deletes from cursor to end of line
func (e *GoEngine) deleteToEndOfLine() {
	if e.currentBuffer == nil {
		return
	}

	line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

	// Ensure valid cursor position
	if e.currentBuffer.cursorCol > len(line) {
		e.currentBuffer.cursorCol = len(line)
	}

	// Get text to yank
	e.yankRegister = line[e.currentBuffer.cursorCol:]

	// Create new line with deletion
	newLine := line[:e.currentBuffer.cursorCol]
	e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{newLine})
}

// deleteToStartOfLine deletes from cursor to start of line
func (e *GoEngine) deleteToStartOfLine() {
	if e.currentBuffer == nil {
		return
	}

	line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

	// Ensure valid cursor position
	if e.currentBuffer.cursorCol > len(line) {
		e.currentBuffer.cursorCol = len(line)
	}

	// Get text to yank
	e.yankRegister = line[:e.currentBuffer.cursorCol]

	// Create new line with deletion
	newLine := line[e.currentBuffer.cursorCol:]
	e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{newLine})

	// Reset cursor to start of line
	e.currentBuffer.cursorCol = 0
}

// deleteBackwardWord deletes from cursor to start of word
func (e *GoEngine) deleteBackwardWord() {
	if e.currentBuffer == nil {
		return
	}

	// Remember current position
	startRow := e.currentBuffer.cursorRow
	startCol := e.currentBuffer.cursorCol

	// Find the destination by using moveWordBackward
	moveWordBackward(e, 1)

	// Get destination position
	endRow := e.currentBuffer.cursorRow
	endCol := e.currentBuffer.cursorCol

	// Ensure start is after end for deletion
	if startRow < endRow || (startRow == endRow && startCol < endCol) {
		// Swap positions for proper deletion
		startRow, endRow = endRow, startRow
		startCol, endCol = endCol, startCol
	}

	// Perform deletion
	if startRow == endRow {
		// Same line deletion
		line := e.currentBuffer.GetLine(startRow)

		// Ensure valid indices
		if endCol > len(line) {
			endCol = len(line)
		}
		if startCol > len(line) {
			startCol = len(line)
		}

		// Get the text to yank (store in register)
		e.yankRegister = line[endCol:startCol]

		// Create new line with deletion
		newLine := line[:endCol] + line[startCol:]
		e.currentBuffer.SetLines(startRow-1, startRow, []string{newLine})

		// Reset cursor to end position
		e.currentBuffer.cursorRow = endRow
		e.currentBuffer.cursorCol = endCol
	} else {
		// Multi-line deletion
		startLine := e.currentBuffer.GetLine(startRow)
		endLine := e.currentBuffer.GetLine(endRow)

		// Ensure valid indices
		if endCol > len(endLine) {
			endCol = len(endLine)
		}
		if startCol > len(startLine) {
			startCol = len(startLine)
		}

		// Get text to yank
		e.yankRegister = endLine[:endCol] + "\n" + startLine[:startCol]

		// Create new joined line
		newLine := endLine[:endCol] + startLine[startCol:]

		// Replace the lines
		e.currentBuffer.SetLines(endRow-1, startRow, []string{newLine})

		// Reset cursor to end position
		e.currentBuffer.cursorRow = endRow
		e.currentBuffer.cursorCol = endCol
	}
}

// deleteToWordEnd deletes from cursor to end of word
func (e *GoEngine) deleteToWordEnd() {
	if e.currentBuffer == nil {
		return
	}

	// Remember current position
	startRow := e.currentBuffer.cursorRow
	startCol := e.currentBuffer.cursorCol

	// Find the destination by using moveWordEnd
	moveWordEnd(e, 1)

	// Get destination position - note the +1 to include the character at the end
	endRow := e.currentBuffer.cursorRow
	endCol := e.currentBuffer.cursorCol + 1

	// Perform deletion
	if startRow == endRow {
		// Same line deletion
		line := e.currentBuffer.GetLine(startRow)

		// Ensure valid indices
		if startCol > len(line) {
			startCol = len(line)
		}
		if endCol > len(line) {
			endCol = len(line)
		}

		// Get the text to yank (store in register)
		e.yankRegister = line[startCol:endCol]

		// Create new line with deletion
		newLine := line[:startCol] + line[endCol:]
		e.currentBuffer.SetLines(startRow-1, startRow, []string{newLine})

		// Reset cursor to start position
		e.currentBuffer.cursorRow = startRow
		e.currentBuffer.cursorCol = startCol
	} else {
		// Multi-line deletion
		startLine := e.currentBuffer.GetLine(startRow)
		endLine := e.currentBuffer.GetLine(endRow)

		// Ensure valid indices
		if startCol > len(startLine) {
			startCol = len(startLine)
		}
		if endCol > len(endLine) {
			endCol = len(endLine)
		}

		// Get text to yank
		e.yankRegister = startLine[startCol:] + "\n" + endLine[:endCol]

		// Create new joined line
		newLine := startLine[:startCol] + endLine[endCol:]

		// Replace the lines
		e.currentBuffer.SetLines(startRow-1, endRow, []string{newLine})

		// Reset cursor to start position
		e.currentBuffer.cursorRow = startRow
		e.currentBuffer.cursorCol = startCol
	}
}

// deleteLines deletes the specified number of lines
func (e *GoEngine) deleteLines(count int) {
	if e.currentBuffer == nil {
		return
	}

	lineCount := e.currentBuffer.GetLineCount()
	startRow := e.currentBuffer.cursorRow
	endRow := startRow + count - 1

	// Ensure we don't go past end of buffer
	if endRow > lineCount {
		endRow = lineCount
	}

	// Yank the lines
	yankedText := ""
	for i := startRow; i <= endRow; i++ {
		yankedText += e.currentBuffer.GetLine(i) + "\n"
	}
	e.yankRegister = yankedText

	// Delete the lines
	var newLines []string
	for i := 1; i < startRow; i++ {
		newLines = append(newLines, e.currentBuffer.GetLine(i))
	}
	for i := endRow + 1; i <= lineCount; i++ {
		newLines = append(newLines, e.currentBuffer.GetLine(i))
	}

	// If we deleted all lines, leave an empty line
	if len(newLines) == 0 {
		newLines = []string{""}
	}

	// Replace buffer content
	e.currentBuffer.SetLines(0, lineCount, newLines)

	// Adjust cursor position
	if startRow > len(newLines) {
		e.currentBuffer.cursorRow = len(newLines)
	} else {
		e.currentBuffer.cursorRow = startRow
	}

	// Move to first non-whitespace character
	line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
	e.currentBuffer.cursorCol = 0
	for i := 0; i < len(line); i++ {
		if line[i] != ' ' && line[i] != '\t' {
			e.currentBuffer.cursorCol = i
			break
		}
	}
}

// changeWord deletes from cursor to next word and enters insert mode
// Note: Unlike deleteWord, changeWord only deletes word characters, not trailing whitespace
func (e *GoEngine) changeWord(count int) {
	// Set up change command tracking before deletion
	e.changeCommandActive = true
	e.changeCommandType = "cw"
	e.changeCommandCount = count
	e.changeCommandPos = [2]int{e.currentBuffer.cursorRow, e.currentBuffer.cursorCol}
	
	// Use a special change-specific word deletion that preserves whitespace
	e.changeWordDelete()
	e.mode = ModeInsert
}

// changeWordDelete deletes word characters only, preserving trailing whitespace
// This is used by change commands (cw) which behave differently from delete commands (dw)
func (e *GoEngine) changeWordDelete() {
	if e.currentBuffer == nil {
		return
	}

	line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
	if e.currentBuffer.cursorCol >= len(line) {
		return // Nothing to delete
	}

	startCol := e.currentBuffer.cursorCol
	endCol := startCol

	// Find the end of the current word (word characters only)
	for endCol < len(line) && isWordChar(line[endCol]) {
		endCol++
	}

	// If we didn't move (not on a word character), delete the non-word characters until next word
	if endCol == startCol {
		for endCol < len(line) && !isWordChar(line[endCol]) {
			endCol++
		}
	}

	// Store the deleted text in yank register
	if endCol > startCol {
		e.yankRegister = line[startCol:endCol]
	}

	// Create new line with the word deleted
	newLine := line[:startCol] + line[endCol:]
	e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{newLine})
}

// changeToEndOfLine deletes to end of line and enters insert mode
func (e *GoEngine) changeToEndOfLine() {
	e.deleteToEndOfLine()
	e.mode = ModeInsert
}

// changeToStartOfLine deletes to start of line and enters insert mode
func (e *GoEngine) changeToStartOfLine() {
	e.deleteToStartOfLine()
	e.mode = ModeInsert
}

// changeBackwardWord deletes to start of previous word and enters insert mode
func (e *GoEngine) changeBackwardWord() {
	e.deleteBackwardWord()
	e.mode = ModeInsert
}

// changeToWordEnd deletes to end of word and enters insert mode
func (e *GoEngine) changeToWordEnd() {
	e.deleteToWordEnd()
	e.mode = ModeInsert
}

// yankWord yanks from cursor to next word
func (e *GoEngine) yankWord() {
	if e.currentBuffer == nil {
		return
	}

	// Remember current position
	startRow := e.currentBuffer.cursorRow
	startCol := e.currentBuffer.cursorCol

	// Find the destination by using moveWordForward
	moveWordForward(e, 1)

	// Get destination position
	endRow := e.currentBuffer.cursorRow
	endCol := e.currentBuffer.cursorCol

	// Perform yanking
	if startRow == endRow {
		// Same line yanking
		line := e.currentBuffer.GetLine(startRow)

		// Ensure valid indices
		if startCol > len(line) {
			startCol = len(line)
		}
		if endCol > len(line) {
			endCol = len(line)
		}

		// Get the text to yank (store in register)
		e.yankRegister = line[startCol:endCol]
	} else {
		// Multi-line yanking
		startLine := e.currentBuffer.GetLine(startRow)
		endLine := e.currentBuffer.GetLine(endRow)

		// Ensure valid indices
		if startCol > len(startLine) {
			startCol = len(startLine)
		}
		if endCol > len(endLine) {
			endCol = len(endLine)
		}

		// Get text to yank
		e.yankRegister = startLine[startCol:] + "\n" + endLine[:endCol]
	}

	// Reset cursor to start position
	e.currentBuffer.cursorRow = startRow
	e.currentBuffer.cursorCol = startCol
}

// yankToEndOfLine yanks from cursor to end of line
func (e *GoEngine) yankToEndOfLine() {
	if e.currentBuffer == nil {
		return
	}

	line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

	// Ensure valid cursor position
	if e.currentBuffer.cursorCol > len(line) {
		e.currentBuffer.cursorCol = len(line)
	}

	// Get text to yank
	e.yankRegister = line[e.currentBuffer.cursorCol:]
}

// yankToStartOfLine yanks from cursor to start of line
func (e *GoEngine) yankToStartOfLine() {
	if e.currentBuffer == nil {
		return
	}

	line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

	// Ensure valid cursor position
	if e.currentBuffer.cursorCol > len(line) {
		e.currentBuffer.cursorCol = len(line)
	}

	// Get text to yank
	e.yankRegister = line[:e.currentBuffer.cursorCol]
}

// yankBackwardWord yanks from cursor to start of word
func (e *GoEngine) yankBackwardWord() {
	if e.currentBuffer == nil {
		return
	}

	// Remember current position
	startRow := e.currentBuffer.cursorRow
	startCol := e.currentBuffer.cursorCol

	// Find the destination by using moveWordBackward
	moveWordBackward(e, 1)

	// Get destination position
	endRow := e.currentBuffer.cursorRow
	endCol := e.currentBuffer.cursorCol

	// Ensure start is after end for yanking
	if startRow < endRow || (startRow == endRow && startCol < endCol) {
		// Swap positions for proper yanking
		startRow, endRow = endRow, startRow
		startCol, endCol = endCol, startCol
	}

	// Perform yanking
	if startRow == endRow {
		// Same line yanking
		line := e.currentBuffer.GetLine(startRow)

		// Ensure valid indices
		if endCol > len(line) {
			endCol = len(line)
		}
		if startCol > len(line) {
			startCol = len(line)
		}

		// Get the text to yank (store in register)
		e.yankRegister = line[endCol:startCol]
	} else {
		// Multi-line yanking
		startLine := e.currentBuffer.GetLine(startRow)
		endLine := e.currentBuffer.GetLine(endRow)

		// Ensure valid indices
		if endCol > len(endLine) {
			endCol = len(endLine)
		}
		if startCol > len(startLine) {
			startCol = len(startLine)
		}

		// Get text to yank
		e.yankRegister = endLine[:endCol] + "\n" + startLine[:startCol]
	}

	// Reset cursor to original position
	e.currentBuffer.cursorRow = startRow
	e.currentBuffer.cursorCol = startCol
}

// yankToWordEnd yanks from cursor to end of word
func (e *GoEngine) yankToWordEnd() {
	if e.currentBuffer == nil {
		return
	}

	// Remember current position
	startRow := e.currentBuffer.cursorRow
	startCol := e.currentBuffer.cursorCol

	// Find the destination by using moveWordEnd
	moveWordEnd(e, 1)

	// Get destination position - note the +1 to include the character at the end
	endRow := e.currentBuffer.cursorRow
	endCol := e.currentBuffer.cursorCol + 1

	// Perform yanking
	if startRow == endRow {
		// Same line yanking
		line := e.currentBuffer.GetLine(startRow)

		// Ensure valid indices
		if startCol > len(line) {
			startCol = len(line)
		}
		if endCol > len(line) {
			endCol = len(line)
		}

		// Get the text to yank (store in register)
		e.yankRegister = line[startCol:endCol]
	} else {
		// Multi-line yanking
		startLine := e.currentBuffer.GetLine(startRow)
		endLine := e.currentBuffer.GetLine(endRow)

		// Ensure valid indices
		if startCol > len(startLine) {
			startCol = len(startLine)
		}
		if endCol > len(endLine) {
			endCol = len(endLine)
		}

		// Get text to yank
		e.yankRegister = startLine[startCol:] + "\n" + endLine[:endCol]
	}

	// Reset cursor to original position
	e.currentBuffer.cursorRow = startRow
	e.currentBuffer.cursorCol = startCol
}

// getNormalizedVisualSelection returns selection bounds in order
func (e *GoEngine) getNormalizedVisualSelection() (startRow, startCol, endRow, endCol int) {
	startRow = e.visualStart[0]
	startCol = e.visualStart[1]
	endRow = e.visualEnd[0]
	endCol = e.visualEnd[1]

	// Ensure start is before end (for consistent operations)
	if startRow > endRow || (startRow == endRow && startCol > endCol) {
		startRow, endRow = endRow, startRow
		startCol, endCol = endCol, startCol
	}

	// For line-wise visual mode (type 1), select entire lines
	if e.visualType == 1 {
		// Select from beginning to end of lines
		startCol = 0
		
		// Go to the end of the last line
		if endRow <= e.currentBuffer.GetLineCount() {
			line := e.currentBuffer.GetLine(endRow)
			endCol = len(line)
		}
		
		return startRow, startCol, endRow, endCol
	}

	// For character-wise visual mode (type 0)
	// Add 1 to endCol to make the selection inclusive
	// This matches Vim behavior where the character under the cursor is included
	if endRow <= e.currentBuffer.GetLineCount() {
		line := e.currentBuffer.GetLine(endRow)
		if endCol < len(line) {
			endCol++
		}
	}

	return startRow, startCol, endRow, endCol
}

// yankVisualSelection yanks the current visual selection
func (e *GoEngine) yankVisualSelection() {
	if e.currentBuffer == nil {
		return
	}

	// Get normalized selection bounds (start before end)
	startRow, startCol, endRow, endCol := e.getNormalizedVisualSelection()

	// Handle differently based on visual mode type
	if e.visualType == 1 {
		// Line-wise visual mode: yank entire lines with newlines
		var content strings.Builder

		// Include all lines from startRow to endRow
		for row := startRow; row <= endRow; row++ {
			line := e.currentBuffer.GetLine(row)
			content.WriteString(line)
			content.WriteString("\n") // Add newline after each line
		}

		// Store yanked content and mark it as line-wise
		e.yankRegister = content.String()
		e.yankRegisterType = 1 // 1 for line-wise yank
	} else {
		// Character-wise visual mode
		if startRow == endRow {
			// Single line selection
			line := e.currentBuffer.GetLine(startRow)
			if startCol < len(line) && endCol <= len(line) {
				e.yankRegister = line[startCol:endCol]
			}
		} else {
			// Multi-line selection
			var content strings.Builder

			// First line (partial)
			line := e.currentBuffer.GetLine(startRow)
			if startCol < len(line) {
				content.WriteString(line[startCol:])
				content.WriteString("\n")
			}

			// Middle lines (complete)
			for row := startRow + 1; row < endRow; row++ {
				line := e.currentBuffer.GetLine(row)
				content.WriteString(line)
				content.WriteString("\n")
			}

			// Last line (partial)
			if endRow <= e.currentBuffer.GetLineCount() {
				line = e.currentBuffer.GetLine(endRow)
				if endCol <= len(line) {
					content.WriteString(line[:endCol])
				}
			}

			e.yankRegister = content.String()
		}
	}

	// Reset cursor to the start of the selection
	e.currentBuffer.cursorRow = startRow
	e.currentBuffer.cursorCol = startCol
}

// deleteVisualSelection deletes the current visual selection
func (e *GoEngine) deleteVisualSelection() {
	if e.currentBuffer == nil {
		return
	}

	// Save for undo
	startRow, startCol, endRow, endCol := e.getNormalizedVisualSelection()
	e.UndoSaveRegion(startRow, endRow)

	// Yank before deleting
	e.yankVisualSelection()

	// Handle differently based on visual mode type
	if e.visualType == 1 {
		// Line-wise visual mode: delete entire lines
		
		// Remove all lines from startRow to endRow
		// If we're removing all lines, leave an empty line
		if startRow == 1 && endRow == e.currentBuffer.GetLineCount() {
			// Deleting all lines, leave one empty line
			e.currentBuffer.SetLines(0, endRow, []string{""})
			e.currentBuffer.cursorRow = 1
			e.currentBuffer.cursorCol = 0
		} else {
			// Create a new set of lines without the selected range
			lineCount := e.currentBuffer.GetLineCount()
			newLines := make([]string, 0, lineCount-(endRow-startRow+1))
			
			// Add lines before selection
			for i := 1; i < startRow; i++ {
				newLines = append(newLines, e.currentBuffer.GetLine(i))
			}
			
			// Add lines after selection
			for i := endRow + 1; i <= lineCount; i++ {
				newLines = append(newLines, e.currentBuffer.GetLine(i))
			}
			
			// If we removed all lines, ensure at least an empty line remains
			if len(newLines) == 0 {
				newLines = []string{""}
			}
			
			// Replace buffer content
			e.currentBuffer.SetLines(0, lineCount, newLines)
			
			// Move cursor to line after the deletion (or the last line if we deleted at the end)
			if startRow <= len(newLines) {
				e.currentBuffer.cursorRow = startRow
			} else if len(newLines) > 0 {
				e.currentBuffer.cursorRow = len(newLines)
			} else {
				e.currentBuffer.cursorRow = 1
			}
			
			// Move to first non-whitespace character on the line
			line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
			e.currentBuffer.cursorCol = 0
			for i := 0; i < len(line); i++ {
				if line[i] != ' ' && line[i] != '\t' {
					e.currentBuffer.cursorCol = i
					break
				}
			}
		}
	} else {
		// Character-wise visual mode
		if startRow == endRow {
			// Single line selection
			line := e.currentBuffer.GetLine(startRow)
			if startCol < len(line) {
				// Create new line with deletion
				newLine := ""
				if startCol > 0 {
					newLine = line[:startCol]
				}
				if endCol < len(line) {
					newLine += line[endCol:]
				}
				e.currentBuffer.SetLines(startRow-1, startRow, []string{newLine})
			}
			
			// Reset cursor to the start of the selection
			e.currentBuffer.cursorRow = startRow
			e.currentBuffer.cursorCol = startCol
		} else {
			// Multi-line selection
			var newLine string

			// Combine the first line's prefix with the last line's suffix
			firstLine := e.currentBuffer.GetLine(startRow)
			lastLine := e.currentBuffer.GetLine(endRow)

			if startCol < len(firstLine) {
				newLine = firstLine[:startCol]
			}

			if endCol < len(lastLine) {
				newLine += lastLine[endCol:]
			}

			// Replace all the selected lines with the new combined line
			e.currentBuffer.SetLines(startRow-1, endRow, []string{newLine})
			
			// Reset cursor to the start of the selection
			e.currentBuffer.cursorRow = startRow
			e.currentBuffer.cursorCol = startCol
		}
	}
}

// changeCaseVisualSelection changes the case of characters in the visual selection
func (e *GoEngine) changeCaseVisualSelection() {
	if e.currentBuffer == nil {
		return
	}

	// Get normalized selection bounds (start before end)
	startRow, startCol, endRow, endCol := e.getNormalizedVisualSelection()

	// Save region for undo
	e.UndoSaveRegion(startRow, endRow)

	// Helper function to toggle case of a character
	toggleCase := func(char byte) byte {
		if char >= 'a' && char <= 'z' {
			// Convert lowercase to uppercase
			return char - 32
		} else if char >= 'A' && char <= 'Z' {
			// Convert uppercase to lowercase
			return char + 32
		}
		// Non-alphabetic character remains unchanged
		return char
	}

	// Handle based on visual mode type
	if e.visualType == 1 {
		// Line-wise visual mode: change case of entire lines
		for row := startRow; row <= endRow; row++ {
			line := e.currentBuffer.GetLine(row)
			if len(line) > 0 {
				newLine := ""

				// Toggle case for all characters in the line
				for i := 0; i < len(line); i++ {
					newLine += string(toggleCase(line[i]))
				}

				// Update the line
				e.currentBuffer.SetLines(row-1, row, []string{newLine})
			}
		}
	} else {
		// Character-wise visual mode
		if startRow == endRow {
			// Single line selection
			line := e.currentBuffer.GetLine(startRow)

			// Ensure indices are valid
			if startCol >= len(line) {
				startCol = len(line) - 1
				if startCol < 0 {
					startCol = 0
				}
			}
			if endCol > len(line) {
				endCol = len(line)
			}

			// Create new line with case changes
			newLine := ""
			if startCol > 0 {
				newLine = line[:startCol]
			}

			// Toggle case of selected characters
			for i := startCol; i < endCol && i < len(line); i++ {
				newLine += string(toggleCase(line[i]))
			}

			// Add remainder of line
			if endCol < len(line) {
				newLine += line[endCol:]
			}

			// Update the buffer
			e.currentBuffer.SetLines(startRow-1, startRow, []string{newLine})
		} else {
			// Multi-line selection
			// Handle first line
			line := e.currentBuffer.GetLine(startRow)
			if startCol < len(line) {
				newLine := ""
				if startCol > 0 {
					newLine = line[:startCol]
				}

				// Toggle case for remaining characters on first line
				for i := startCol; i < len(line); i++ {
					newLine += string(toggleCase(line[i]))
				}

				// Update the first line
				e.currentBuffer.SetLines(startRow-1, startRow, []string{newLine})
			}

			// Handle middle lines (complete lines)
			for row := startRow + 1; row < endRow; row++ {
				line := e.currentBuffer.GetLine(row)
				if len(line) > 0 {
					newLine := ""

					// Toggle case for all characters
					for i := 0; i < len(line); i++ {
						newLine += string(toggleCase(line[i]))
					}

					// Update the middle line
					e.currentBuffer.SetLines(row-1, row, []string{newLine})
				}
			}

			// Handle last line
			if endRow <= e.currentBuffer.GetLineCount() {
				line := e.currentBuffer.GetLine(endRow)
				if endCol > 0 && endCol <= len(line) {
					newLine := ""

					// Toggle case for selected characters on last line
					for i := 0; i < endCol && i < len(line); i++ {
						newLine += string(toggleCase(line[i]))
					}

					// Add remainder of line
					if endCol < len(line) {
						newLine += line[endCol:]
					}

					// Update the last line
					e.currentBuffer.SetLines(endRow-1, endRow, []string{newLine})
				}
			}
		}
	}

	// Reset cursor to the start of the selection
	e.currentBuffer.cursorRow = startRow
	e.currentBuffer.cursorCol = startCol
}

// indentVisualSelection indents all lines in the visual selection
func (e *GoEngine) indentVisualSelection() {
	if e.currentBuffer == nil {
		return
	}

	// Get normalized selection bounds (start before end)
	startRow, _, endRow, _ := e.getNormalizedVisualSelection()

	// Save region for undo
	e.UndoSaveRegion(startRow, endRow)

	// For line-wise visual mode, indent all lines in the selection
	if e.visualType == 1 {
		lineCount := endRow - startRow + 1
		for row := startRow; row <= endRow; row++ {
			line := e.currentBuffer.GetLine(row)
			newLine := e.indentLine(line)
			e.currentBuffer.SetLines(row-1, row, []string{newLine})
		}
		
		// Record for dot command - visual line indent should be repeatable
		// Use a special command name to distinguish from normal mode >>
		e.recordEditCommand("visual_indent", lineCount)
	}

	// Move cursor to first non-blank character of the first line
	line := e.currentBuffer.GetLine(startRow)
	e.currentBuffer.cursorRow = startRow
	e.currentBuffer.cursorCol = 0
	for i := 0; i < len(line); i++ {
		if line[i] != ' ' && line[i] != '\t' {
			e.currentBuffer.cursorCol = i
			break
		}
	}
}

// dedentVisualSelection dedents all lines in the visual selection
func (e *GoEngine) dedentVisualSelection() {
	if e.currentBuffer == nil {
		return
	}

	// Get normalized selection bounds (start before end)
	startRow, _, endRow, _ := e.getNormalizedVisualSelection()

	// Save region for undo
	e.UndoSaveRegion(startRow, endRow)

	// For line-wise visual mode, dedent all lines in the selection
	if e.visualType == 1 {
		lineCount := endRow - startRow + 1
		for row := startRow; row <= endRow; row++ {
			line := e.currentBuffer.GetLine(row)
			newLine := e.dedentLine(line)
			e.currentBuffer.SetLines(row-1, row, []string{newLine})
		}
		
		// Record for dot command - visual line dedent should be repeatable
		// Use a special command name to distinguish from normal mode <<
		e.recordEditCommand("visual_dedent", lineCount)
	}

	// Move cursor to first non-blank character of the first line
	line := e.currentBuffer.GetLine(startRow)
	e.currentBuffer.cursorRow = startRow
	e.currentBuffer.cursorCol = 0
	for i := 0; i < len(line); i++ {
		if line[i] != ' ' && line[i] != '\t' {
			e.currentBuffer.cursorCol = i
			break
		}
	}
}

// Key processes a key with terminal codes replaced
func (e *GoEngine) Key(s string) {
	// Handle ESC key specially to ensure proper mode switching
	if s == "<esc>" {
		e.Input("\x1b")
		return
	}

	// Handle special key commands in normal mode first
	if e.mode == ModeNormal {
		// Special handling for 'g' commands
		if e.currentCommand == "g" {
			if s == "g" {
				// Execute "gg" command to move to first line
				if handler, exists := motionHandlers["g"]; exists {
					handler(e, 1)
					e.currentCommand = ""
					return
				}
			}

			// Reset g command state if followed by anything else
			e.currentCommand = ""
		}
	}

	// Check if we're awaiting a motion command
	// For verb+motion combinations like "dw", "cw", etc.
	if e.mode == ModeNormal && e.awaitingMotion {
		// Direct handling of common verb+motion combinations
		switch e.currentCommand {
		case "d":
			switch s {
			case "w":
				e.deleteWord()
				e.recordEditCommand("dw", 1)
				e.awaitingMotion = false
				e.currentCommand = ""
				return
			case "$":
				e.deleteToEndOfLine()
				e.recordEditCommand("d$", 1)
				e.awaitingMotion = false
				e.currentCommand = ""
				return
			case "0":
				e.deleteToStartOfLine()
				e.recordEditCommand("d0", 1)
				e.awaitingMotion = false
				e.currentCommand = ""
				return
			case "b":
				e.deleteBackwardWord()
				e.recordEditCommand("db", 1)
				e.awaitingMotion = false
				e.currentCommand = ""
				return
			case "e":
				e.deleteToWordEnd()
				e.recordEditCommand("de", 1)
				e.awaitingMotion = false
				e.currentCommand = ""
				return
			case "d":
				// Special case for "dd" - delete entire line
				e.deleteLines(1)
				e.recordEditCommand("dd", 1)
				e.awaitingMotion = false
				e.currentCommand = ""
				return
			}
		case "c":
			switch s {
			case "w":
				count := e.commandCount
				if count == 0 {
					count = 1
				}
				e.changeWord(count)
				e.awaitingMotion = false
				e.currentCommand = ""
				return
			case "$":
				e.changeToEndOfLine()
				e.awaitingMotion = false
				e.currentCommand = ""
				return
			case "0":
				e.changeToStartOfLine()
				e.awaitingMotion = false
				e.currentCommand = ""
				return
			case "b":
				e.changeBackwardWord()
				e.awaitingMotion = false
				e.currentCommand = ""
				return
			case "e":
				e.changeToWordEnd()
				e.awaitingMotion = false
				e.currentCommand = ""
				return
			case "c":
				// Special case for "cc" - change entire line
				e.deleteLines(1)
				e.mode = ModeInsert
				e.awaitingMotion = false
				e.currentCommand = ""
				return
			}
		case "y":
			switch s {
			case "w":
				e.yankWord()
				e.awaitingMotion = false
				e.currentCommand = ""
				return
			case "$":
				e.yankToEndOfLine()
				e.awaitingMotion = false
				e.currentCommand = ""
				return
			case "0":
				e.yankToStartOfLine()
				e.awaitingMotion = false
				e.currentCommand = ""
				return
			case "b":
				e.yankBackwardWord()
				e.awaitingMotion = false
				e.currentCommand = ""
				return
			case "e":
				e.yankToWordEnd()
				e.awaitingMotion = false
				e.currentCommand = ""
				return
			case "y":
				// Special case for "yy" - yank entire line
				line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
				e.yankRegister = line + "\n"
				e.awaitingMotion = false
				e.currentCommand = ""
				return
			}
		}
	}

	// Handle special state cleanup for normal mode
	if e.mode == ModeNormal && e.currentCommand != "" && !e.awaitingMotion {
		// If we still have a pending command but don't expect a motion,
		// reset the state to avoid leftover command state
		e.currentCommand = ""
	}

	// Handle arrow keys for all modes
	switch s {
	case "<left>":
		if e.currentBuffer != nil {
			if e.currentBuffer.cursorCol > 0 {
				e.currentBuffer.cursorCol--
				e.updateVisualSelection()
			} else if e.currentBuffer.cursorRow > 1 && e.mode == ModeInsert {
				// In insert mode, allow wrapping to previous line
				e.currentBuffer.cursorRow--
				line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)
				e.currentBuffer.cursorCol = len(line)
				e.updateVisualSelection()
			}
		}
		return

	case "<right>":
		if e.currentBuffer != nil {
			line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

			if e.mode == ModeInsert {
				// In insert mode, can position at end of line
				if e.currentBuffer.cursorCol < len(line) {
					e.currentBuffer.cursorCol++
					e.updateVisualSelection()
				}
			} else {
				// In normal mode, can only position on actual characters
				if len(line) > 0 && e.currentBuffer.cursorCol < len(line)-1 {
					e.currentBuffer.cursorCol++
					e.updateVisualSelection()
				}
			}
		}
		return

	case "<up>":
		if e.currentBuffer != nil && e.currentBuffer.cursorRow > 1 {
			// Save column position for vertical movement
			desiredCol := e.currentBuffer.cursorCol

			// Move up one line
			e.currentBuffer.cursorRow--

			// Adjust column based on new line length
			line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

			if e.mode == ModeInsert {
				// In insert mode, can position at end of line
				if desiredCol > len(line) {
					e.currentBuffer.cursorCol = len(line)
				} else {
					e.currentBuffer.cursorCol = desiredCol
				}
			} else {
				// In normal mode, can only position on actual characters
				if len(line) == 0 {
					e.currentBuffer.cursorCol = 0
				} else if desiredCol >= len(line) {
					e.currentBuffer.cursorCol = len(line) - 1
				} else {
					e.currentBuffer.cursorCol = desiredCol
				}
			}

			e.updateVisualSelection()
		}
		return

	case "<down>":
		if e.currentBuffer != nil && e.currentBuffer.cursorRow < e.currentBuffer.GetLineCount() {
			// Save column position for vertical movement
			desiredCol := e.currentBuffer.cursorCol

			// Move down one line
			e.currentBuffer.cursorRow++

			// Adjust column based on new line length
			line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

			if e.mode == ModeInsert {
				// In insert mode, can position at end of line
				if desiredCol > len(line) {
					e.currentBuffer.cursorCol = len(line)
				} else {
					e.currentBuffer.cursorCol = desiredCol
				}
			} else {
				// In normal mode, can only position on actual characters
				if len(line) == 0 {
					e.currentBuffer.cursorCol = 0
				} else if desiredCol >= len(line) {
					e.currentBuffer.cursorCol = len(line) - 1
				} else {
					e.currentBuffer.cursorCol = desiredCol
				}
			}

			e.updateVisualSelection()
		}
		return
	}

	// Handle insert mode specific keys
	if e.mode == ModeInsert {
		switch s {
		case "<cr>", "<enter>", "<return>":
			if e.currentBuffer != nil {
				// In insert mode, add a new line
				line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

				// Split the line at cursor position
				left := line[:e.currentBuffer.cursorCol]
				right := ""
				if e.currentBuffer.cursorCol < len(line) {
					right = line[e.currentBuffer.cursorCol:]
				}

				// Update current line and add a new line
				e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{left})
				e.currentBuffer.SetLines(e.currentBuffer.cursorRow, e.currentBuffer.cursorRow, []string{right})

				// Move cursor to beginning of new line
				e.currentBuffer.cursorRow++
				e.currentBuffer.cursorCol = 0
			}
			return

		case "<bs>", "<backspace>":
			if e.currentBuffer != nil {
				line := e.currentBuffer.GetLine(e.currentBuffer.cursorRow)

				if e.currentBuffer.cursorCol > 0 {
					// Delete one character before cursor
					newLine := line[:e.currentBuffer.cursorCol-1] + line[e.currentBuffer.cursorCol:]
					e.currentBuffer.SetLines(e.currentBuffer.cursorRow-1, e.currentBuffer.cursorRow, []string{newLine})
					e.currentBuffer.cursorCol--
				} else if e.currentBuffer.cursorRow > 1 {
					// At beginning of line, join with previous line
					prevLine := e.currentBuffer.GetLine(e.currentBuffer.cursorRow - 1)
					e.currentBuffer.SetLines(e.currentBuffer.cursorRow-2, e.currentBuffer.cursorRow, []string{prevLine + line})
					e.currentBuffer.cursorRow--
					e.currentBuffer.cursorCol = len(prevLine)
				}
			}
			return
		}
	}

	// For other special keys, pass to input handler
	e.Input(s)
}
