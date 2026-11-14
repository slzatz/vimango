package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/slzatz/vimango/terminal"
	"github.com/slzatz/vimango/vim"
)

type Location int

const (
	BL Location = iota //Bottom Left
	BR
	TL
	TR
)

// dbConfig holds application configuration loaded from config.json.
// Sensitive credentials can be overridden via environment variables:
//   - VIMANGO_PG_PASSWORD: overrides postgres.password
//   - VIMANGO_PG_SSL_MODE: overrides postgres.ssl_mode (disable, require, verify-ca, verify-full)
//   - VIMANGO_PG_SSL_CA_CERT: overrides postgres.ssl_ca_cert (path to CA certificate)
type dbConfig struct {
	Postgres struct {
		Host      string `json:"host"`
		Port      string `json:"port"`
		User      string `json:"user"`
		Password  string `json:"password"` // Can be overridden by VIMANGO_PG_PASSWORD env var
		DB        string `json:"db"`
		Test      string `json:"test"`
		SSLMode   string `json:"ssl_mode"`    // disable, require, verify-ca, verify-full (overridden by VIMANGO_PG_SSL_MODE)
		SSLCACert string `json:"ssl_ca_cert"` // Path to CA certificate (overridden by VIMANGO_PG_SSL_CA_CERT)
	} `json:"postgres"`

	// An older config.json also has "fts_db" here: the search index was a
	// second file until 2026-11-10. The key is ignored.
	Sqlite3 struct {
		DB string `json:"db"`
	} `json:"sqlite3"`

	Options struct {
		Type  string `json:"type"`
		Title string `json:"title"`
	} `json:"options"`

	Chroma struct {
		Style string `json:"style"`
	} `json:"chroma"`

	Glamour struct {
		Style string `json:"style"`
	} `json:"glamour"`

	// "webview" is historical: the TUI's webview window is gone (2026-11-08),
	// and these now style only --render-html, the hybrid preview pane. The
	// key keeps its name so existing config.json files still apply.
	Webview struct {
		// Accent colors the blockquote bar and list markers in rendered
		// HTML previews. "" → default blue, "none"/"off" → no accent
		// (they take the surrounding text color), or any #rgb/#rrggbb.
		Accent string `json:"accent"`
		// CodeAccent colors inline code and code blocks (fenced or
		// indented). Same vocabulary as Accent, but a separate knob:
		// code and the blockquote/marker accent are different kinds of
		// emphasis and are meant to be retunable apart. "" → whatever
		// Accent resolved to, so out of the box they match.
		CodeAccent string `json:"code_accent"`
	} `json:"webview"`
}

// Preferences holds user UI preferences that persist across sessions
type Preferences struct {
	ImageScale         int `json:"image_scale"`           // Image width in columns (10-100)
	EdPct              int `json:"ed_pct"`                // Editor percentage (1-99)
	ImageCacheMaxWidth int `json:"image_cache_max_width"` // Max pixel width for cached images (default 800)
}

// validateGlamourStyle checks if a glamour style file exists and returns an error if not.
// This should be called at application startup.
func validateGlamourStyle() error {
	// Try config value first
	if app.Config != nil && app.Config.Glamour.Style != "" {
		if _, err := os.Stat(app.Config.Glamour.Style); err == nil {
			return nil // Found configured style
		}
	}

	// Try default.json
	if _, err := os.Stat("default.json"); err == nil {
		return nil // Found default style
	}

	// Neither file exists
	configuredStyle := "not specified"
	if app.Config != nil && app.Config.Glamour.Style != "" {
		configuredStyle = app.Config.Glamour.Style
	}
	return fmt.Errorf("glamour style files not found:\n  Configured style: %s\n  Fallback style: default.json\nPlease ensure at least one of these files exists", configuredStyle)
}

// getGlamourStylePath is the glamour style file the organizer's preview is
// rendered with: glamourStylePath of the style config.json names.
func getGlamourStylePath() string {
	configured := ""
	if app.Config != nil {
		configured = app.Config.Glamour.Style
	}
	return glamourStylePath(configured)
}

// glamourStylePath returns the path to the glamour style file with fallback logic:
// 1. Try the configured style, from config.json
// 2. Try default.json
// This function assumes validateGlamourStyle() has already been called at startup.
func glamourStylePath(configured string) string {
	// Try config value first
	if configured != "" {
		if _, err := os.Stat(configured); err == nil {
			return configured
		}
	}

	// Try default.json
	if _, err := os.Stat("default.json"); err == nil {
		return "default.json"
	}

	// This should not happen if validateGlamourStyle() was called at startup
	// Return empty string as last resort
	return ""
}

var z0 = struct{}{}

var sortColumns = map[string]struct{}{
	"added":    z0,
	"modified": z0,
	"priority": z0,
}

type Mode int

const (
	NORMAL      Mode = iota // just seeing this after an escape
	NORMAL_BUSY             // Replace and almost any other keystroke in normal
	INSERT
	COMMAND_LINE // only in organizer mode
	EX_COMMAND   // only in editor mode
	VISUAL_LINE  // only editor mode
	VISUAL
	VISUAL_BLOCK // only editor mode
	SEARCH       // only editor mode
	//FIND            // only organizer mode
	SPELLING        // choosing a z= suggestion by number (spellcheck.go)
	NAVIGATE_NOTICE // only in organizer mode
	HELP            // organizer and editor mode
	CONTAINER       // overlay for choosing folder/context
	LINKS           // only in organizer mode
	PENDING
	OTHER // Just in case
)

var modeMap = map[vim.Mode]Mode{
	vim.ModeNormal:     NORMAL, // seems to be only after an escape
	vim.ModeVisual:     VISUAL,
	vim.ModeOpPending:  PENDING,
	vim.ModeCmdline:    SEARCH, // also COMMAND
	vim.ModeInsert:     INSERT,
	vim.ModeNormalBusy: NORMAL_BUSY, // just about any keystroke when in NORMAL mode
}

// v -> 118; V -> 86; ctrl-v -> 22
var visualModeMap = map[int]Mode{
	22:  VISUAL_BLOCK,
	86:  VISUAL_LINE,
	118: VISUAL,
}

const (
	TZ_OFFSET            = 4
	LINKED_NOTE_HEIGHT   = 20
	TOP_MARGIN           = 1
	MAX                  = 500
	TIME_COL_WIDTH       = 18
	IMAGE_MARKER_WIDTH   = 2
	IMAGE_MARKER_AGE_GAP = 1
	LEFT_MARGIN          = 1
	LEFT_MARGIN_OFFSET   = 4

	BASE_DATE string = "1970-01-01 00:00"

	RESET string = "\x1b[0m"
	BOLD  string = "\x1b[1m"
	// IMAGE_MARKER_SYMBOL is printed next to the age column when a note has image links
	IMAGE_MARKER_SYMBOL = "\uf03e"
)

func ctrlKey(b byte) int {
	return int(b & 0x1f)
}

var googleDriveRegex = regexp.MustCompile(`!\[([^\]]*)\]\((https://drive\.google\.com/file/d/[^)]+)\)`)
var googleDriveShortRegex = regexp.MustCompile(`!\[([^\]]*)\]\((gdrive:[a-zA-Z0-9_-]+)\)`)

type Row struct {
	id       int
	tid      int
	title    string
	ftsTitle string
	star     bool
	deleted  bool
	archived bool
	hasImage bool
	//modified  string
	sort string

	// below not in db
	dirty  bool
	marked bool
}

func containsGoogleDriveImage(note string) bool {
	return googleDriveRegex.MatchString(note) || googleDriveShortRegex.MatchString(note)
}

// ConvertGoogleDriveURLsToShort converts all Google Drive URLs in markdown to gdrive:ID format
// Returns the converted markdown and the count of URLs converted
func ConvertGoogleDriveURLsToShort(markdown string) (string, int) {
	re := regexp.MustCompile(`!\[([^\]]*)\]\((https://drive\.google\.com/file/d/([a-zA-Z0-9_-]+)[^)]*)\)`)

	count := 0
	result := re.ReplaceAllStringFunc(markdown, func(match string) string {
		submatches := re.FindStringSubmatch(match)
		if len(submatches) >= 4 {
			altText := submatches[1]
			fileID := submatches[3]
			count++
			return fmt.Sprintf("![%s](gdrive:%s)", altText, fileID)
		}
		return match
	})

	return result, count
}

// Keys that produce no character arrive from the key reader as synthetic codes
// well above any rune a terminal will send. These are aliases for the reader's
// own constants rather than a second hand-maintained copy: the two blocks used
// to be kept in step by hand and had already drifted -- terminal.KeyF1 and
// KeyF2 landed on codes main had named NOP and SHIFT_TAB, and F3 through Ins
// had no name here at all.
//
// What becomes of one on its way to vim is pane.SendToVim.
const (
	BACKSPACE = 127 // a real byte, unlike the codes below

	ARROW_LEFT  = terminal.KeyArrowLeft
	ARROW_RIGHT = terminal.KeyArrowRight
	ARROW_UP    = terminal.KeyArrowUp
	ARROW_DOWN  = terminal.KeyArrowDown
	DEL_KEY     = terminal.KeyDelete
	HOME_KEY    = terminal.KeyHome
	END_KEY     = terminal.KeyEnd
	PAGE_UP     = terminal.KeyPageUp
	PAGE_DOWN   = terminal.KeyPageDown
)

func (m Mode) String() string {
	return [...]string{
		"NORMAL",
		"NORMAL BUSY",
		"INSERT",
		"COMMAND LINE",
		"EX COMMAND",
		"VISUAL LINE",
		"VISUAL",
		"VISUAL BLOCK",
		"SEARCH",
		//"FIND",
		"SPELLING",
		"NAVIGATE_NOTICE",
		"HELP",
		"CONTAINER",
		"LINKS",
		"PENDING",
		"OTHER",
	}[m]
}

type View int

const (
	TASK View = iota
	CONTEXT
	FOLDER
	KEYWORD
	//SYNC_LOG_VIEW
)

func (v View) String() string {
	return [...]string{
		"task",
		"context",
		"folder",
		"keyword",
	}[v]
}

// type TaskView int
const (
	BY_CONTEXT = iota
	BY_FOLDER
	BY_KEYWORD
	BY_JOIN
	BY_RECENT
	BY_FIND
)

func timeDelta(t string) string {
	var t0 time.Time
	if strings.Contains(t, "T") {
		t0, _ = time.Parse("2006-01-02T15:04:05Z", t)
	} else {
		t0, _ = time.Parse("2006-01-02 15:04:05", t)
	}
	diff := time.Since(t0)

	diff = diff / 1000000000
	if diff <= 120 {
		return fmt.Sprintf("%d seconds ago", diff)
	} else if diff <= 60*120 {
		return fmt.Sprintf("%d minutes ago", diff/60) // <120 minutes we report minute
	} else if diff <= 48*60*60 {
		return fmt.Sprintf("%d hours ago", diff/3600) // <48 hours report hours
	} else if diff <= 24*60*60*60 {
		return fmt.Sprintf("%d days ago", diff/3600/24) // <60 days report days
	} else if diff <= 24*30*24*60*60 {
		return fmt.Sprintf("%d months ago", diff/3600/24/30) // <24 months rep
	} else {
		return fmt.Sprintf("%d years ago", diff/3600/24/30/12)
	}
}
