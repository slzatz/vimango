//go:build cgo && !windows

package main

import (
	"fmt"
	"image"
	"io"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/klippa-app/go-libheif"
	"github.com/klippa-app/go-libheif/library"
)

var (
	heicInitOnce sync.Once
	heicInitErr  error
	devNull      *os.File

	// Guards ShutdownHEICDecoder: libheif.DeInit unconditionally calls
	// Kill on the plugin client, which is nil until a successful Init.
	//
	// Held across the worker's start, so a shutdown that arrives while it
	// is starting (a signal: see killHEICWorkerOnSignal) waits and then
	// kills it, rather than finding nothing to kill and leaving it behind.
	// Once shut down, no worker is started: the process is on its way out.
	heicWorkerMu      sync.Mutex
	heicWorkerStarted bool
	heicShutDown      bool
)

func init() {
	// Suppress debug output from hashicorp/go-plugin used by go-libheif
	os.Setenv("HCLOG_LEVEL", "OFF")

	// Open /dev/null for redirecting stderr during HEIC operations
	var err error
	devNull, err = os.OpenFile("/dev/null", os.O_WRONLY, 0)
	if err != nil {
		devNull = nil
	}
}

// suppressOutput temporarily redirects both stdout and stderr to /dev/null and returns a restore function
func suppressOutput() func() {
	if devNull == nil {
		return func() {} // no-op if /dev/null couldn't be opened
	}

	// Save original stdout and stderr
	originalStdout, err1 := syscall.Dup(int(os.Stdout.Fd()))
	originalStderr, err2 := syscall.Dup(int(os.Stderr.Fd()))
	if err1 != nil || err2 != nil {
		if err1 == nil {
			syscall.Close(originalStdout)
		}
		if err2 == nil {
			syscall.Close(originalStderr)
		}
		return func() {}
	}

	// Redirect stdout and stderr to /dev/null
	syscall.Dup2(int(devNull.Fd()), int(os.Stdout.Fd()))
	syscall.Dup2(int(devNull.Fd()), int(os.Stderr.Fd()))

	// Return restore function
	return func() {
		syscall.Dup2(originalStdout, int(os.Stdout.Fd()))
		syscall.Dup2(originalStderr, int(os.Stderr.Fd()))
		syscall.Close(originalStdout)
		syscall.Close(originalStderr)
	}
}

// CGOHEICDecoder provides go-libheif-based HEIC decoding
type CGOHEICDecoder struct {
	initialized bool
}

// createHEICDecoder creates a new CGO-based HEIC decoder
func createHEICDecoder() HEICDecoder {
	decoder := &CGOHEICDecoder{}

	// Initialize go-libheif once
	heicInitOnce.Do(func() {
		// Determine worker binary path
		// Worker should be built and placed alongside main binary
		execPath, err := os.Executable()
		if err != nil {
			heicInitErr = fmt.Errorf("failed to get executable path: %v", err)
			return
		}

		workerPath := filepath.Join(filepath.Dir(execPath), "heic_worker")

		// Check if worker exists
		if _, err := os.Stat(workerPath); os.IsNotExist(err) {
			// Try current directory as fallback
			workerPath = "./heic_worker"
			if _, err := os.Stat(workerPath); os.IsNotExist(err) {
				heicInitErr = fmt.Errorf("HEIC worker binary not found (looked in %s and ./heic_worker)", filepath.Dir(execPath))
				return
			}
		}

		// Initialize libheif with worker (suppress debug output)
		restore := suppressOutput()
		defer restore()

		config := libheif.Config{
			LibraryConfig: library.Config{
				Command: library.Command{
					BinPath: workerPath,
					Args:    []string{},
				},
			},
		}

		heicWorkerMu.Lock()
		defer heicWorkerMu.Unlock()
		if heicShutDown {
			heicInitErr = fmt.Errorf("the HEIC decoder has been shut down")
			return
		}
		if err := libheif.Init(config); err != nil {
			heicInitErr = fmt.Errorf("failed to initialize libheif: %v", err)
			return
		}
		heicWorkerStarted = true

		decoder.initialized = true
	})

	return decoder
}

// shutdownHEICDecoder kills the heic_worker plugin subprocess. See
// ShutdownHEICDecoder in heic.go for why this is not automatic.
//
// One-shot by design: go-libheif v1.3.0's DeInit leaves its
// isInitialized flag set to true (an upstream bug — it should clear
// it), so a later Init returns early and decoding would fail against a
// killed worker. Callers must treat this as a shutdown, not a way to
// recycle the worker.
func shutdownHEICDecoder() {
	heicWorkerMu.Lock()
	defer heicWorkerMu.Unlock()
	heicShutDown = true
	if !heicWorkerStarted {
		return
	}
	heicWorkerStarted = false

	// go-plugin chatters on stderr as it tears the client down, and
	// stdout here is the rendered HTML.
	restore := suppressOutput()
	defer restore()
	libheif.DeInit()
}

func (d *CGOHEICDecoder) IsAvailable() bool {
	return d.initialized && heicInitErr == nil
}

func (d *CGOHEICDecoder) Decode(r io.Reader) (image.Image, error) {
	if !d.IsAvailable() {
		return nil, fmt.Errorf("HEIC decoder not available: %v", heicInitErr)
	}

	// Suppress debug output during decoding
	restore := suppressOutput()
	defer restore()

	// Use libheif to decode the image
	img, err := libheif.DecodeImage(r)
	if err != nil {
		return nil, fmt.Errorf("failed to decode HEIC image: %v", err)
	}

	return img, nil
}
