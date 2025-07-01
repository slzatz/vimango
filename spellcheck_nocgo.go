//go:build !cgo || windows

package main

// No init function needed - isSpellCheckAvailableDefault remains false

// createCGOSpellChecker provides a stub implementation when CGO is not available
func createCGOSpellChecker() SpellChecker {
	// CGO spell checker is not available, return stub implementation
	return createStubSpellChecker()
}