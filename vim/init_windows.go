//go:build windows

package vim

import (
    "github.com/slzatz/vimango/vim/interfaces"
)

// InitializeVim forces the Go implementation on Windows.
func InitializeVim(useGoImplementation bool, argc int) {
    // On Windows, we ALWAYS use the Go implementation.
    SwitchToGoImplementation()
    Engine = GetEngineWrapper()
    Engine.Init(argc)
}

// SwitchToCImplementation is a no-op on Windows.
func SwitchToCImplementation() {
    // This function does nothing, preventing CGO from being used.
    // We could panic here to indicate a programming error if it's ever called.
    panic("Attempted to use CGO implementation on Windows")
}


// ToggleImplementation switches between Go and C implementations
func ToggleImplementation() string {
	// Get current buffer before switching to ensure we can
	// reset its state after the switch
	var currentBuffer interfaces.VimBuffer
	if Engine != nil {
		currentBuffer = Engine.BufferGetCurrent()
	}

	// On Windows, we always switch to the Go implementation.
	SwitchToGoImplementation()

	// Reset buffer state if we had an active buffer
	if currentBuffer != nil && Engine != nil {
		// Force buffer reset to clean any stale state
		Engine.BufferSetCurrent(currentBuffer)
	}

	return GetActiveImplementation()
}
