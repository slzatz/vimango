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

// Engine is the active engine instance
// The CGOEngineWrapper and the GoEngineWrapper are the two implementations
// the Engine Wrappers need to satisfy the VimEngine interface
var Engine interfaces.VimEngine

// InitializeVim sets up the vim engine with the selected implementation
func InitializeVim(useGoImplementation bool, argc int) {
	// Set the implementation
	if useGoImplementation {
		SwitchToGoImplementation()
	} else {
		SwitchToCImplementation()
	}

	// Get the engine wrapper for the active implementation
	Engine = GetEngineWrapper()

	// Initialize vim
	Engine.Init(argc)
}

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
func GetEngineWrapper() interfaces.VimEngine {
	return activeImpl.GetEngineWrapper()
}

// Init initializes vim
func Init(argc int) {
	if Engine == nil {
		InitializeVim(false, argc)
	} else {
		Engine.Init(argc)
	}
}

// API Functions - These wrap the engine calls

// OpenBuffer opens a file and returns a buffer
func OpenBuffer(filename string, lnum int, flags int) interfaces.VimBuffer {
	return Engine.BufferOpen(filename, lnum, flags)
}

// NewBuffer creates a new empty buffer
// Returns VimBuffer for the new adapter API but can be used with old code too
func NewBuffer(flags int) (result interfaces.VimBuffer) {
	return Engine.BufferNew(flags)
}

// For backward compatibility with existing code
func BufferNew(flags int) cvim.Buffer {
	b := Engine.BufferNew(flags)
	if wrapper, ok := b.(*CGOBufferWrapper); ok {
		return wrapper.buf
	}
	// Fallback for non-CGO implementation - this may cause issues
	return nil
}

// GetCurrentBuffer gets the current buffer
func GetCurrentBuffer() interfaces.VimBuffer {
	return Engine.BufferGetCurrent()
}

// SetCurrentBuffer sets the current buffer
func SetCurrentBuffer(buf interfaces.VimBuffer) {
	Engine.BufferSetCurrent(buf)
}

// For backward compatibility with existing code
func BufferSetCurrent(buf cvim.Buffer) {
	if buf == nil {
		return
	}
	wrapper := &CGOBufferWrapper{buf: buf}
	Engine.BufferSetCurrent(wrapper)
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

// BufferLines gets all lines from a buffer (old style)
func BufferLines(buf cvim.Buffer) []string {
	if buf == nil {
		return nil
	}
	wrapper := &CGOBufferWrapper{buf: buf}
	return wrapper.Lines()
}

// BufferGetLastChangedTick gets the last changed tick
func BufferGetLastChangedTick(buf cvim.Buffer) int {
	if buf == nil {
		return 0
	}
	wrapper := &CGOBufferWrapper{buf: buf}
	return wrapper.GetLastChangedTick()
}

// BufferSetLines sets lines in a buffer
func BufferSetLines(buf cvim.Buffer, start, end int, lines []string, count int) {
	if buf == nil {
		return
	}
	wrapper := &CGOBufferWrapper{buf: buf}
	wrapper.SetLines(start, end, lines)
}

// ToggleImplementation switches between Go and C implementations
func ToggleImplementation() string {
	// Get current buffer before switching to ensure we can
	// reset its state after the switch
	var currentBuffer interfaces.VimBuffer
	if Engine != nil {
		currentBuffer = Engine.BufferGetCurrent()
	}

	// Switch implementation
	if IsUsingGoImplementation() {
		SwitchToCImplementation()
	} else {
		SwitchToGoImplementation()
	}

	// Reset buffer state if we had an active buffer
	if currentBuffer != nil && Engine != nil {
		// Force buffer reset to clean any stale state
		Engine.BufferSetCurrent(currentBuffer)
	}

	return GetActiveImplementation()
}

// Helper functions to convert between buffer types

// BufferToVimBuffer converts an old-style Buffer to a VimBuffer interface
func BufferToVimBuffer(buf cvim.Buffer) interfaces.VimBuffer {
	if buf == nil {
		return nil
	}
	return &CGOBufferWrapper{buf: buf}
}

// VimBufferToBuffer attempts to convert a VimBuffer to a Buffer
// Returns nil if conversion is not possible
func VimBufferToBuffer(buf interfaces.VimBuffer) cvim.Buffer {
	if buf == nil {
		return nil
	}
	if wrapper, ok := buf.(*CGOBufferWrapper); ok {
		return wrapper.buf
	}
	return nil
}
