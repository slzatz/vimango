//go:build darwin

package main

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

var (
	pillowInitOnce sync.Once
	pillowPython   string // path to .venv/bin/python3
	pillowScript   string // path to heic_convert.py
	pillowInitErr  error
)

func init() {
	isHEICAvailableDefault = true
}

// PillowHEICDecoder uses pillow-heif via a Python subprocess
type PillowHEICDecoder struct{}

func createHEICDecoder() HEICDecoder {
	pillowInitOnce.Do(func() {
		// Look for .venv and heic_convert.py relative to the executable
		execPath, err := os.Executable()
		if err != nil {
			pillowInitErr = fmt.Errorf("failed to get executable path: %v", err)
			return
		}
		execDir := filepath.Dir(execPath)

		// Try next to executable first, then current directory
		for _, base := range []string{execDir, "."} {
			py := filepath.Join(base, ".venv", "bin", "python3")
			script := filepath.Join(base, "heic_convert.py")
			if _, err := os.Stat(py); err == nil {
				if _, err := os.Stat(script); err == nil {
					pillowPython = py
					pillowScript = script
					return
				}
			}
		}
		pillowInitErr = fmt.Errorf("HEIC support requires .venv with pillow-heif and heic_convert.py")
	})

	return &PillowHEICDecoder{}
}

func (d *PillowHEICDecoder) IsAvailable() bool {
	return pillowInitErr == nil && pillowPython != ""
}

func (d *PillowHEICDecoder) Decode(r io.Reader) (image.Image, error) {
	if !d.IsAvailable() {
		return nil, fmt.Errorf("pillow HEIC decoder not available: %v", pillowInitErr)
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("failed to read HEIC data: %v", err)
	}

	cmd := exec.Command(pillowPython, pillowScript)
	cmd.Stdin = bytes.NewReader(data)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pillow HEIC decode failed: %v: %s", err, stderr.String())
	}

	img, err := png.Decode(&stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to decode PNG from pillow output: %v", err)
	}

	return img, nil
}
