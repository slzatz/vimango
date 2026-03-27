//go:build cgo && !windows

package vim

import (
	"github.com/slzatz/vimango/vim/interfaces"
)

// InitializeVim sets up the vim engine for non-Windows systems.
func InitializeVim(useGoImplementation bool, argc int) {
    if useGoImplementation {
        SwitchToGoImplementation()
    } else {
        SwitchToCImplementation()
    }
    Engine = GetEngineWrapper()
    Engine.Init(argc)
}

// SwitchToCImplementation switches to the C implementation.
func SwitchToCImplementation() {
    ActiveImplementation = ImplC
    activeImpl = &CGOImplementation{}
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
