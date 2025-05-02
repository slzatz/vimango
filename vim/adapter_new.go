package vim

import (
	"fmt"
	"log"
	"os"
	
	govim "github.com/slzatz/vimango/vim/govim"
)

// Implementation constants
const (
	ImplC  = "cgo"
	ImplGo = "go"
)

// ActiveImplementation tracks which implementation is active
var ActiveImplementation = ImplC

// CGOImplementation provides the C-based vim implementation
type CGOImplementation struct{}

// GetEngine returns the C-based engine
func (c *CGOImplementation) GetEngine() VimEngine {
	return &CGOEngine{}
}

// GetName returns the implementation name
func (c *CGOImplementation) GetName() string {
	return ImplC
}

// GoImplementation provides the pure Go vim implementation
type GoImplementation struct{
	logger *log.Logger
}

// GetEngine returns the Go-based engine
func (g *GoImplementation) GetEngine() VimEngine {
	engine := &GoEngine{
		debugLog: g.logger,
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
	vimInit(argc)
}

// BufferOpen opens a file and returns a buffer
func (e *CGOEngine) BufferOpen(filename string, lnum int, flags int) VimBuffer {
	buf := BufferOpen(filename, lnum, flags)
	return &CGOBufferWrapper{buf: buf}
}

// BufferNew creates a new empty buffer
func (e *CGOEngine) BufferNew(flags int) VimBuffer {
	buf := CBufferNew(flags)
	return &CGOBufferWrapper{buf: buf}
}

// BufferGetCurrent gets the current buffer
func (e *CGOEngine) BufferGetCurrent() VimBuffer {
	buf := BufferGetCurrent()
	return &CGOBufferWrapper{buf: buf}
}

// BufferSetCurrent sets the current buffer
func (e *CGOEngine) BufferSetCurrent(buf VimBuffer) {
	if cgoBuf, ok := buf.(*CGOBufferWrapper); ok {
		CBufferSetCurrent(cgoBuf.buf)
	}
}

// CursorGetLine gets the current cursor line
func (e *CGOEngine) CursorGetLine() int {
	return CursorGetLine()
}

// CursorGetPosition gets the cursor position
func (e *CGOEngine) CursorGetPosition() [2]int {
	return CursorGetPosition()
}

// CursorSetPosition sets the cursor position
func (e *CGOEngine) CursorSetPosition(row, col int) {
	CursorSetPosition(row, col)
}

// Input sends input to vim
func (e *CGOEngine) Input(s string) {
	Input(s)
}

// Input2 sends multiple character input
func (e *CGOEngine) Input2(s string) {
	Input2(s)
}

// Key sends special key input
func (e *CGOEngine) Key(s string) {
	Key(s)
}

// Execute runs an ex command
func (e *CGOEngine) Execute(s string) {
	Execute(s)
}

// GetMode gets the current mode
func (e *CGOEngine) GetMode() int {
	return GetMode()
}

// VisualGetRange gets the visual selection range
func (e *CGOEngine) VisualGetRange() [2][2]int {
	return VisualGetRange()
}

// VisualGetType gets the visual mode type
func (e *CGOEngine) VisualGetType() int {
	return VisualGetType()
}

// Eval evaluates a vim expression
func (e *CGOEngine) Eval(expr string) string {
	return Eval(expr)
}

// SearchGetMatchingPair finds matching brackets
func (e *CGOEngine) SearchGetMatchingPair() [2]int {
	return SearchGetMatchingPair()
}

// CGOBufferWrapper wraps a C buffer
type CGOBufferWrapper struct {
	buf Buffer
}

// GetID gets the buffer ID
func (b *CGOBufferWrapper) GetID() int {
	return BufferGetId(b.buf)
}

// GetLine gets a line from the buffer
func (b *CGOBufferWrapper) GetLine(lnum int) string {
	return BufferGetLine(b.buf, lnum)
}

// GetLineB gets a line as bytes from the buffer
func (b *CGOBufferWrapper) GetLineB(lnum int) []byte {
	return BufferGetLineB(b.buf, lnum)
}

// GetLineCount gets the number of lines in the buffer
func (b *CGOBufferWrapper) GetLineCount() int {
	return BufferGetLineCount(b.buf)
}

// Lines gets all lines from the buffer
func (b *CGOBufferWrapper) Lines() []string {
	return CBufferLines(b.buf)
}

// LinesB gets all lines as bytes from the buffer
func (b *CGOBufferWrapper) LinesB() [][]byte {
	return BufferLinesB(b.buf)
}

// SetCurrent sets this buffer as the current buffer
func (b *CGOBufferWrapper) SetCurrent() {
	BufferSetCurrent(b.buf)
}

// IsModified checks if the buffer has been modified
func (b *CGOBufferWrapper) IsModified() bool {
	return BufferGetModified(b.buf)
}

// GetLastChangedTick gets the last changed tick value
func (b *CGOBufferWrapper) GetLastChangedTick() int {
	return CBufferGetLastChangedTick(b.buf)
}

// SetLines sets all lines in the buffer
func (b *CGOBufferWrapper) SetLines(start, end int, lines []string) {
	CBufferSetLines(b.buf, start, end, lines, len(lines))
}

// GoEngine implements the Go-based vim engine
type GoEngine struct{
	// Add debug logger
	debugLog *log.Logger
}

// Init initializes the vim engine
func (e *GoEngine) Init(argc int) {
	govim.Init(argc)
}

// BufferOpen opens a file and returns a buffer
func (e *GoEngine) BufferOpen(filename string, lnum int, flags int) VimBuffer {
	buf := govim.BufferOpen(filename, lnum, flags)
	return &GoBufferWrapper{buf: buf}
}

// BufferNew creates a new empty buffer
func (e *GoEngine) BufferNew(flags int) VimBuffer {
	if e.debugLog != nil {
		e.debugLog.Println("BufferNew called with flags:", flags)
	}
	
	buf := govim.BufferNew(flags)
	if buf == nil {
		if e.debugLog != nil {
			e.debugLog.Println("BufferNew failed - returned nil buffer")
		}
		fmt.Println("WARNING: Go implementation BufferNew returned nil")
	} else {
		if e.debugLog != nil {
			e.debugLog.Println("BufferNew succeeded")
		}
	}
	
	return &GoBufferWrapper{buf: buf}
}

// BufferGetCurrent gets the current buffer
func (e *GoEngine) BufferGetCurrent() VimBuffer {
	buf := govim.BufferGetCurrent()
	return &GoBufferWrapper{buf: buf}
}

// BufferSetCurrent sets the current buffer
func (e *GoEngine) BufferSetCurrent(buf VimBuffer) {
	if goBuf, ok := buf.(*GoBufferWrapper); ok {
		govim.BufferSetCurrent(goBuf.buf)
	}
}

// CursorGetLine gets the current cursor line
func (e *GoEngine) CursorGetLine() int {
	return govim.CursorGetLine()
}

// CursorGetPosition gets the cursor position
func (e *GoEngine) CursorGetPosition() [2]int {
	return govim.CursorGetPosition()
}

// CursorSetPosition sets the cursor position
func (e *GoEngine) CursorSetPosition(row, col int) {
	govim.CursorSetPosition(row, col)
}

// Input sends input to vim
func (e *GoEngine) Input(s string) {
	govim.Input(s)
}

// Input2 sends multiple character input
func (e *GoEngine) Input2(s string) {
	govim.Input2(s)
}

// Key sends special key input
func (e *GoEngine) Key(s string) {
	govim.Key(s)
}

// Execute runs an ex command
func (e *GoEngine) Execute(s string) {
	govim.Execute(s)
}

// GetMode gets the current mode
func (e *GoEngine) GetMode() int {
	return govim.GetMode()
}

// VisualGetRange gets the visual selection range
func (e *GoEngine) VisualGetRange() [2][2]int {
	return govim.VisualGetRange()
}

// VisualGetType gets the visual mode type
func (e *GoEngine) VisualGetType() int {
	return govim.VisualGetType()
}

// Eval evaluates a vim expression
func (e *GoEngine) Eval(expr string) string {
	return govim.Eval(expr)
}

// SearchGetMatchingPair finds matching brackets
func (e *GoEngine) SearchGetMatchingPair() [2]int {
	return govim.SearchGetMatchingPair()
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
	return b.buf.GetLine(lnum)
}

// GetLineB gets a line as bytes from the buffer
func (b *GoBufferWrapper) GetLineB(lnum int) []byte {
	return b.buf.GetLineB(lnum)
}

// GetLineCount gets the number of lines in the buffer
func (b *GoBufferWrapper) GetLineCount() int {
	return b.buf.GetLineCount()
}

// Lines gets all lines from the buffer
func (b *GoBufferWrapper) Lines() []string {
	return b.buf.Lines()
}

// LinesB gets all lines as bytes from the buffer
func (b *GoBufferWrapper) LinesB() [][]byte {
	return b.buf.LinesB()
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
	// For debugging - print to stdout
	fmt.Printf("GoBufferWrapper.SetLines called: start=%d, end=%d, lines length=%d\n", start, end, len(lines))
	
	// Convert nils to empty arrays to prevent panics
	if lines == nil {
		lines = []string{}
	}
	
	// Try to call SetLines with error handling
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("PANIC in GoBufferWrapper.SetLines:", r)
			// Print debug information
			fmt.Printf("Buffer: %v, Start: %d, End: %d, Lines length: %d\n", b.buf, start, end, len(lines))
			
			// Try to recover by using an alternative approach
			if goBuf, ok := b.buf.(*govim.GoBuffer); ok {
				// Safe operation on the detected buffer
				fmt.Println("Attempting recovery with direct line manipulation")
				
				// Validate inputs for safety
				if start < 0 {
					start = 0
				}
				
				// Handle single-line update case (common with 'x' command)
				if len(lines) == 1 && start >= 0 && start < goBuf.GetLineCount() {
					// Get all lines
					allLines := goBuf.Lines()
					
					// Replace just the one line
					allLines[start] = lines[0]
					
					// Get a new reference and update all lines directly
					goBuf.SetLines(0, goBuf.GetLineCount(), allLines)
					fmt.Println("Recovery succeeded")
				}
			}
		}
	}()
	
	// Now that Buffer interface has SetLines method, we can call it directly
	// But still try with concrete type first for better error handling
	if goBuf, ok := b.buf.(*govim.GoBuffer); ok {
		goBuf.SetLines(start, end, lines)
	} else {
		// Fall back to the interface method
		b.buf.SetLines(start, end, lines)
	}
}

// API Functions (to be used by the application)

// SwitchToGoImplementation switches to the Go implementation
func SwitchToGoImplementation() {
	fmt.Println("Switching to Go implementation")
	
	// Set up logging for the Go implementation
	logFile, err := os.OpenFile("govim_debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Failed to open log file:", err)
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
func GetEngine() VimEngine {
	return activeImpl.GetEngine()
}