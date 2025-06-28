package vim

// This file provides build-independent functions that need to be available
// in all builds regardless of CGO availability

// Default implementation detection - this will be overridden by build-specific files
var isCGOAvailableDefault = false

// IsCGOAvailable returns whether the CGO vim implementation is available
// This function is implemented differently based on build tags
func IsCGOAvailable() bool {
	return isCGOAvailableDefault
}