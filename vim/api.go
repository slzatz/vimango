package vim

import (
	"github.com/slzatz/vimango/vim/cvim"
	"github.com/slzatz/vimango/vim/interfaces"
)

// This file provides the API layer the application uses to interact with vim.
// libvim via CGO is the only implementation.

// Engine is the active engine wrapper (CGOEngineWrapper, set by InitializeVim)
var Engine interfaces.VimEngine

// API Functions - called by package main as vim.NewBuffer(..), vim.SendKey(..), etc.

// NewBuffer creates a new empty buffer
// Returns VimBuffer for the new adapter API but can be used with old code too
func NewBuffer(flags int) interfaces.VimBuffer {
	return Engine.BufferNew(flags)
}

/*
// For backward compatibility with existing code
func BufferNew(flags int) cvim.Buffer {
	b := Engine.BufferNew(flags)
	if wrapper, ok := b.(*CGOBufferWrapper); ok {
		return wrapper.buf
	}
	// Fallback for non-CGO implementation - this may cause issues
	return nil
}
*/

// GetCurrentBuffer gets the current buffer
func GetCurrentBuffer() interfaces.VimBuffer {
	return Engine.BufferGetCurrent()
}

// SetCurrentBuffer sets the current buffer
func SetCurrentBuffer(buf interfaces.VimBuffer) {
	Engine.BufferSetCurrent(buf)
}

// GetCursorPosition gets the cursor position
func GetCursorPosition() [2]int {
	return Engine.CursorGetPosition()
}

// SetCursorPosition sets the cursor position
func SetCursorPosition(row, col int) {
	Engine.CursorSetPosition(row, col)
}

// SendInput sends input to vim
func SendInput(s string) {
	Engine.Input(s)
}

// SendMultiInput sends multiple character input
func SendMultiInput(s string) {
	Engine.Input2(s)
}

// SendKey sends special key input
func SendKey(s string) {
	Engine.Key(s)
}

// ExecuteCommand runs an ex command
func ExecuteCommand(s string) {
	Engine.Execute(s)
}

// SendUserKey forwards a key the user typed with vim's mapping expansion on,
// so definitions from vimango.vim apply to it. Every other Send* call runs
// with expansion off: vimango's own key sequences (the ":" forwards, the
// cmdline's <c-e><c-u>) must mean what vim defines them to mean, whatever the
// user has mapped.
func SendUserKey(send func()) {
	Engine.SetMappingsEnabled(true)
	defer Engine.SetMappingsEnabled(false)
	send()
}

// GetCurrentMode gets the current mode
// This is specifically used by the editor to determine the mode
func GetCurrentMode() int {
	return Engine.GetMode()
}

// This is specifically used by the editor to determine the mode
func GetSubMode() cvim.SubMode {
	return Engine.GetSubMode()
}

// GetVisualRange gets the visual selection range
func GetVisualRange() [2][2]int {
	return Engine.VisualGetRange()
}

// GetVisualType gets the visual mode type
func GetVisualType() int {
	return Engine.VisualGetType()
}

// EvaluateExpression evaluates a vim expression
func EvaluateExpression(expr string) string {
	return Engine.Eval(expr)
}

// GetMatchingPair finds matching brackets
func GetMatchingPair() [2]int {
	return Engine.SearchGetMatchingPair()
}

// GetSpellSuggestions returns the word z= would change (the misspelling under
// or after the cursor, else the word under it) and its suggestions, best
// first. Sending "{n}z=" replaces the word with suggestions[n-1]. ok is false
// when 'spell' is off or there is no word.
func GetSpellSuggestions() (word string, suggestions []string, ok bool) {
	return Engine.SpellGetSuggestions()
}

// TakeSpellSuggestRequest reports whether the user typed z= (without a count)
// since the last call, and clears the request.
func TakeSpellSuggestRequest() bool {
	return Engine.TakeSpellSuggestRequest()
}

// Command-line (cmdline) state — like BufferNew, these call cvim directly
// rather than going through the Engine wrapper.

// CommandLineGetType returns ':', '/' or '?' while vim is in cmdline mode, 0 otherwise
func CommandLineGetType() byte {
	return cvim.CommandLineGetType()
}

// CommandLineGetText returns the current contents of vim's cmdline buffer
func CommandLineGetText() string {
	return cvim.CommandLineGetText()
}

// CommandLineGetPosition returns the cursor position within the cmdline
func CommandLineGetPosition() int {
	return cvim.CommandLineGetPosition()
}
