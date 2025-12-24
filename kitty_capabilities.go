package main

import (
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour/ansi"
)

var semverRe = regexp.MustCompile(`\d+\.\d+\.\d+`)

// isActualKittyTerminal returns true only if we're running in the actual
// kitty terminal (not ghostty or other kitty-graphics-compatible terminals).
// This is used to distinguish features only kitty supports (like OSC 66 text sizing)
// from features that other terminals also support (like the graphics protocol).
func isActualKittyTerminal() bool {
	// TERM=xterm-kitty is only set by actual kitty terminal
	return strings.ToLower(os.Getenv("TERM")) == "xterm-kitty"
}

// DetectKittyCapabilities populates kitty-related feature flags on the global app.
// Best-effort: falls back quietly when kitty is unavailable or version is unknown.
// Distinguishes between actual kitty and other terminals (like ghostty) that support
// the kitty graphics protocol but not all kitty-specific features like text sizing.
func (a *App) DetectKittyCapabilities() {
	if !IsTermKitty() {
		a.kitty = false
		return
	}

	a.kitty = true

	// Check if we're in actual kitty vs another kitty-graphics-compatible terminal
	isActualKitty := isActualKittyTerminal()

	version := os.Getenv("VIMANGO_ASSUME_KITTY_VERSION")
	if version == "" && isActualKitty {
		// Only try to get kitty version if we're actually in kitty
		if v, err := kittyBinaryVersion(); err == nil {
			version = v
		}
	}
	a.kittyVersion = version

	// If we can't determine version, enable graphics protocol features
	// (these work in ghostty too) but only enable text sizing for actual kitty
	if version == "" {
		a.kittyPlace = true
		// a.kittyRelative = true // Reserved for future side-by-side image support
		// Only enable text sizing for actual kitty - ghostty doesn't support OSC 66
		a.kittyTextSizing = isActualKitty
	} else {
		if semverAtLeast(version, "0.28.0") {
			a.kittyPlace = true
		}
		// Reserved for future side-by-side image support:
		// if semverAtLeast(version, "0.31.0") {
		// 	a.kittyRelative = true
		// }
		if semverAtLeast(version, "0.40.0") {
			a.kittyTextSizing = true
		}
	}

	// Manual override - escape hatch if text sizing causes issues in a specific terminal
	// (No runtime toggle exists for text sizing; use :toggleimages for image control)
	if os.Getenv("VIMANGO_DISABLE_KITTY_TEXT_SIZING") != "" {
		a.kittyTextSizing = false
	}

	// Initialize image display settings
	a.showImages = (a.kitty && a.kittyPlace) // Enable images if kitty supports placeholders
	a.imageScale = 45                        // Default image width in columns

	// Enable Kitty text sizing in glamour if supported
	if a.kittyTextSizing {
		ansi.SetKittyTextSizingEnabled(true)
	}

	// Clear kitty image cache at startup to avoid stale/evicted textures.
	deleteAllKittyImages()
	kittySessionImageMux.Lock()
	kittySessionImages = make(map[uint32]kittySessionEntry)
	kittySessionImageMux.Unlock()
	kittyIDMap = make(map[string]uint32)
	kittyIDReverse = make(map[uint32]string)
	kittyIDNext = 1
}

func kittyBinaryVersion() (string, error) {
	cmd := exec.Command("kitty", "--version")
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return "", err
	}
	out := buf.String()
	if m := semverRe.FindString(out); m != "" {
		return m, nil
	}
	return strings.TrimSpace(out), nil
}

// semverAtLeast performs a minimal comparison on dotted triples.
// Returns false if either version string is empty or malformed.
func semverAtLeast(have, want string) bool {
	if have == "" || want == "" {
		return false
	}
	hParts := strings.SplitN(have, ".", 3)
	wParts := strings.SplitN(want, ".", 3)
	if len(hParts) != 3 || len(wParts) != 3 {
		return false
	}
	for i := 0; i < 3; i++ {
		cmp := strings.Compare(padNum(hParts[i]), padNum(wParts[i]))
		if cmp > 0 {
			return true
		}
		if cmp < 0 {
			return false
		}
	}
	return true
}

// padNum left-pads numeric strings to 3 chars for naive lexical compare.
func padNum(s string) string {
	if len(s) >= 3 {
		return s
	}
	return strings.Repeat("0", 3-len(s)) + s
}
