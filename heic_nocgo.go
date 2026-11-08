//go:build !cgo || windows

package main

// The complement of heic_cgo.go. Neither case is a supported build (libvim
// needs cgo; Windows was dropped), so this only keeps the HEIC files
// complete for every build constraint.

// createHEICDecoder returns the stub: there is no heic_worker to drive.
func createHEICDecoder() HEICDecoder {
	return createStubHEICDecoder()
}

// shutdownHEICDecoder is a no-op: the stub decoder holds nothing open.
func shutdownHEICDecoder() {}
