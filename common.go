package main

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Location int

const (
	BL Location = iota //Bottom Left
	BR
	TL
	TR
)

type Position struct {
	rowNum int
	start  int
	end    int
}

type Window interface {
	drawText()
	drawFrame()
	drawStatusBar()
}

type dbConfig struct {
	Postgres struct {
		Host     string `json:"host"`
		Port     string `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
		DB       string `json:"db"`
		Test     string `json:"test"`
	} `json:"postgres"`

	Sqlite3 struct {
		DB     string `json:"db"`
		FTS_DB string `json:"fts_db"`
	} `json:"sqlite3"`

	Options struct {
		Type  string `json:"type"`
		Title string `json:"title"`
	} `json:"options"`

	Chroma struct {
		Style string `json:"style"`
	} `json:"chroma"`

	Claude struct {
		ApiKey string `json:"api_key"`
	} `json:"claude"`
}

var z0 = struct{}{}

var Languages = map[string]string{
	"golang": "go",
	"go":     "go",
	"cpp":    "cpp",
	"c++":    "cpp",
	"python": "python",
}

var sortColumns = map[string]struct{}{
	"added":    z0,
	"modified": z0,
	"priority": z0,
}

/*
var navKeys = map[int]struct{}{
	ARROW_UP:    z0,
	ARROW_DOWN:  z0,
	ARROW_LEFT:  z0,
	ARROW_RIGHT: z0,
	'j':         z0,
	'k':         z0,
	'h':         z0,
	'l':         z0,
	'g':         z0,
	'G':         z0,
}
*/

var termcodes = map[int]string{
	ARROW_UP:    "<up>", // could also do with vimKey as <Right>, <Left>, <Up>, <Down>
	ARROW_DOWN:  "<down>",
	ARROW_RIGHT: "<right>",
	ARROW_LEFT:  "<left>",
	BACKSPACE:   "<bs>", //? also works "\x08"
	HOME_KEY:    "<home>",
	DEL_KEY:     "<del>",
	PAGE_UP:     "<pageup>",
	PAGE_DOWN:   "<pagedown>",
	//0x6:         "<c-f>",
	//<end> <tab> <insert> <cr> or <enter> <f1> ... <f12>
}

var Lsps = map[string]string{
	"go":  "gopls",
	"cpp": "clangd",
	"py":  "python-language-server",
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
	PREVIEW         // only editor mode - for previewing markdown
	VIEW_LOG        // only in editor mode - for debug viewing of vim message hx
	SPELLING        // this mode recognizes 'z='
	NAVIGATE_NOTICE // only in organizer mode
	LINKS           // only in organizer mode
	PENDING
	OTHER // Just in case
)

var modeMap = map[int]Mode{
	1:   NORMAL, // seems to be only after an escape
	2:   VISUAL, //VISUAL_MODE,
	4:   PENDING,
	8:   SEARCH, // Also COMMAND
	16:  INSERT,
	257: NORMAL_BUSY, // just about any keystroke when in NORMAL mode
}

// v -> 118; V -> 86; ctrl-v -> 22
var visualModeMap = map[int]Mode{
	22:  VISUAL_BLOCK,
	86:  VISUAL_LINE,
	118: VISUAL,
}

const (
	TZ_OFFSET          = 4
	LINKED_NOTE_HEIGHT = 20
	TOP_MARGIN         = 1
	MAX                = 500
	TIME_COL_WIDTH     = 18
	LEFT_MARGIN        = 1
	LEFT_MARGIN_OFFSET = 4

	BASE_DATE string = "1970-01-01 00:00"

	RESET string = "\x1b[0m"
	BOLD  string = "\x1b[1m"

	maxUint = ^uint(0)
	maxInt  = int(maxUint >> 1)
)

func ctrlKey(b byte) int {
	return int(b & 0x1f)
}

func truncate(s string, length int) string {
	if len(s) > length {
		return s[:length] + "..."
	} else {
		return s
	}
}

func tc(s string, l int, b bool) string {
	if len(s) > l {
		e := ""
		if b {
			e = "..."
		}
		return s[:l] + e
	} else {
		return s
	}
}

type Row struct {
	id       int
	tid      int
	title    string
	ftsTitle string
	star     bool
	deleted  bool
	archived bool
	//modified  string
	sort string

	// below not in db
	dirty  bool
	marked bool
}

type AltRow struct {
	id    int
	title string
	star  bool
}

// used in synchronize and getEntryInfo
type NewEntry struct {
	id          int
	tid         int
	title       string
	folder_tid  int
	context_tid int
	star        bool
	note        sql.NullString //string
	added       string
	archived    bool
	deleted     bool
	modified    string
}

type Entry struct {
	id          int
	tid         int
	title       string
	folder_tid  int
	context_tid int
	star        bool
	note        sql.NullString //string
	added       sql.NullString
	completed   sql.NullString
	deleted     bool
	modified    string
}
type serverEntry struct {
	id         int
	title      string
	folder_id  int
	context_id int
	star       bool
	note       sql.NullString //string
	added      sql.NullString //string
	completed  sql.NullString //sql.NullTime since sqlite doesn't have datetime type
	deleted    bool
	modified   string
}

type Container struct {
	id       int
	tid      int
	title    string
	star     bool
	deleted  bool
	modified string
	count    int
}

// type outlineKey int
const (
	BACKSPACE  = iota + 127
	ARROW_LEFT = iota + 999 //would have to be < 127 to be chars
	ARROW_RIGHT
	ARROW_UP
	ARROW_DOWN
	DEL_KEY
	HOME_KEY
	END_KEY
	PAGE_UP
	PAGE_DOWN
	NOP
	SHIFT_TAB
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
		"PREVIEW",
		"VIEW LOG",
		"SPELLING",
		"NAVIGATE_NOTICE",
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

const leader = " "

func getStringInBetween(str string, start string, end string) string {
	k, v, ok := strings.Cut(str, start)
	if !ok {
		return ""
	}
	k, v, ok = strings.Cut(v, start)
	if !ok {
		return ""
	}
	return k
}

func extractFilePath(input string) string {
	// Normalize line breaks
	normalized := strings.ReplaceAll(input, "\n", "")

	// Split by the arrow
	parts := strings.Split(normalized, "→")
	if len(parts) < 2 {
		return ""
	}
	location := strings.Split(parts[1], "|")
	// Get everything after the arrow
	if len(location) < 2 {
		return ""
	} else {
		return location[1]
	}
}

func extractFilePath_(input string) string {
	// Normalize line breaks
	normalized := strings.ReplaceAll(input, "\n", "")

	// Split by the arrow
	parts := strings.Split(normalized, "→")
	if len(parts) < 2 {
		return ""
	}

	// Get everything after the arrow
	afterArrow := strings.TrimSpace(parts[1])

	// Strip ANSI escape codes
	ansiEscapeRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	cleanedText := ansiEscapeRegex.ReplaceAllString(afterArrow, "")

	// Enhanced regex for complex paths and URLs
	// This handles:
	// 1. URLs with multiple path segments
	// 2. Wikipedia-style URLs with dimensions in the path
	// 3. Traditional file paths and simple URLs
	pathRegex := regexp.MustCompile(`((?:https?://)[^\s]+?/[^\s]+?\.(jpg|jpeg|png|gif|bmp|tiff|webp)(?:/[^\s]+?\.(jpg|jpeg|png|gif|bmp|tiff|webp))?(?:\?[^\s]*)?|(?:/|[A-Za-z]:\\|[A-Za-z0-9_\-\.]+/)[^\s]+?\.(jpg|jpeg|png|gif|bmp|tiff|webp)(?:\?[^\s]*)?)`)

	match := pathRegex.FindString(cleanedText)

	// If no match yet, try a more permissive regex focused on URLs
	if match == "" {
		urlRegex := regexp.MustCompile(`https?://[^\s]+?\.(jpg|jpeg|png|gif|bmp|tiff|webp)(?:/[^\s]*?)?`)
		match = urlRegex.FindString(cleanedText)
	}

	return match
}

func extractFilePath__(input string) string {
	// First, normalize line breaks to handle word wrapping
	normalized := strings.ReplaceAll(input, "\n", "")

	// Split by the arrow
	parts := strings.Split(normalized, "→")
	if len(parts) < 2 {
		return ""
	}

	// Get everything after the arrow
	afterArrow := strings.TrimSpace(parts[1])

	// Strip ANSI escape codes that might be surrounding the path
	// ANSI escape codes typically start with ESC[ (represented as \x1b[ or \033[)
	// and end with a letter (m is common for color codes)
	ansiEscapeRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	cleanedText := ansiEscapeRegex.ReplaceAllString(afterArrow, "")

	// Match URLs and file paths
	pathRegex := regexp.MustCompile(`((?:https?://|/|[A-Za-z]:\\|[A-Za-z0-9_\-\.]+/)[^\s]+?(?:\.(jpg|jpeg|png|gif|bmp|tiff|webp)(?:[\?#][^\s]*)?))`)
	match := pathRegex.FindString(cleanedText)

	return match
}

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

/*
type BufLinesEvent struct {
	Buffer nvim.Buffer
	//Changetick  int64
	Changetick  interface{} //int64
	FirstLine   interface{} //int64
	LastLine    interface{} //int64
	LineData    string
	IsMultipart bool
}
*/
