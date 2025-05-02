package vim

import (
	"testing"
)

func TestAdapterLayerWithCImplementation(t *testing.T) {
	// Configure with C implementation
	Configure(Config{UseGoImplementation: false})
	
	// Check that we're using the C implementation
	if IsUsingGoImplementation() {
		t.Errorf("Expected to be using C implementation, but using Go implementation")
	}
	
	// Create a buffer and do some basic operations
	buf := NewBuffer(0)
	SetCurrentBuffer(buf)
	
	// Get current buffer
	currentBuf := GetCurrentBuffer()
	if currentBuf == nil {
		t.Errorf("Failed to get current buffer")
	}
}

func TestAdapterLayerWithGoImplementation(t *testing.T) {
	// Skip if CGO is needed for the implementation
	t.Skip("Skipping Go implementation test until it's fully implemented")
	
	// Configure with Go implementation
	Configure(Config{UseGoImplementation: true})
	
	// Check that we're using the Go implementation
	if !IsUsingGoImplementation() {
		t.Errorf("Expected to be using Go implementation, but using C implementation")
	}
	
	// Create a buffer and do some basic operations
	buf := NewBuffer(0)
	SetCurrentBuffer(buf)
	
	// Get current buffer
	currentBuf := GetCurrentBuffer()
	if currentBuf == nil {
		t.Errorf("Failed to get current buffer")
	}
	
	// Test that buffer operations work
	if currentBuf.GetLineCount() < 1 {
		t.Errorf("Expected at least one line in buffer")
	}
	
	// Check cursor operations
	pos := GetCursorPosition()
	if pos[0] < 1 || pos[1] < 0 {
		t.Errorf("Invalid cursor position: %v", pos)
	}
	
	// Set new cursor position
	SetCursorPosition(1, 0)
	
	// Verify position was set
	newPos := GetCursorPosition()
	if newPos[0] != 1 || newPos[1] != 0 {
		t.Errorf("Cursor position not set correctly. Expected [1,0], got %v", newPos)
	}
}

func TestToggleImplementation(t *testing.T) {
	// Configure with C implementation
	Configure(Config{UseGoImplementation: false})
	
	// Check initial state
	if IsUsingGoImplementation() {
		t.Errorf("Expected to start with C implementation")
	}
	
	// Toggle to Go implementation
	impl := ToggleImplementation()
	if impl != ImplGo {
		t.Errorf("Expected to toggle to Go implementation, got %s", impl)
	}
	
	// Toggle back to C implementation
	impl = ToggleImplementation()
	if impl != ImplC {
		t.Errorf("Expected to toggle back to C implementation, got %s", impl)
	}
}