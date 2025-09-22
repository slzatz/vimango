package vim

import (
	"log"

	"github.com/slzatz/vimango/vim/cvim"
	"github.com/slzatz/vimango/vim/govim"
	"github.com/slzatz/vimango/vim/interfaces"
)

// Implementation constants
const (
	ImplC  = "cgo"
	ImplGo = "go"
)

// VimImplementation allows switching between C and Go implementations
type VimImplementation interface {
	GetEngineWrapper() interfaces.VimEngine
	//GetBufferWrapper() interfaces.VimBuffer
	GetName() string
}

// GoImplementation provides the pure Go vim implementation
type GoImplementation struct {
	logger *log.Logger
}

// GetEngineWrapper returns the Go-based engine wrapper
func (g *GoImplementation) GetEngineWrapper() interfaces.VimEngine {
	engine := &GoEngineWrapper{
		debugLog: g.logger,
		engine:   govim.NewEngine(), // This is the Go vim engine from package govim
	}

	if g.logger != nil {
		g.logger.Println("Created GoEngine instance")
	}

	return engine
}

// does nothing to set buf to nil
func (g *GoImplementation) GetBufferWrapper() interfaces.VimBuffer {
	return &GoBufferWrapper{
		buf: &govim.GoBuffer{}, // This is the Go vim buffer from package govim
	}
}

// GetName returns the implementation name
func (g *GoImplementation) GetName() string {
	return ImplGo
}

// GoEngineWrapper wraps the Go-based vim engine in package govim
// GoEngineWrapper satisfies the VimEngine interface
type GoEngineWrapper struct {
	// Add debug logger
	debugLog *log.Logger
	engine   *govim.GoEngine
}

// Init initializes the vim engine
func (e *GoEngineWrapper) Init(argc int) {
	e.engine.Init(argc)
}

// BufferOpen opens a file and returns a buffer
func (e *GoEngineWrapper) BufferOpen(filename string, lnum int, flags int) interfaces.VimBuffer {
	buf := e.engine.BufferOpen(filename, lnum, flags)
	return &GoBufferWrapper{buf: buf}
}

// BufferNew creates a new empty buffer
func (e *GoEngineWrapper) BufferNew(flags int) interfaces.VimBuffer {
	buf := e.engine.BufferNew(flags)
	return &GoBufferWrapper{buf: buf}
}

// BufferGetCurrent gets the current buffer
func (e *GoEngineWrapper) BufferGetCurrent() interfaces.VimBuffer {
	buf := e.engine.BufferGetCurrent()
	return &GoBufferWrapper{buf: buf}
}

// BufferSetCurrent sets the current buffer
func (e *GoEngineWrapper) BufferSetCurrent(buf interfaces.VimBuffer) {
	// really could just do buf.(*GoBufferWrapper).buf
	// govim.engine.BufferSetCurrent is expecting a *GoBuffer
	/*
		if goBufWrap, ok := buf.(*GoBufferWrapper); ok {
			e.engine.BufferSetCurrent(goBufWrap.buf)
		}
	*/
	// doesn't need if statement to check if buf is a GoBufferWrapper
	e.engine.BufferSetCurrent(buf.(*GoBufferWrapper).buf)
}

// CursorGetLine gets the current cursor line
func (e *GoEngineWrapper) CursorGetLine() int {
	return e.engine.CursorGetLine()
}

// CursorGetPosition gets the cursor position
func (e *GoEngineWrapper) CursorGetPosition() [2]int {
	return e.engine.CursorGetPosition()
}

// CursorSetPosition sets the cursor position
func (e *GoEngineWrapper) CursorSetPosition(row, col int) {
	e.engine.CursorSetPosition(row, col)
}

// Input sends input to vim
func (e *GoEngineWrapper) Input(s string) {
	e.engine.Input(s)
}

// Input2 sends multiple character input
func (e *GoEngineWrapper) Input2(s string) {
	for _, x := range s {
		e.engine.Input(string(x))
	}
}

// Key sends special key input
func (e *GoEngineWrapper) Key(s string) {
	e.engine.Key(s)
}

// Execute runs an ex command
func (e *GoEngineWrapper) Execute(s string) {
	e.engine.Execute(s)
}

// GetMode gets the current mode
func (e *GoEngineWrapper) GetMode() int {
	mode := e.engine.GetMode()
	return mode
}

// GetCurrentMode gets the current mode with application-compatible mappings
func (e *GoEngineWrapper) GetCurrentMode() int {
	return e.engine.GetMode()
}

// GetCurrentMode gets the current mode with application-compatible mappings
func (e *GoEngineWrapper) GetSubMode() cvim.SubMode {
	return e.engine.GetSubMode()
}

// VisualGetRange gets the visual selection range
func (e *GoEngineWrapper) VisualGetRange() [2][2]int {
	return e.engine.VisualGetRange()
}

// VisualGetType gets the visual mode type
func (e *GoEngineWrapper) VisualGetType() int {
	return e.engine.VisualGetType()
}

// Eval evaluates a vim expression
func (e *GoEngineWrapper) Eval(expr string) string {
	return e.engine.Eval(expr)
}

// SearchGetMatchingPair finds matching brackets
func (e *GoEngineWrapper) SearchGetMatchingPair() [2]int {
	return e.engine.SearchGetMatchingPair()
}

// GoBufferWrapper wraps an instance of govim.GoBuffer
// GoBufferWrapper satisfies the VimBuffer interface
type GoBufferWrapper struct {
	buf *govim.GoBuffer
}

// GetID gets the buffer ID
func (b *GoBufferWrapper) GetID() int {
	return b.buf.GetID()
}

// GetLine gets a line from the buffer
func (b *GoBufferWrapper) GetLine(lnum int) string {
	// Safety check for lines that don't exist
	if lnum < 1 || lnum > b.buf.GetLineCount() {
		return ""
	}

	// Get the line content safely
	return b.buf.GetLine(lnum)
}

// GetLineB gets a line as bytes from the buffer
func (b *GoBufferWrapper) GetLineB(lnum int) []byte {
	// Use our safer GetLine implementation
	return []byte(b.GetLine(lnum))
}

// GetLineCount gets the number of lines in the buffer
func (b *GoBufferWrapper) GetLineCount() int {
	return b.buf.GetLineCount()
}

// Lines gets all lines from the buffer
func (b *GoBufferWrapper) Lines() []string {
	// Create a completely fresh copy of the lines to prevent unexpected sharing
	lines := b.buf.Lines()
	/*
		result := make([]string, len(lines))
		for i, line := range lines {
			result[i] = line
		}
		return result
	*/
	return lines
}

// LinesB gets all lines as bytes from the buffer
func (b *GoBufferWrapper) LinesB() [][]byte {
	// Create a completely fresh copy of the lines to prevent unexpected sharing
	lines := b.buf.LinesB()
	result := make([][]byte, len(lines))
	for i, line := range lines {
		// Make a copy of each line's bytes
		newLine := make([]byte, len(line))
		copy(newLine, line)
		result[i] = newLine
	}
	return result
}

// SetCurrent sets this buffer as the current buffer
func (b *GoBufferWrapper) SetCurrent() {
	b.buf.SetCurrent()
}

// IsModified checks if the buffer has been modified
func (b *GoBufferWrapper) IsModified() bool {
	return b.buf.IsModified()
}

// GetLastChangedTick gets the last changed tick value
func (b *GoBufferWrapper) GetLastChangedTick() int {
	return b.buf.GetLastChangedTick()
}

// SetLines sets all lines in the buffer
func (b *GoBufferWrapper) SetLines(start, end int, lines []string) {
	// Convert nils to empty arrays to prevent panics
	if lines == nil {
		lines = []string{}
	}

	// Create a deep copy of the lines to avoid any shared references
	safeLines := make([]string, len(lines))
	for i, line := range lines {
		safeLines[i] = line
	}

	// Ensure we have at least one line for complete buffer replacement
	if start == 0 && end == -1 && len(safeLines) == 0 {
		safeLines = []string{""}
	}

	// Update the buffer
	b.buf.SetLines(start, end, safeLines)
}
