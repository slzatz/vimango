package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/disintegration/imaging"
	"google.golang.org/api/drive/v3"
)

func ExtractFileID(url string) (string, error) {
	// Handle gdrive: format (new standardized format)
	if strings.HasPrefix(url, "gdrive:") {
		id := strings.TrimPrefix(url, "gdrive:")
		if isValidGoogleDriveID(id) {
			return id, nil
		}
		return "", errors.New("invalid gdrive: format")
	}

	// Handle full URL format (existing - backward compatible)
	// A regular expression to find the file ID.
	// It looks for a string of letters, numbers, hyphens, and underscores
	// that is between "/d/" and the next "/".
	re := regexp.MustCompile(`/d/([a-zA-Z0-9_-]+)`)

	// Find the submatches in the URL string.
	matches := re.FindStringSubmatch(url)

	// The result of FindStringSubmatch is a slice where:
	// - matches[0] is the full text that matched the expression (e.g., "/d/1Fj-...")
	// - matches[1] is the text captured by the first group `(...)` (the file ID)
	if len(matches) > 1 {
		return matches[1], nil
	}

	return "", errors.New("google drive file ID not found in URL")
}

// isValidGoogleDriveID validates that a string looks like a valid Google Drive file ID
// Google Drive IDs are typically 28-33 characters, alphanumeric with - and _
func isValidGoogleDriveID(id string) bool {
	match, _ := regexp.MatchString(`^[a-zA-Z0-9_-]{20,50}$`, id)
	return match
}

// decodeImageWithOrientation decodes an image and applies EXIF orientation correction.
// Uses the imaging library's AutoOrientation feature to handle phone camera images
// that store orientation in EXIF metadata. Falls back gracefully if no EXIF present.
// HEIC is decoded by the heic_worker subprocess (heic_cgo.go).
func decodeImageWithOrientation(r io.Reader) (image.Image, string, error) {
	// Buffer the input so we can read it multiple times
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, "", err
	}

	// Check for HEIC format first (before standard library detection)
	if IsHEICData(data) {
		if IsHEICAvailable() {
			img, err := GetHEICDecoder().Decode(bytes.NewReader(data))
			if err != nil {
				return nil, "", fmt.Errorf("HEIC decode failed: %v", err)
			}
			// No EXIF pass: libheif applies the HEIC's own rotation/mirror
			// when it decodes. Return as "heic" format - will be converted to
			// PNG for caching
			return img, "heic", nil
		}
		// HEIC not available - return meaningful error
		return nil, "", fmt.Errorf("HEIC format detected but not supported (heic_worker missing or failed to start: build cmd/heic_worker and put it next to vimango)")
	}

	// Detect format using standard library for non-HEIC formats
	_, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, "", err
	}

	// Decode with EXIF orientation using imaging library
	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, "", err
	}

	return img, format, nil
}

// ErrGoogleDriveNotConfigured is returned when trying to load a Google Drive image
// but Google Drive credentials haven't been set up
var ErrGoogleDriveNotConfigured = errors.New("Google Drive not configured - see README.md for setup instructions")

// driveImages downloads images from Google Drive: the signed-in service, and
// the width a download is reduced to before it is cached (preferences.json's
// image_cache_max_width). The TUI's preview and --render-html each have one.
type driveImages struct {
	service  *drive.Service // nil when nobody is signed in
	maxWidth int
}

// loadGoogleImage is the TUI's: the session's Drive service and the
// preference the app holds.
func loadGoogleImage(path string) (img image.Image, imgFmt string, err error) {
	return driveImages{service: app.Session.googleDrive, maxWidth: app.imageCacheMaxWidth}.load(path)
}

// load downloads and decodes one image, named by a gdrive:ID or a Drive URL.
// ErrGoogleDriveNotConfigured when there is no service to ask.
func (d driveImages) load(path string) (img image.Image, imgFmt string, err error) {
	if d.service == nil {
		err = ErrGoogleDriveNotConfigured
		return
	}

	fileID, err := ExtractFileID(path)
	if err != nil {
		return
	}

	resp, err := d.service.Files.Get(fileID).Download()
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = errors.New("Received non-OK response code")
		//return fmt.Errorf("received non-OK response status: %s", resp.Status)
		return
	}

	img, imgFmt, err = decodeImageWithOrientation(resp.Body)
	// Downsize large images for cache efficiency - configurable via preferences.json
	// Use Resize with height=0 to preserve aspect ratio while constraining width
	if err == nil && img != nil && d.maxWidth > 0 && img.Bounds().Dx() > d.maxWidth {
		img = imaging.Resize(img, d.maxWidth, 0, imaging.Lanczos)
	}
	return
}

func loadImage(path string, maxWidth, maxHeight int) (img image.Image, imgFmt string, err error) {
	//fmt.Printf("loadImage: path=%s, maxWidth=%d, maxHeight=%d\n", path, maxWidth, maxHeight)
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	img, imgFmt, err = decodeImageWithOrientation(f)
	if err != nil {
		return
	}
	if img.Bounds().Max.X > maxWidth || img.Bounds().Max.Y > maxHeight {
		//img = imaging.Resize(img, maxWidth, 0, imaging.Lanczos)
		img = imaging.Fit(img, maxWidth, maxHeight, imaging.Lanczos)
	}
	//sess.showOrgMessage("bounds = %v", img.Bounds())
	return
}

func loadWebImage(URL string) (img image.Image, imgFmt string, err error) {
	//Get the response bytes from the url
	response, err := http.Get(URL)
	if err != nil {
		return
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		err = errors.New("Received non 200 response code")
		return
	}
	img, imgFmt, err = decodeImageWithOrientation(response.Body)
	if err != nil {
		return
	}
	if img.Bounds().Max.Y > app.Session.imgSizeY {
		img = imaging.Resize(img, 0, app.Session.imgSizeY, imaging.Lanczos)
	}
	return
}

func IsTmuxScreen() bool {
	TERM := strings.ToLower(strings.TrimSpace(os.Getenv("TERM")))
	return strings.HasPrefix(TERM, "screen")
}

func lcaseEnv(k string) string {
	return strings.ToLower(strings.TrimSpace(os.Getenv(k)))
}

func GetEnvIdentifiers() map[string]string {

	KEYS := []string{"TERM", "TERM_PROGRAM", "LC_TERMINAL"}
	V := make(map[string]string)
	for _, K := range KEYS {
		V[K] = lcaseEnv(K)
	}

	return V
}

// IsTermKitty checks if the terminal supports the kitty graphics protocol.
// This includes Kitty terminal, Ghostty, Rex, and other compatible terminals.
// NOTE: $TERM may be overwritten by tmux
func IsTermKitty() bool {
	V := GetEnvIdentifiers()

	// Kitty terminal
	if V["TERM"] == "xterm-kitty" {
		return true
	}

	// Ghostty terminal - check TERM, TERM_PROGRAM, or ghostty-specific env var
	if strings.Contains(V["TERM"], "ghostty") {
		return true
	}
	if strings.Contains(V["TERM_PROGRAM"], "ghostty") {
		return true
	}
	if os.Getenv("GHOSTTY_RESOURCES_DIR") != "" {
		return true
	}

	// Rex (superlogical's terminal multiplexer) supports the graphics protocol
	// but sets TERM=xterm-256color; it identifies itself only by TERM_PROGRAM.
	if V["TERM_PROGRAM"] == "rex" {
		return true
	}

	return false
}
