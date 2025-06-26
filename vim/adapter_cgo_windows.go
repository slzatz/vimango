//go:build windows

package vim

import (
	"github.com/slzatz/vimango/vim/interfaces"
)

// CGOImplementation provides a dummy C-based vim implementation for Windows.
type CGOImplementation struct{}

// GetEngineWrapper returns a dummy C-based engine wrapper.
func (c *CGOImplementation) GetEngineWrapper() interfaces.VimEngine {
	return &CGOEngineWrapper{}
}

// GetName returns the implementation name.
func (c *CGOImplementation) GetName() string {
	return ImplC
}

// activeImpl is initialized here for Windows builds, always to GoImplementation.
func init() {
	activeImpl = &GoImplementation{}
}

// CGOEngineWrapper is a dummy wrapper for Windows builds.
type CGOEngineWrapper struct{}

// Init is a no-op on Windows.
func (e *CGOEngineWrapper) Init(argc int) {}

// BufferOpen is a no-op on Windows.
func (e *CGOEngineWrapper) BufferOpen(filename string, lnum int, flags int) interfaces.VimBuffer {
	return nil
}

// BufferNew is a no-op on Windows.
func (e *CGOEngineWrapper) BufferNew(flags int) interfaces.VimBuffer {
	return nil
}

// BufferGetCurrent is a no-op on Windows.
func (e *CGOEngineWrapper) BufferGetCurrent() interfaces.VimBuffer {
	return nil
}

// BufferSetCurrent is a no-op on Windows.
func (e *CGOEngineWrapper) BufferSetCurrent(buf interfaces.VimBuffer) {}

// CursorGetLine is a no-op on Windows.
func (e *CGOEngineWrapper) CursorGetLine() int {
	return 0
}

// CursorGetPosition is a no-op on Windows.
func (e *CGOEngineWrapper) CursorGetPosition() [2]int {
	return [2]int{0, 0}
}

// CursorSetPosition is a no-op on Windows.
func (e *CGOEngineWrapper) CursorSetPosition(row, col int) {}

// Input is a no-op on Windows.
func (e *CGOEngineWrapper) Input(s string) {}

// Input2 is a no-op on Windows.
func (e *CGOEngineWrapper) Input2(s string) {}

// Key is a no-op on Windows.
func (e *CGOEngineWrapper) Key(s string) {}

// Execute is a no-op on Windows.
func (e *CGOEngineWrapper) Execute(s string) {}

// GetMode is a no-op on Windows.
func (e *CGOEngineWrapper) GetMode() int {
	return 0
}

// GetCurrentMode is a no-op on Windows.
func (e *CGOEngineWrapper) GetCurrentMode() int {
	return 0
}

// VisualGetRange is a no-op on Windows.
func (e *CGOEngineWrapper) VisualGetRange() [2][2]int {
	return [2][2]int{{0, 0}, {0, 0}}
}

// VisualGetType is a no-op on Windows.
func (e *CGOEngineWrapper) VisualGetType() int {
	return 0
}

// Eval is a no-op on Windows.
func (e *CGOEngineWrapper) Eval(expr string) string {
	return ""
}

// SearchGetMatchingPair is a no-op on Windows.
func (e *CGOEngineWrapper) SearchGetMatchingPair() [2]int {
	return [2]int{0, 0}
}

// CGOBufferWrapper is a dummy wrapper for Windows builds.
type CGOBufferWrapper struct {
	buf uintptr // Use uintptr as a dummy for cvim.Buffer
}

// GetID is a no-op on Windows.
func (b *CGOBufferWrapper) GetID() int {
	return 0
}

// GetLine is a no-op on Windows.
func (b *CGOBufferWrapper) GetLine(lnum int) string {
	return ""
}

// GetLineB is a no-op on Windows.
func (b *CGOBufferWrapper) GetLineB(lnum int) []byte {
	return nil
}

// GetLineCount is a no-op on Windows.
func (b *CGOBufferWrapper) GetLineCount() int {
	return 0
}

// Lines is a no-op on Windows.
func (b *CGOBufferWrapper) Lines() []string {
	return nil
}

// LinesB is a no-op on Windows.
func (b *CGOBufferWrapper) LinesB() [][]byte {
	return nil
}

// SetCurrent is a no-op on Windows.
func (b *CGOBufferWrapper) SetCurrent() {}

// IsModified is a no-op on Windows.
func (b *CGOBufferWrapper) IsModified() bool {
	return false
}

// GetLastChangedTick is a no-op on Windows.
func (b *CGOBufferWrapper) GetLastChangedTick() int {
	return 0
}

// SetLines is a no-op on Windows.
func (b *CGOBufferWrapper) SetLines(start, end int, lines []string) {}
