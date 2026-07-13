//go:build !spell || !cgo || windows

package main

// No init function needed - isSpellCheckAvailableDefault remains false

// createCGOSpellChecker provides a stub implementation when the hunspell
// spell checker is not compiled in (no `spell` build tag, no CGO, or
// windows). <leader>sp / <leader>su report "not available".
func createCGOSpellChecker() SpellChecker {
	return createStubSpellChecker()
}