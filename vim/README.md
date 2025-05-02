# Vimango Vim Package

This package provides vim editing functionality for the Vimango application.

## Overview

The vim package uses a flexible adapter layer that supports both:
1. A C implementation (via CGO)
2. A pure Go implementation 

This design allows for a smooth transition from CGO to pure Go while maintaining 
compatibility with the existing application.

## Directory Structure

- `vim/` - Main package with adapter layer and API
  - `interfaces.go` - Core interfaces for vim functionality
  - `api.go` - API functions for application use
  - `adapter_new.go` - Implementation of adapter layer
  - `config.go` - Configuration options
  - `vim.go` - C implementation via CGO
  - `govim/` - Pure Go implementation
    - `buffer.go` - Buffer implementation
    - `engine.go` - Engine implementation
    - `normal_mode_motion.go` - Motion commands
    - `wrapper.go` - Compatibility wrapper

## Using the Vim Package

### Basic Usage

```go
import "github.com/slzatz/vimango/vim"

// Initialize vim
vim.Configure(vim.Config{UseGoImplementation: false})

// Create and use buffers
buffer := vim.NewBuffer(0)
vim.SetCurrentBuffer(buffer)

// Send input
vim.SendInput("Hello, world!")
vim.SendKey("<esc>")

// Get state
position := vim.GetCursorPosition()
mode := vim.GetCurrentMode()
```

### Switching Implementations

```go
// Enable Go implementation
vim.Configure(vim.Config{UseGoImplementation: true})

// Or toggle implementation
vim.ToggleImplementation()

// Check which implementation is being used
if vim.IsUsingGoImplementation() {
    fmt.Println("Using Go implementation")
} else {
    fmt.Println("Using C implementation")
}
```

## Development

When developing or extending this package:

1. Make changes through the adapter layer when possible
2. Implement functionality in both C and Go versions
3. Use the interfaces defined in `interfaces.go`
4. Keep implementation details hidden from the application

See `_govim/DEVELOPMENT.md` for detailed development guidelines.

## Current Status

The adapter layer implementation is in progress. Many vim functions have been
implemented in the pure Go version, but there are still features missing.
The C implementation is still the default and most complete option.

Refer to `_govim/PROGRESS.md` for current implementation status.