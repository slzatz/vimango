package vim

import (
	"github.com/slzatz/vimango/vim/cvim"
	"github.com/slzatz/vimango/vim/interfaces"
)

// InitializeVim sets up the vim engine. libvim via CGO is the only
// implementation (the pure-Go govim engine and its adapter layer were
// removed; Windows and CGO-free builds are unsupported).
func InitializeVim(argc int) {
	Engine = &CGOEngineWrapper{}
	Engine.Init(argc)
}

// CGOEngineWrapper wraps the C-based vim implementation in package cvim
// CGOEngineWrapper satisfies the VimEngine interface
type CGOEngineWrapper struct{}

// Init initializes the vim engine
func (e *CGOEngineWrapper) Init(argc int) {
	cvim.VimInit(argc)
	cvim.EnableSpellSuggestCallback()
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

// SetMappingsEnabled turns expansion of :map definitions on or off
func (e *CGOEngineWrapper) SetMappingsEnabled(enabled bool) {
	cvim.SetMappingsEnabled(enabled)
}

// GetMode gets the current mode
func (e *CGOEngineWrapper) GetMode() int {
	return cvim.GetMode()
}

func (e *CGOEngineWrapper) GetSubMode() cvim.SubMode {
	return cvim.GetSubMode()
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

// SpellGetSuggestions returns the word z= would change and its suggestions
func (e *CGOEngineWrapper) SpellGetSuggestions() (string, []string, bool) {
	return cvim.SpellGetSuggestions()
}

// TakeSpellSuggestRequest reports (and clears) whether z= was typed
func (e *CGOEngineWrapper) TakeSpellSuggestRequest() bool {
	return cvim.TakeSpellSuggestRequest()
}

// CGOBufferWrapper wraps a C buffer
type CGOBufferWrapper struct {
	buf cvim.Buffer
}

// GetID gets the buffer ID
func (b *CGOBufferWrapper) GetID() int {
	return cvim.BufferGetId(b.buf)
}

// Lines gets all lines from the buffer
func (b *CGOBufferWrapper) Lines() []string {
	return cvim.CBufferLines(b.buf)
}

// GetLastChangedTick gets the last changed tick value
func (b *CGOBufferWrapper) GetLastChangedTick() int {
	return cvim.CBufferGetLastChangedTick(b.buf)
}

// SetLines sets all lines in the buffer
func (b *CGOBufferWrapper) SetLines(start, end int, lines []string) {
	cvim.CBufferSetLines(b.buf, start, end, lines, len(lines))
}

// SpellErrors returns the misspellings in lines start..end (1-based, inclusive)
func (b *CGOBufferWrapper) SpellErrors(start, end int) []cvim.SpellError {
	return cvim.SpellGetErrors(b.buf, start, end)
}
