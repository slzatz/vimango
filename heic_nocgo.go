//go:build windows

package main

// No init function needed - isHEICAvailableDefault remains false

// createHEICDecoder provides a stub implementation when CGO is not available
func createHEICDecoder() HEICDecoder {
	return createStubHEICDecoder()
}

// shutdownHEICDecoder is a no-op: the stub decoder holds nothing open.
func shutdownHEICDecoder() {}
