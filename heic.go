package main

import (
	"bytes"
	"fmt"
	"image"
	"io"
)

// HEICDecoder provides an interface for HEIC decoding functionality
type HEICDecoder interface {
	// IsAvailable returns true if HEIC decoding is available on this platform
	IsAvailable() bool

	// Decode decodes HEIC image data and returns a standard Go image
	Decode(r io.Reader) (image.Image, error)
}

// Global HEIC decoder instance
var globalHEICDecoder HEICDecoder

// Default availability flag - overridden by heic_cgo.go init()
var isHEICAvailableDefault = false

// GetHEICDecoder returns the global HEIC decoder instance
func GetHEICDecoder() HEICDecoder {
	if globalHEICDecoder == nil {
		globalHEICDecoder = createHEICDecoder()
	}
	return globalHEICDecoder
}

// IsHEICAvailable returns true if HEIC decoding is available
func IsHEICAvailable() bool {
	return GetHEICDecoder().IsAvailable()
}

// HEIC magic bytes detection
// HEIC/HEIF files have "ftyp" at offset 4, followed by brand identifier
// Common brands: "heic", "heix", "hevc", "hevx", "mif1", "msf1"
var heicBrands = [][]byte{
	[]byte("heic"),
	[]byte("heix"),
	[]byte("hevc"),
	[]byte("hevx"),
	[]byte("mif1"),
	[]byte("msf1"),
}

// IsHEICData checks if the given data appears to be HEIC/HEIF format
// Returns true if the data has HEIC magic bytes
func IsHEICData(data []byte) bool {
	// Minimum HEIC header size
	if len(data) < 12 {
		return false
	}

	// Check for "ftyp" at offset 4
	if !bytes.Equal(data[4:8], []byte("ftyp")) {
		return false
	}

	// Check brand identifier at offset 8
	brandBytes := data[8:12]
	for _, brand := range heicBrands {
		if bytes.Equal(brandBytes, brand) {
			return true
		}
	}

	return false
}

// ShowHEICNotAvailableMessage returns a user-friendly message
func ShowHEICNotAvailableMessage() string {
	return fmt.Sprintf("%sHEIC format not supported in this build (requires CGO)%s", YELLOW_BG, RESET)
}

// StubHEICDecoder provides a no-op implementation for platforms without HEIC support
type StubHEICDecoder struct{}

func (s *StubHEICDecoder) IsAvailable() bool {
	return false
}

func (s *StubHEICDecoder) Decode(r io.Reader) (image.Image, error) {
	return nil, fmt.Errorf("HEIC decoding not available in this build (requires CGO)")
}

func createStubHEICDecoder() HEICDecoder {
	return &StubHEICDecoder{}
}
