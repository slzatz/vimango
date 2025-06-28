//go:build cgo && !windows

package vim

func init() {
	// Override the default - CGO vim implementation is available
	isCGOAvailableDefault = true
}