// Package frontmatter is what vimango takes for YAML frontmatter -- a "---"
// fenced block of key: value pairs at the very top of a note:
//
//	---
//	name: some-slug
//	description: "one line"
//	metadata:
//	  type: reference
//	---
//
// A value may also be a block scalar, the lines under "key: |" or "key: >"
// (a Marp deck's "style: |" with CSS below it).
//
// Two panes ask, and they must agree: the preview (--render-html,
// note_html.go in the application), which shows the block as a card of
// keys and values instead of as markdown, and the editor, which draws it
// plain instead of highlighting it as markdown. **A block is frontmatter
// when Split finds it and Parse can display it**; when Parse declines,
// both panes treat the rows as ordinary markdown. Both use this package, so
// a note that looks like a memory doc in one pane looks like one in the
// other, or in neither.
//
// Plain CommonMark has no idea what frontmatter is. goldmark reads the
// opening "---" as a thematic break and the closing one -- which sits
// directly under a paragraph -- as a *setext h2 underline*, so the whole
// block rendered as one giant <hr> + <h2>, auto-heading-id and all. The
// split happens on source, before goldmark ever sees the text.
package frontmatter

import (
	"regexp"
	"strings"
)

// Split peels a leading "---" fenced block off src: "---" alone on the first
// line, to the next line that is "---" alone with a line break after it.
// (The rule is the one chroma's markdown lexer used when the editor
// highlighted with it, until 2026-11-14.)
//
// The strict "---\n" prefix is load-bearing rather than incidental: there
// are notes in the wild that open "---- Task Type: URL ----" and follow it
// with name:/url: lines, and a prefix test that tolerated the extra dashes
// would eat them.
func Split(src string) (frontmatter, body string, ok bool) {
	if !strings.HasPrefix(src, "---\n") && !strings.HasPrefix(src, "---\r\n") {
		return "", src, false
	}
	lineEnd := strings.IndexByte(src, '\n')
	if lineEnd < 0 {
		return "", src, false
	}
	if strings.TrimSuffix(src[:lineEnd], "\r") != "---" {
		return "", src, false
	}
	for pos := lineEnd + 1; pos < len(src); {
		next := strings.IndexByte(src[pos:], '\n')
		if next < 0 {
			break
		}
		lineEnd = pos + next
		if strings.TrimSuffix(src[pos:lineEnd], "\r") == "---" {
			return src[:lineEnd+1], src[lineEnd+1:], true
		}
		pos = lineEnd + 1
	}
	// An unterminated block is not frontmatter; it is a note that happens
	// to start with a horizontal rule.
	return "", src, false
}

// Rows is the editor's question: how many of a note's leading rows are
// frontmatter, both "---" rows included, or 0. rows is the note's text split
// on "\n", as the editor holds it, and the answer is the one Split and Parse
// give for strings.Join(rows, "\n") -- found without joining the whole note,
// since the editor asks on every redraw.
func Rows(rows []string) int {
	if len(rows) == 0 || strings.TrimSuffix(rows[0], "\r") != "---" {
		return 0
	}
	// The first "---" row after the opener closes the block -- unless it is
	// the last row, where no line break follows it, and Split finds none.
	for i := 1; i < len(rows)-1; i++ {
		if strings.TrimSuffix(rows[i], "\r") == "---" {
			if Parse(strings.Join(rows[:i+1], "\n")+"\n") == nil {
				return 0
			}
			return i + 1
		}
	}
	return 0
}

// Entry is one displayed row. A parent key -- "metadata:" with its map
// on the lines below -- carries an empty value and is a header for the
// rows that follow it.
type Entry struct {
	Key   string
	Value string
	Depth int
	// Literal says the value was written as a "|" block scalar: its line
	// breaks and its indentation are the author's and are to be kept as
	// they are (it is CSS, or a script, as often as prose). The value of
	// a folded one (">") is ordinary text that may hold a line break.
	Literal bool
}

// maxDepth caps the indent ladder. Two levels is every frontmatter this
// app has seen and as many as a narrow preview pane has room for; anything
// deeper renders flush at the last level rather than marching off the
// right edge.
const maxDepth = 2

// A displayable line is "key:" or "key: value". The key charset is
// deliberately narrow -- the point is to recognize a metadata block, not
// to accept every legal YAML key, and a permissive pattern would start
// claiming prose.
var pairRe = regexp.MustCompile(`^(\s*)([A-Za-z0-9_.-]+)[ \t]*:(?:[ \t](.*)|)$`)

// A bare "- item" sequence under a parent key.
var itemRe = regexp.MustCompile(`^\s*-[ \t]+(\S.*)$`)

// A block scalar's header, standing where a value would: "|" keeps the
// lines below it as they are and ">" folds them into running text. Either
// may carry a chomping mark, which says only what becomes of the line
// breaks at the end and so changes nothing that is shown. An explicit
// indentation digit ("|2") is not taken: nothing here has written one, and
// Parse declines what it has not been shown.
var blockRe = regexp.MustCompile(`^[|>][+-]?$`)

// YAML reserves these as the first character of a plain scalar: an
// unquoted value opening with one is an anchor, an alias, a tag, a nested
// collection, or the start of flow syntax -- never the string it looks
// like. "b: *anchor" displayed verbatim would name a value rather than
// give it, and "a: &anchor" puts the actual value on the indented lines
// below, which this parser would otherwise render as nested keys of a.
// Bailing hands those notes back to the markdown path unchanged.
//
// These are YAML's indicator characters, which a plain scalar may not start
// with.
const reservedLeaders = `{}[]?,:!-*&@`

// Parse turns the block Split returned into ordered rows, or returns nil if
// it does not look like a flat map of scalars.
//
// nil is the important return. It is not an error path -- it means "render
// this note exactly the way it renders today", so flow JSON, an anchor, a
// comment, or any other YAML this parser has no display for costs nothing
// more than the status quo. (A block scalar was on that list until
// 2026-11-22, when a Marp deck's "style: |" made every deck's frontmatter
// render as markdown; it is a value now, blockScalar.) Guessing would be the expensive
// option: a half-parsed block silently drops the half it could not read,
// and the note is the only copy.
//
// Ordered because a map is not: yaml.v2 is already in the module graph but
// hands back a Go map, and the one thing a metadata block owes the reader
// is the order the author wrote it in.
func Parse(frontmatter string) []Entry {
	lines := strings.Split(frontmatter, "\n")
	// Drop the fences. Split guarantees both exist, the first at index 0
	// and the last on the final non-empty line.
	if len(lines) < 2 {
		return nil
	}
	lines = lines[1:]
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSuffix(lines[i], "\r") == "---" {
			lines = lines[:i]
			break
		}
	}

	var entries []Entry
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSuffix(lines[i], "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		// "- item" lines fold into the parent key above them, comma
		// joined: a list of tags is one fact about the note, and giving
		// each element its own row would out-weigh the key naming them.
		if m := itemRe.FindStringSubmatch(line); m != nil {
			if len(entries) == 0 {
				return nil
			}
			last := &entries[len(entries)-1]
			item := unquote(strings.TrimSpace(m[1]))
			if last.Value == "" {
				last.Value = item
			} else {
				last.Value += ", " + item
			}
			continue
		}
		m := pairRe.FindStringSubmatch(line)
		if m == nil {
			return nil
		}
		// Tabs are not legal YAML indentation, so counting bytes is
		// counting spaces. Two per level is the convention every
		// producer here uses; odd indents round down rather than
		// invent a level.
		depth := len(m[1]) / 2
		if depth > maxDepth {
			depth = maxDepth
		}
		// Trailing space is not hypothetical: the memory-doc writer
		// emits "metadata: " with one, and untrimmed it would make a
		// parent key look like a key with an empty scalar. Same row
		// either way, but only because both spellings trim to the same
		// thing here.
		value := strings.TrimSpace(m[3])
		if blockRe.MatchString(value) {
			folded := value[0] == '>'
			text, next, ok := blockScalar(lines, i+1, len(m[1]), folded)
			if !ok {
				return nil
			}
			entries = append(entries, Entry{Key: m[2], Value: text, Depth: depth, Literal: !folded})
			i = next - 1
			continue
		}
		if value != "" && strings.IndexByte(reservedLeaders, value[0]) >= 0 {
			return nil
		}
		entries = append(entries, Entry{
			Key:   m[2],
			Value: unquote(value),
			Depth: depth,
		})
	}
	if len(entries) == 0 {
		return nil
	}
	return entries
}

// blockScalar reads the lines of a block scalar: those from lines[start]
// that are blank or indented further than the key they belong to, which is
// indented keyIndent. It returns their text and the index of the first line
// that is not part of it.
//
// The first line with text sets the block's indentation, which is taken off
// every line; what a line has beyond it is kept. Blank lines at either end
// are dropped -- whether YAML keeps them is the chomping mark's business,
// and on screen they are only space. A line indented less than the block
// but more than the key belongs to neither, which YAML calls an error and
// this declines (ok false).
//
// Folded, the lines become running text the way YAML folds them: a line
// break between two lines of text is a space, a blank line is a line break,
// and a line indented further than the block keeps the breaks around it.
func blockScalar(lines []string, start, keyIndent int, folded bool) (text string, next int, ok bool) {
	indent := -1
	var content []string
	next = start
	for ; next < len(lines); next++ {
		line := strings.TrimSuffix(lines[next], "\r")
		if strings.TrimSpace(line) == "" {
			content = append(content, "")
			continue
		}
		n := len(line) - len(strings.TrimLeft(line, " "))
		if n <= keyIndent {
			break
		}
		if indent < 0 {
			indent = n
		}
		if n < indent {
			return "", 0, false
		}
		content = append(content, line[indent:])
	}
	for len(content) > 0 && content[0] == "" {
		content = content[1:]
	}
	for len(content) > 0 && content[len(content)-1] == "" {
		content = content[:len(content)-1]
	}
	if !folded {
		return strings.Join(content, "\n"), next, true
	}
	var b strings.Builder
	for i, line := range content {
		if i > 0 {
			prev := content[i-1]
			switch {
			case line == "" && prev == "":
				b.WriteByte('\n')
			case line == "":
				// the break this blank line stands for is written
				// when the text after it is
			case prev == "" || moreIndented(line) || moreIndented(prev):
				b.WriteByte('\n')
			default:
				b.WriteByte(' ')
			}
		}
		b.WriteString(line)
	}
	return b.String(), next, true
}

// moreIndented reports whether a line of a block scalar, its block's own
// indentation already removed, was indented further than the block.
func moreIndented(line string) bool {
	return line != "" && (line[0] == ' ' || line[0] == '\t')
}

// unquote strips one layer of YAML quoting. Only the two escapes double
// quotes actually define are handled, because only those can appear inside
// one; single quotes escape nothing but themselves.
func unquote(s string) string {
	if len(s) >= 2 {
		switch {
		case s[0] == '"' && s[len(s)-1] == '"':
			inner := s[1 : len(s)-1]
			inner = strings.ReplaceAll(inner, `\"`, `"`)
			return strings.ReplaceAll(inner, `\\`, `\`)
		case s[0] == '\'' && s[len(s)-1] == '\'':
			return strings.ReplaceAll(s[1:len(s)-1], `''`, `'`)
		}
	}
	return s
}
