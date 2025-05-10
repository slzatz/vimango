package vim

import (
	"fmt"
	"log"
	"os"

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
	GetEngine() interfaces.VimEngine
	GetName() string
}

// ActiveImplementation tracks which implementation is active
var ActiveImplementation = ImplC

// CGOImplementation provides the C-based vim implementation
type CGOImplementation struct{}

// GetEngine returns the C-based engine
func (c *CGOImplementation) GetEngine() interfaces.VimEngine {
	return &CGOEngine{}
}

// GetName returns the implementation name
func (c *CGOImplementation) GetName() string {
	return ImplC
}

// GoImplementation provides the pure Go vim implementation
type GoImplementation struct {
	logger *log.Logger
}

// GetEngine returns the Go-based engine
func (g *GoImplementation) GetEngine() interfaces.VimEngine {
	engine := &GoEngine{
		debugLog: g.logger,
		engine:   govim.NewEngine(),
	}

	if g.logger != nil {
		g.logger.Println("Created GoEngine instance")
	}

	return engine
}

// GetName returns the implementation name
func (g *GoImplementation) GetName() string {
	return ImplGo
}

// activeImpl is the current implementation (C or Go)
var activeImpl VimImplementation = &CGOImplementation{}

// CGOEngine wraps the C-based vim implementation
type CGOEngine struct{}

// Init initializes the vim engine
func (e *CGOEngine) Init(argc int) {
	cvim.VimInit(argc)
}

// BufferOpen opens a file and returns a buffer
func (e *CGOEngine) BufferOpen(filename string, lnum int, flags int) interfaces.VimBuffer {
	buf := cvim.BufferOpen(filename, lnum, flags)
	return &CGOBufferWrapper{buf: buf}
}

// BufferNew creates a new empty buffer
func (e *CGOEngine) BufferNew(flags int) interfaces.VimBuffer {
	buf := cvim.CBufferNew(flags)
	return &CGOBufferWrapper{buf: buf}
}

// BufferGetCurrent gets the current buffer
func (e *CGOEngine) BufferGetCurrent() interfaces.VimBuffer {
	buf := cvim.BufferGetCurrent()
	return &CGOBufferWrapper{buf: buf}
}

// BufferSetCurrent sets the current buffer
func (e *CGOEngine) BufferSetCurrent(buf interfaces.VimBuffer) {
	if cgoBuf, ok := buf.(*CGOBufferWrapper); ok {
		cvim.CBufferSetCurrent(cgoBuf.buf)
	}
}

// CursorGetLine gets the current cursor line
func (e *CGOEngine) CursorGetLine() int {
	return cvim.CursorGetLine()
}

// CursorGetPosition gets the cursor position
func (e *CGOEngine) CursorGetPosition() [2]int {
	return cvim.CursorGetPosition()
}

// CursorSetPosition sets the cursor position
func (e *CGOEngine) CursorSetPosition(row, col int) {
	cvim.CursorSetPosition(row, col)
}

// Input sends input to vim
func (e *CGOEngine) Input(s string) {
	cvim.Input(s)
}

// Input2 sends multiple character input
func (e *CGOEngine) Input2(s string) {
	cvim.Input2(s)
}

// Key sends special key input
func (e *CGOEngine) Key(s string) {
	cvim.Key(s)
}

// Execute runs an ex command
func (e *CGOEngine) Execute(s string) {
	cvim.Execute(s)
}

// GetMode gets the current mode
func (e *CGOEngine) GetMode() int {
	return cvim.GetMode()
}

// GetCurrentMode gets the current mode with application-compatible mappings
// For CGO implementation, this is the same as GetMode
func (e *CGOEngine) GetCurrentMode() int {
	return cvim.GetMode()
}

// VisualGetRange gets the visual selection range
func (e *CGOEngine) VisualGetRange() [2][2]int {
	return cvim.VisualGetRange()
}

// VisualGetType gets the visual mode type
func (e *CGOEngine) VisualGetType() int {
	return cvim.VisualGetType()
}

// Eval evaluates a vim expression
func (e *CGOEngine) Eval(expr string) string {
	return cvim.Eval(expr)
}

// SearchGetMatchingPair finds matching brackets
func (e *CGOEngine) SearchGetMatchingPair() [2]int {
	return cvim.SearchGetMatchingPair()
}

// CGOBufferWrapper wraps a C buffer
type CGOBufferWrapper struct {
	buf cvim.Buffer
}

// GetID gets the buffer ID
func (b *CGOBufferWrapper) GetID() int {
	return cvim.BufferGetId(b.buf)
}

// GetLine gets a line from the buffer
func (b *CGOBufferWrapper) GetLine(lnum int) string {
	return cvim.BufferGetLine(b.buf, lnum)
}

// GetLineB gets a line as bytes from the buffer
func (b *CGOBufferWrapper) GetLineB(lnum int) []byte {
	return cvim.BufferGetLineB(b.buf, lnum)
}

// GetLineCount gets the number of lines in the buffer
func (b *CGOBufferWrapper) GetLineCount() int {
	return cvim.BufferGetLineCount(b.buf)
}

// Lines gets all lines from the buffer
func (b *CGOBufferWrapper) Lines() []string {
	return cvim.CBufferLines(b.buf)
}

// LinesB gets all lines as bytes from the buffer
func (b *CGOBufferWrapper) LinesB() [][]byte {
	return cvim.BufferLinesB(b.buf)
}

// SetCurrent sets this buffer as the current buffer
func (b *CGOBufferWrapper) SetCurrent() {
	BufferSetCurrent(b.buf)
}

// IsModified checks if the buffer has been modified
func (b *CGOBufferWrapper) IsModified() bool {
	return cvim.BufferGetModified(b.buf)
}

// GetLastChangedTick gets the last changed tick value
func (b *CGOBufferWrapper) GetLastChangedTick() int {
	return cvim.CBufferGetLastChangedTick(b.buf)
}

// SetLines sets all lines in the buffer
func (b *CGOBufferWrapper) SetLines(start, end int, lines []string) {
	cvim.CBufferSetLines(b.buf, start, end, lines, len(lines))
}

// GoEngine implements the Go-based vim engine
type GoEngine struct {
	// Add debug logger
	debugLog *log.Logger
	engine   govim.Engine
}

// Init initializes the vim engine
func (e *GoEngine) Init(argc int) {
	e.engine.Init(argc)
}

// BufferOpen opens a file and returns a buffer
func (e *GoEngine) BufferOpen(filename string, lnum int, flags int) interfaces.VimBuffer {
	//buf := govim.BufferOpen(filename, lnum, flags)
	buf := e.engine.BufferOpen(filename, lnum, flags)
	return &GoBufferWrapper{buf: buf}
}

// BufferNew creates a new empty buffer
func (e *GoEngine) BufferNew(flags int) interfaces.VimBuffer {
	buf := e.engine.BufferNew(flags)
	return &GoBufferWrapper{buf: buf}
}

// BufferGetCurrent gets the current buffer
func (e *GoEngine) BufferGetCurrent() interfaces.VimBuffer {
	buf := e.engine.BufferGetCurrent()
	return &GoBufferWrapper{buf: buf}
}

// BufferSetCurrent sets the current buffer
func (e *GoEngine) BufferSetCurrent(buf interfaces.VimBuffer) {
	if goBuf, ok := buf.(*GoBufferWrapper); ok {
		e.engine.BufferSetCurrent(goBuf.buf)
	}
}

// CursorGetLine gets the current cursor line
func (e *GoEngine) CursorGetLine() int {
	return e.engine.CursorGetLine()
}

// CursorGetPosition gets the cursor position
func (e *GoEngine) CursorGetPosition() [2]int {
	return e.engine.CursorGetPosition()
}

// CursorSetPosition sets the cursor position
func (e *GoEngine) CursorSetPosition(row, col int) {
	e.engine.CursorSetPosition(row, col)
}

// Input sends input to vim
func (e *GoEngine) Input(s string) {
	e.engine.Input(s)
}

// Input2 sends multiple character input
func (e *GoEngine) Input2(s string) {
	for _, x := range s {
		e.engine.Input(string(x))
	}
}

// Key sends special key input
func (e *GoEngine) Key(s string) {
	e.engine.Key(s)
}

// Execute runs an ex command
func (e *GoEngine) Execute(s string) {
	e.engine.Execute(s)
}

// GetMode gets the current mode
func (e *GoEngine) GetMode() int {
	mode := e.engine.GetMode()
	// Add optional debug logging to see if mode is being correctly passed
	fmt.Fprintf(os.Stderr, "GoEngine GetMode: govim.GetMode returned %d\n", mode)
	return mode
}

// GetCurrentMode gets the current mode with application-compatible mappings
func (e *GoEngine) GetCurrentMode() int {
	return e.engine.GetMode()
}

// VisualGetRange gets the visual selection range
func (e *GoEngine) VisualGetRange() [2][2]int {
	return e.engine.VisualGetRange()
}

// VisualGetType gets the visual mode type
func (e *GoEngine) VisualGetType() int {
	return e.engine.VisualGetType()
}

// Eval evaluates a vim expression
func (e *GoEngine) Eval(expr string) string {
	return e.engine.Eval(expr)
}

// SearchGetMatchingPair finds matching brackets
func (e *GoEngine) SearchGetMatchingPair() [2]int {
	return e.engine.SearchGetMatchingPair()
}

// GoBufferWrapper wraps a Go buffer
type GoBufferWrapper struct {
	buf govim.Buffer
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
	result := make([]string, len(lines))
	for i, line := range lines {
		result[i] = line
	}
	return result
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

// API Functions (to be used by the application)

// SwitchToGoImplementation switches to the Go implementation
func SwitchToGoImplementation() {
	// Set up logging for the Go implementation
	logFile, err := os.OpenFile("govim_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		// Log to stderr instead of stdout to avoid affecting the UI
		fmt.Fprintf(os.Stderr, "Failed to open govim log file: %v\n", err)
	}

	ActiveImplementation = ImplGo
	goImpl := &GoImplementation{}

	// Initialize the logger if file was opened successfully
	if err == nil {
		goImpl.logger = log.New(logFile, "GoVim: ", log.Ltime|log.Lshortfile)
		goImpl.logger.Println("Go implementation activated")
	}

	activeImpl = goImpl
}

// SwitchToCImplementation switches to the C implementation
func SwitchToCImplementation() {
	ActiveImplementation = ImplC
	activeImpl = &CGOImplementation{}
}

// GetActiveImplementation returns the name of the active implementation
func GetActiveImplementation() string {
	return activeImpl.GetName()
}

// GetEngine gets the current engine implementation
func GetEngine() interfaces.VimEngine {
	return activeImpl.GetEngine()
}
