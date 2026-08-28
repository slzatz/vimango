package main

import (
	"html"
	"regexp"
	"strings"
)

// YAML frontmatter -- a "---" fenced block of key: value pairs at the very
// top of a note:
//
//	---
//	name: some-slug
//	description: "one line"
//	metadata:
//	  type: reference
//	---
//
// Plain CommonMark has no idea what that is. goldmark reads the opening
// "---" as a thematic break and the closing one -- which sits directly
// under a paragraph -- as a *setext h2 underline*, so the whole block
// rendered as one giant <hr> + <h2>, auto-heading-id and all. The editor
// meanwhile has always been right about it: chroma's markdown lexer peels
// the block off and hands it to the YAML lexer, which is why the keys come
// up colored on the terminal side. This file closes that gap for the
// webview.
//
// Not a goldmark hook, unlike the callouts next door: frontmatter is not a
// paragraph inside anything, and by the time the earliest hook runs the
// block has already been misparsed. The split happens on source, before
// goldmark ever sees the text -- the same place preprocessMarkdownImages
// works.

// splitFrontmatter peels a leading "---" fenced block off src. It is a
// deliberate transcription of chroma's own splitFrontmatter (lexers/
// markdown.go), so that the editor and the preview agree byte for byte on
// what counts as frontmatter. A note that looks like a memory doc in one
// pane looks like one in the other, or in neither.
//
// The strict "---\n" prefix is load-bearing rather than incidental: there
// are notes in the wild that open "---- Task Type: URL ----" and follow it
// with name:/url: lines, and a prefix test that tolerated the extra dashes
// would eat them.
func splitFrontmatter(src string) (frontmatter, body string, ok bool) {
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

// fmEntry is one displayed row. A parent key -- "metadata:" with its map
// on the lines below -- carries an empty value and is a header for the
// rows that follow it.
type fmEntry struct {
	key   string
	value string
	depth int
}

// fmMaxDepth caps the indent ladder. Two levels is every frontmatter this
// app has seen and as many as a narrow preview pane has room for; anything
// deeper renders flush at the last level rather than marching off the
// right edge.
const fmMaxDepth = 2

// A displayable line is "key:" or "key: value". The key charset is
// deliberately narrow -- the point is to recognize a metadata block, not
// to accept every legal YAML key, and a permissive pattern would start
// claiming prose.
var fmPairRe = regexp.MustCompile(`^(\s*)([A-Za-z0-9_.-]+)[ \t]*:(?:[ \t](.*)|)$`)

// A bare "- item" sequence under a parent key.
var fmItemRe = regexp.MustCompile(`^\s*-[ \t]+(\S.*)$`)

// YAML reserves these as the first character of a plain scalar: an
// unquoted value opening with one is an anchor, an alias, a tag, a nested
// collection, or the start of flow syntax -- never the string it looks
// like. "b: *anchor" displayed verbatim would name a value rather than
// give it, and "a: &anchor" puts the actual value on the indented lines
// below, which this parser would otherwise render as nested keys of a.
// Bailing hands those notes back to the markdown path unchanged.
//
// The set is chroma's, lifted from the plain-scalar rule in its YAML
// lexer -- the same lexer the editor pane uses, so the two agree on what
// is a scalar as well as on what is frontmatter.
const fmReservedLeaders = `{}[]?,:!-*&@`

// parseFrontmatter turns the block splitFrontmatter returned into ordered
// rows, or returns nil if it does not look like a flat map of scalars.
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
func parseFrontmatter(frontmatter string) []fmEntry {
	lines := strings.Split(frontmatter, "\n")
	// Drop the fences. splitFrontmatter guarantees both exist, the first
	// at index 0 and the last on the final non-empty line.
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

	var entries []fmEntry
	for _, raw := range lines {
		line := strings.TrimSuffix(raw, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		// "- item" lines fold into the parent key above them, comma
		// joined: a list of tags is one fact about the note, and giving
		// each element its own row would out-weigh the key naming them.
		if m := fmItemRe.FindStringSubmatch(line); m != nil {
			if len(entries) == 0 {
				return nil
			}
			last := &entries[len(entries)-1]
			item := fmUnquote(strings.TrimSpace(m[1]))
			if last.value == "" {
				last.value = item
			} else {
				last.value += ", " + item
			}
			continue
		}
		m := fmPairRe.FindStringSubmatch(line)
		if m == nil {
			return nil
		}
		// Tabs are not legal YAML indentation, so counting bytes is
		// counting spaces. Two per level is the convention every
		// producer here uses; odd indents round down rather than
		// invent a level.
		depth := len(m[1]) / 2
		if depth > fmMaxDepth {
			depth = fmMaxDepth
		}
		// Trailing space is not hypothetical: the memory-doc writer
		// emits "metadata: " with one, and untrimmed it would make a
		// parent key look like a key with an empty scalar. Same row
		// either way, but only because both spellings trim to the same
		// thing here.
		value := strings.TrimSpace(m[3])
		if value != "" && strings.IndexByte(fmReservedLeaders, value[0]) >= 0 {
			return nil
		}
		entries = append(entries, fmEntry{
			key:   m[2],
			value: fmUnquote(value),
			depth: depth,
		})
	}
	if len(entries) == 0 {
		return nil
	}
	return entries
}

// fmUnquote strips one layer of YAML quoting. Only the two escapes double
// quotes actually define are handled, because only those can appear inside
// one; single quotes escape nothing but themselves.
func fmUnquote(s string) string {
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

// frontmatterHTML renders the rows as a definition list. A dl because that
// is what this is, and because the stylesheet already colors dt with the
// preview accent -- the sheet's standing answer for the term half of a
// key/value pair, shared with list markers and the task checkbox. The
// class carries geometry only.
//
// Values are text, not markdown: a YAML scalar is a string, so backticks
// and underscores in a description stay on screen instead of turning into
// code spans and emphasis.
func frontmatterHTML(entries []fmEntry) string {
	var b strings.Builder
	b.WriteString("<dl class=\"frontmatter\">\n")
	for _, e := range entries {
		cls := ""
		if e.depth > 0 {
			cls = " class=\"fm-l" + string(rune('0'+e.depth)) + "\""
		}
		b.WriteString("<dt" + cls + ">")
		b.WriteString(html.EscapeString(e.key))
		b.WriteString("</dt><dd" + cls + ">")
		b.WriteString(html.EscapeString(e.value))
		b.WriteString("</dd>\n")
	}
	b.WriteString("</dl>\n")
	return b.String()
}
