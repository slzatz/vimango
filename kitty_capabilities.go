package main

import (
	"bytes"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var semverRe = regexp.MustCompile(`\d+\.\d+\.\d+`)

// DetectKittyCapabilities populates kitty-related feature flags on the global app.
// Best-effort: falls back quietly when kitty is unavailable or version is unknown.
func (a *App) DetectKittyCapabilities() {
	if !IsTermKitty() {
		a.kitty = false
		return
	}

	a.kitty = true

	version := os.Getenv("VIMANGO_ASSUME_KITTY_VERSION")
	if version == "" {
		if v, err := kittyBinaryVersion(); err == nil {
			version = v
		}
	}
	a.kittyVersion = version

	// If we can't determine version but know we're in kitty, assume modern enough.
	if version == "" {
		a.kittyPlace = true
		a.kittyRelative = true
	} else {
		if semverAtLeast(version, "0.28.0") {
			a.kittyPlace = true
		}
		if semverAtLeast(version, "0.31.0") {
			a.kittyRelative = true
		}
	}

	// Manual overrides
	if os.Getenv("VIMANGO_ENABLE_KITTY_PLACEHOLDERS") != "" {
		a.kittyPlace = true
	}
	if os.Getenv("VIMANGO_ENABLE_KITTY_RELATIVE") != "" {
		a.kittyRelative = true
	}

	// Initialize image display settings
	a.showImages = (a.kitty && a.kittyPlace) // Enable images if kitty supports placeholders
	a.imageScale = 45                        // Default image width in columns

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
