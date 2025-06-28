//go:build !cgo && !windows

package vim

import (
	"github.com/slzatz/vimango/vim/interfaces"
)

// GoOnlyImplementation provides the pure Go vim implementation
// This is used when CGO is not available or on Windows
type GoOnlyImplementation struct {
	engine interfaces.VimEngine
}

// SwitchToCImplementation is a no-op when CGO is not available
func SwitchToCImplementation() {
	// CGO implementation not available, fallback to Go implementation
	SwitchToGoImplementation()
}

