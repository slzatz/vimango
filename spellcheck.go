package main

import "fmt"

// SpellChecker provides an interface for spell checking functionality
type SpellChecker interface {
	// IsAvailable returns true if spell checking is available on this platform
	IsAvailable() bool
	
	// Spell checks if a word is spelled correctly
	Spell(word string) bool
	
	// Suggest returns spelling suggestions for a misspelled word
	Suggest(word string) []string
}

// Global spell checker instance
var globalSpellChecker SpellChecker

// GetSpellChecker returns the global spell checker instance
func GetSpellChecker() SpellChecker {
	if globalSpellChecker == nil {
		globalSpellChecker = createSpellChecker()
	}
	return globalSpellChecker
}

// IsSpellCheckAvailable returns true if spell checking is available
func IsSpellCheckAvailable() bool {
	return GetSpellChecker().IsAvailable()
}

// CheckSpelling checks if a word is spelled correctly
func CheckSpelling(word string) bool {
	checker := GetSpellChecker()
	if !checker.IsAvailable() {
		return true // If spell check isn't available, assume words are correct
	}
	return checker.Spell(word)
}

// GetSpellingSuggestions returns spelling suggestions for a word
func GetSpellingSuggestions(word string) []string {
	checker := GetSpellChecker()
	if !checker.IsAvailable() {
		return []string{} // Return empty suggestions if not available
	}
	return checker.Suggest(word)
}

// ShowSpellCheckNotAvailableMessage displays a user-friendly message
func ShowSpellCheckNotAvailableMessage() string {
	return fmt.Sprintf("%sSpell check not available on this platform%s", RED_BG, RESET)
}

// Default implementation that will be overridden by build-specific files
var isSpellCheckAvailableDefault = false

// createSpellChecker creates the appropriate spell checker implementation
// This function will be overridden by build-specific files
func createSpellChecker() SpellChecker {
	if isSpellCheckAvailableDefault {
		return createCGOSpellChecker()
	}
	return createStubSpellChecker()
}

// createCGOSpellChecker will be implemented in build-specific files

func createStubSpellChecker() SpellChecker {
	return &StubSpellChecker{}
}

// StubSpellChecker provides a no-op implementation for platforms without spell check
type StubSpellChecker struct{}

func (s *StubSpellChecker) IsAvailable() bool {
	return false
}

func (s *StubSpellChecker) Spell(word string) bool {
	return true // Assume all words are correct when spell check is not available
}

func (s *StubSpellChecker) Suggest(word string) []string {
	return []string{} // No suggestions available
}