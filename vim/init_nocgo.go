//go:build !cgo && !windows

package vim

import "fmt"

// InitializeVim sets up the vim engine when CGO is not available
// Forces Go implementation since CGO implementation is not compiled in
func InitializeVim(useGoImplementation bool, argc int) {
    // When CGO is not available, always use Go implementation
    SwitchToGoImplementation()
    Engine = GetEngineWrapper()
    
    if !useGoImplementation {
        // User requested CGO but it's not available - log a message
        fmt.Printf("CGO vim implementation not available, using Go implementation\n")
    } else {
        fmt.Printf("Using Go vim implementation\n")
    }
}