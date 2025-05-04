package vim

import (
	"fmt"
)

// This file provides an API layer for the application to interact with vim
// regardless of whether the C or Go implementation is being used.

// Engine is the default engine instance
var Engine VimEngine

// InitializeVim sets up the vim engine with the selected implementation
func InitializeVim(useGoImplementation bool, argc int) {
	// Set the implementation
	if useGoImplementation {
		SwitchToGoImplementation()
	} else {
		SwitchToCImplementation()
	}
	
	// Get the engine
	Engine = GetEngine()
	
	// Initialize vim
	Engine.Init(argc)
}

// API Functions - These wrap the engine calls

// Init initializes vim
func Init(argc int) {
	if Engine == nil {
		InitializeVim(false, argc)
	} else {
		Engine.Init(argc)
	}
}

// OpenBuffer opens a file and returns a buffer
func OpenBuffer(filename string, lnum int, flags int) (result VimBuffer) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC in OpenBuffer: %v - falling back to C implementation\n", r)
			// Attempt to fallback to C implementation
			origImpl := GetActiveImplementation()
			SwitchToCImplementation()
			result = Engine.BufferOpen(filename, lnum, flags)
			// Switch back to original implementation for other operations
			if origImpl == ImplGo {
				SwitchToGoImplementation()
			}
		}
	}()
	return Engine.BufferOpen(filename, lnum, flags)
}

// NewBuffer creates a new empty buffer
// Returns VimBuffer for the new adapter API but can be used with old code too
func NewBuffer(flags int) (result VimBuffer) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC in NewBuffer: %v - falling back to C implementation\n", r)
			// Attempt to fallback to C implementation
			origImpl := GetActiveImplementation()
			SwitchToCImplementation()
			result = Engine.BufferNew(flags)
			// Switch back to original implementation for other operations
			if origImpl == ImplGo {
				SwitchToGoImplementation()
			}
		}
	}()
	return Engine.BufferNew(flags)
}

// For backward compatibility with existing code
func BufferNew(flags int) Buffer {
	b := Engine.BufferNew(flags)
	if wrapper, ok := b.(*CGOBufferWrapper); ok {
		return wrapper.buf
	}
	// Fallback for non-CGO implementation - this may cause issues
	return nil
}

// GetCurrentBuffer gets the current buffer
func GetCurrentBuffer() (result VimBuffer) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC in GetCurrentBuffer: %v - falling back to C implementation\n", r)
			// Attempt to fallback to C implementation
			origImpl := GetActiveImplementation()
			SwitchToCImplementation()
			result = Engine.BufferGetCurrent()
			// Switch back to original implementation for other operations
			if origImpl == ImplGo {
				SwitchToGoImplementation()
			}
		}
	}()
	return Engine.BufferGetCurrent()
}

// SetCurrentBuffer sets the current buffer
func SetCurrentBuffer(buf VimBuffer) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC in SetCurrentBuffer: %v - falling back to C implementation\n", r)
			// Attempt to fallback to C implementation
			origImpl := GetActiveImplementation()
			SwitchToCImplementation()
			Engine.BufferSetCurrent(buf)
			// Switch back to original implementation for other operations
			if origImpl == ImplGo {
				SwitchToGoImplementation()
			}
		}
	}()
	Engine.BufferSetCurrent(buf)
}

// For backward compatibility with existing code
func BufferSetCurrent(buf Buffer) {
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
	if s == "<cr>" && IsUsingGoImplementation() && GetCurrentMode() == 16 {
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
	// If we're using the Go implementation, make sure to map our internal 
	// mode values to what the application expects
	if IsUsingGoImplementation() {
		// GetCurrentMode handles the special mapping for command mode
		return Engine.GetCurrentMode()
	}
	// For C implementation, use regular GetMode
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
func BufferLines(buf Buffer) []string {
	if buf == nil {
		return nil
	}
	wrapper := &CGOBufferWrapper{buf: buf}
	return wrapper.Lines()
}

// BufferGetLastChangedTick gets the last changed tick
func BufferGetLastChangedTick(buf Buffer) int {
	if buf == nil {
		return 0
	}
	wrapper := &CGOBufferWrapper{buf: buf}
	return wrapper.GetLastChangedTick()
}

// BufferSetLines sets lines in a buffer
func BufferSetLines(buf Buffer, start, end int, lines []string, count int) {
	if buf == nil {
		return
	}
	
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("PANIC in BufferSetLines: %v - falling back to direct C implementation\n", r)
			// Try using the C implementation directly if possible
			origImpl := GetActiveImplementation()
			SwitchToCImplementation()
			
			// Create a fresh wrapper to avoid any corrupted state
			wrapper := &CGOBufferWrapper{buf: buf}
			wrapper.SetLines(start, end, lines)
			
			// Switch back to original implementation for other operations
			if origImpl == ImplGo {
				SwitchToGoImplementation()
			}
		}
	}()
	
	wrapper := &CGOBufferWrapper{buf: buf}
	wrapper.SetLines(start, end, lines)
}

// ToggleImplementation switches between Go and C implementations
func ToggleImplementation() string {
	// Get current buffer before switching to ensure we can
	// reset its state after the switch
	var currentBuffer VimBuffer
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
func BufferToVimBuffer(buf Buffer) VimBuffer {
	if buf == nil {
		return nil
	}
	return &CGOBufferWrapper{buf: buf}
}

// VimBufferToBuffer attempts to convert a VimBuffer to a Buffer
// Returns nil if conversion is not possible
func VimBufferToBuffer(buf VimBuffer) Buffer {
	if buf == nil {
		return nil
	}
	if wrapper, ok := buf.(*CGOBufferWrapper); ok {
		return wrapper.buf
	}
	return nil
}