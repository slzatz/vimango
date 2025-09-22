package vim

import (
	"fmt"
	"log"
	"os"

	"github.com/slzatz/vimango/vim/cvim"
	"github.com/slzatz/vimango/vim/interfaces"
)

// This file provides an API layer for the application to interact with vim
// regardless of whether the C or Go implementation is being used.

// Engine is the active engine wrapper of the C or Go implementation:
// either  CGOEngineWrapper or  GoEngineWrapper, which satisfy the VimEngine interface
var Engine interfaces.VimEngine

// activeImpl is the current implementation (C or Go)
var activeImpl VimImplementation

// ActiveImplementation tracks which implementation is active
var ActiveImplementation = ImplC

// API Functions that deal with which implementation is being used

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

// GetActiveImplementation returns the name of the active implementation
func GetActiveImplementation() string {
	return activeImpl.GetName()
}

// GetEngine gets the current engine implementation
func GetEngineWrapper() interfaces.VimEngine {
	return activeImpl.GetEngineWrapper()
}

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
	// For the enter key in insert mode, we need special handling in our Go implementation
	// This ensures the auto-indent functionality works properly
	if (s == "<cr>" || s == "<enter>" || s == "<return>") && IsUsingGoImplementation() && GetCurrentMode() == 16 {
		// Handle enter key specially - as if \r was typed
		Engine.Input("\r")
		return
	}

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

// IsUsingGoImplementation checks if we're using the Go implementation
func IsUsingGoImplementation() bool {
	return GetActiveImplementation() == ImplGo
}

// Additional backward compatibility functions

// ToggleImplementation switches between Go and C implementations

// Helper functions to convert between buffer types
// (CGO-specific conversion functions are in api_cgo_compat.go)
