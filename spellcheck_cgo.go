//go:build cgo && !windows

package main

import "github.com/slzatz/vimango/hunspell"

func init() {
	// Override the default - spell check is available with CGO
	isSpellCheckAvailableDefault = true
}

// CGOSpellChecker provides hunspell-based spell checking
type CGOSpellChecker struct {
	hunspell *hunspell.Hunhandle
}

// createCGOSpellChecker creates a new CGO-based spell checker
func createCGOSpellChecker() SpellChecker {
	// Use the same paths as the original implementation
	h := hunspell.Hunspell("/usr/share/hunspell/en_US.aff", "/usr/share/hunspell/en_US.dic")
	return &CGOSpellChecker{hunspell: h}
}

func (c *CGOSpellChecker) IsAvailable() bool {
	return c.hunspell != nil
}

func (c *CGOSpellChecker) Spell(word string) bool {
	if c.hunspell == nil {
		return true // Fallback: assume words are correct if hunspell failed to initialize
	}
	return c.hunspell.Spell(word)
}

func (c *CGOSpellChecker) Suggest(word string) []string {
	if c.hunspell == nil {
		return []string{} // Return empty suggestions if hunspell failed to initialize
	}
	return c.hunspell.Suggest(word)
}