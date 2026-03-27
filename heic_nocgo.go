//go:build (!cgo || windows) && !darwin

package main

// No init function needed - isHEICAvailableDefault remains false

// createHEICDecoder provides a stub implementation when CGO is not available
func createHEICDecoder() HEICDecoder {
	return createStubHEICDecoder()
}
