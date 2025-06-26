//go:build !windows

package vim

import (
	"github.com/slzatz/vimango/vim/cvim"
	"github.com/slzatz/vimango/vim/interfaces"
)

// CGOImplementation provides the C-based vim implementation
type CGOImplementation struct{}

// GetEngineWrapper returns the C-based engine wrapper
func (c *CGOImplementation) GetEngineWrapper() interfaces.VimEngine {
	return &CGOEngineWrapper{}
}

// GetName returns the implementation name
func (c *CGOImplementation) GetName() string {
	return ImplC
}

// activeImpl is the current implementation (C or Go)
// This is initialized here for non-Windows builds
func init() {
	activeImpl = &CGOImplementation{}
}

// CGOEngineWrapper wraps the C-based vim implementation in package cvim
// CGOEngineWrapper satisfies the VimEngine interface
type CGOEngineWrapper struct{}

// Init initializes the vim engine
func (e *CGOEngineWrapper) Init(argc int) {
	cvim.VimInit(argc)
}

// BufferOpen opens a file and returns a buffer
func (e *CGOEngineWrapper) BufferOpen(filename string, lnum int, flags int) interfaces.VimBuffer {
	buf := cvim.BufferOpen(filename, lnum, flags)
	return &CGOBufferWrapper{buf: buf}
}

// BufferNew creates a new empty buffer
func (e *CGOEngineWrapper) BufferNew(flags int) interfaces.VimBuffer {
	buf := cvim.CBufferNew(flags)
	return &CGOBufferWrapper{buf: buf}
}

// BufferGetCurrent gets the current buffer
func (e *CGOEngineWrapper) BufferGetCurrent() interfaces.VimBuffer {
	buf := cvim.BufferGetCurrent()
	return &CGOBufferWrapper{buf: buf}
}

// BufferSetCurrent sets the current buffer
func (e *CGOEngineWrapper) BufferSetCurrent(buf interfaces.VimBuffer) {
	/*
		if cgoBufWrap, ok := buf.(*CGOBufferWrapper); ok {
			cvim.CBufferSetCurrent(cgoBufWrap.buf)
		}
	*/
	// doesn't need if statement to check if buf is a CGOBufferWrapper
	cvim.CBufferSetCurrent(buf.(*CGOBufferWrapper).buf)
}

// CursorGetLine gets the current cursor line
func (e *CGOEngineWrapper) CursorGetLine() int {
	return cvim.CursorGetLine()
}

// CursorGetPosition gets the cursor position
func (e *CGOEngineWrapper) CursorGetPosition() [2]int {
	return cvim.CursorGetPosition()
}

// CursorSetPosition sets the cursor position
func (e *CGOEngineWrapper) CursorSetPosition(row, col int) {
	cvim.CursorSetPosition(row, col)
}

// Input sends input to vim
func (e *CGOEngineWrapper) Input(s string) {
	cvim.Input(s)
}

// Input2 sends multiple character input
func (e *CGOEngineWrapper) Input2(s string) {
	cvim.Input2(s)
}

// Key sends special key input
func (e *CGOEngineWrapper) Key(s string) {
	cvim.Key(s)
}

// Execute runs an ex command
func (e *CGOEngineWrapper) Execute(s string) {
	cvim.Execute(s)
}

// GetMode gets the current mode
func (e *CGOEngineWrapper) GetMode() int {
	return cvim.GetMode()
}

// GetCurrentMode gets the current mode with application-compatible mappings
// For CGO implementation, this is the same as GetMode
func (e *CGOEngineWrapper) GetCurrentMode() int {
	return cvim.GetMode()
}

// VisualGetRange gets the visual selection range
func (e *CGOEngineWrapper) VisualGetRange() [2][2]int {
	return cvim.VisualGetRange()
}

// VisualGetType gets the visual mode type
func (e *CGOEngineWrapper) VisualGetType() int {
	return cvim.VisualGetType()
}

// Eval evaluates a vim expression
func (e *CGOEngineWrapper) Eval(expr string) string {
	return cvim.Eval(expr)
}

// SearchGetMatchingPair finds matching brackets
func (e *CGOEngineWrapper) SearchGetMatchingPair() [2]int {
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
