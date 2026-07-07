package vim

import (
	"github.com/slzatz/vimango/vim/cvim"
	"github.com/slzatz/vimango/vim/interfaces"
)

// This file provides the API layer the application uses to interact with vim.
// libvim via CGO is the only implementation.

// Engine is the active engine wrapper (CGOEngineWrapper, set by InitializeVim)
var Engine interfaces.VimEngine

// API Functions - These functions are called by package main as vim.OpenBuffer (..) for example

// OpenBuffer opens a file and returns a buffer - not currently in use
func OpenBuffer(filename string, lnum int, flags int) interfaces.VimBuffer {
	return Engine.BufferOpen(filename, lnum, flags)
}

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

// GetCursorLine gets the current cursor line
func GetCursorLine() int {
	return Engine.CursorGetLine()
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

// Helper functions to convert between buffer types
// (CGO-specific conversion functions are in api_cgo_compat.go)
