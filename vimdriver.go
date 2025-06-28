package main

import (
	"fmt"
	"runtime"

	"github.com/slzatz/vimango/vim"
)

// VimDriver represents the available Vim implementation options
type VimDriver int

const (
	VimDriverGo  VimDriver = iota // Pure Go implementation
	VimDriverCGO                  // CGO implementation (libvim)
)

// VimConfig holds the configuration for Vim implementation selection
type VimConfig struct {
	Driver VimDriver
}

// GetVimDriverDisplayName returns a human-readable name for the driver
func (cfg *VimConfig) GetVimDriverDisplayName() string {
	switch cfg.Driver {
	case VimDriverCGO:
		return "libvim (CGO)"
	case VimDriverGo:
		return "govim (Pure Go)"
	default:
		return "govim (Pure Go)"
	}
}

// DetermineVimDriver determines which Vim implementation to use based on:
// 1. Command line arguments
// 2. Platform constraints (Windows only supports pure Go)
// 3. Build constraints (CGO availability)
func DetermineVimDriver(args []string) *VimConfig {
	cfg := &VimConfig{
		Driver: VimDriverCGO, // Default to CGO if available
	}

	// On Windows, force pure Go implementation
	if runtime.GOOS == "windows" {
		cfg.Driver = VimDriverGo
		return cfg
	}

	// If CGO vim is not available, force pure Go
	if !vim.IsCGOAvailable() {
		cfg.Driver = VimDriverGo
		return cfg
	}

	// Check for --go-vim flag (forces pure Go)
	for _, arg := range args {
		if arg == "--go-vim" {
			cfg.Driver = VimDriverGo
			return cfg
		}
	}

	// Default behavior: use CGO implementation if available
	return cfg
}

// ShouldUseGoVim returns true if the Go vim implementation should be used
func (cfg *VimConfig) ShouldUseGoVim() bool {
	return cfg.Driver == VimDriverGo
}

// LogVimDriverChoice logs which Vim implementation is being used
func LogVimDriverChoice(cfg *VimConfig) {
	fmt.Printf("Using Vim implementation: %s\n", cfg.GetVimDriverDisplayName())
}