// heic_worker is a subprocess binary required by go-libheif for safe HEIC decoding.
// The worker architecture isolates libheif C library crashes from the main application.
// Build this binary and place it alongside the main vimango executable.
//
// Build: cd cmd/heic_worker && go build -o ../../heic_worker
package main

import (
	"github.com/klippa-app/go-libheif/library/plugin"
)

func main() {
	plugin.StartPlugin()
}
