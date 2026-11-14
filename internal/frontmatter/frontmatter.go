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

// Split peels a leading "---" fenced block off src. It is a deliberate
// transcription of chroma's own splitFrontmatter (lexers/markdown.go),
// which the editor highlighted with until 2026-11-14.
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

// YAML reserves these as the first character of a plain scalar: an
// unquoted value opening with one is an anchor, an alias, a tag, a nested
// collection, or the start of flow syntax -- never the string it looks
// like. "b: *anchor" displayed verbatim would name a value rather than
// give it, and "a: &anchor" puts the actual value on the indented lines
// below, which this parser would otherwise render as nested keys of a.
// Bailing hands those notes back to the markdown path unchanged.
//
// The set is chroma's, lifted from the plain-scalar rule in its YAML
// lexer.
const reservedLeaders = `{}[]?,:!-*&@`

// Parse turns the block Split returned into ordered rows, or returns nil if
// it does not look like a flat map of scalars.
//
// nil is the important return. It is not an error path -- it means "render
// this note exactly the way it renders today", so a block scalar, flow
// JSON, an anchor, or any other YAML this parser has no display for costs
// nothing more than the status quo. Guessing would be the expensive
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
	for _, raw := range lines {
		line := strings.TrimSuffix(raw, "\r")
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
