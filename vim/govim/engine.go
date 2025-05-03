package govim

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
