package main

import (
	"regexp"
	"strings"
)

// splitFrontmatter is a copy of the one in ~/vimango/frontmatter.go, which
// lives in package main there and can't be imported from cmd/. Keep the
// two identical: this command has to agree with every client on what
// frontmatter *is*, and that one is the definition the Swift ports follow.
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

var uuidRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// cleaned is what cleanTaskNote makes of one note.
type cleaned struct {
	uuid    string   // the task, lowercased
	note    string   // the new body; "" means empty
	removed []string // the lines taken out, for --dry-run
}

// cleanTaskNote takes the Taskwarrior link out of a note's text. It returns
// false for a note that isn't linked, which it leaves alone.
//
// A note is linked when its leading frontmatter holds a `taskwarrior: <uuid>`
// line, matched the way the clients matched it (the line trimmed, a valid
// uuid). Then it removes:
//   - that line, and any top-level `project:` line Taskchamp wrote beside it;
//   - the whole block, if nothing else is left in it;
//   - the `[Open task in Taskchamp](taskchampdev://task/<uuid>)` line, plus
//     one blank line after it when it sat between blank lines (or at the top);
//   - blank lines left at the top once the block is gone.
func cleanTaskNote(note string) (cleaned, bool) {
	fm, body, ok := splitFrontmatter(note)
	if !ok {
		return cleaned{}, false
	}
	// Lines keep any "\r", so a CRLF note is rebuilt with its own endings.
	lines := strings.Split(strings.TrimSuffix(fm, "\n"), "\n")
	open, inner, close := lines[0], lines[1:len(lines)-1], lines[len(lines)-1]

	var c cleaned
	for _, l := range inner {
		t := strings.TrimSpace(l)
		if v, found := strings.CutPrefix(t, "taskwarrior:"); found && uuidRe.MatchString(strings.TrimSpace(v)) {
			c.uuid = strings.ToLower(strings.TrimSpace(v))
			break
		}
	}
	if c.uuid == "" {
		return cleaned{}, false
	}

	var kept []string
	for _, l := range inner {
		t := strings.TrimSpace(l)
		v, isTW := strings.CutPrefix(t, "taskwarrior:")
		switch {
		case isTW && strings.EqualFold(strings.TrimSpace(v), c.uuid),
			strings.HasPrefix(l, "project:"):
			c.removed = append(c.removed, strings.TrimSuffix(l, "\r"))
		default:
			kept = append(kept, l)
		}
	}
	blockLeft := false
	for _, l := range kept {
		if strings.TrimSpace(l) != "" {
			blockLeft = true
		}
	}
	block := ""
	if blockLeft {
		block = strings.Join(append(append([]string{open}, kept...), close), "\n") + "\n"
	} else {
		c.removed = append([]string{"---"}, append(c.removed, "---")...)
	}

	openTask := "[Open task in Taskchamp](taskchampdev://task/" + c.uuid + ")"
	blank := func(l string) bool { return strings.TrimSpace(l) == "" }
	bodyLines := strings.Split(body, "\n")
	var out []string
	for i := 0; i < len(bodyLines); i++ {
		l := bodyLines[i]
		if !strings.EqualFold(strings.TrimSpace(l), openTask) {
			out = append(out, l)
			continue
		}
		c.removed = append(c.removed, strings.TrimSuffix(l, "\r"))
		// The final element is what follows the last "\n", not a line, so
		// it is never eaten as a blank one.
		if (len(out) == 0 || blank(out[len(out)-1])) && i+2 < len(bodyLines) && blank(bodyLines[i+1]) {
			i++
		}
	}
	rest := strings.Join(out, "\n")
	if !blockLeft {
		rest = strings.TrimLeft(rest, "\r\n")
	}

	c.note = block + rest
	if strings.TrimSpace(c.note) == "" {
		c.note = ""
	}
	return c, true
}

var conflictRe = regexp.MustCompile(` \(conflict \d{4}-\d{2}-\d{2}\)$`)

// isConflictCopy reports whether a title is one a sync conflict copy gets:
// "<title> (conflict YYYY-MM-DD)", as both sync.go and VimNotes name them.
func isConflictCopy(title string) bool {
	return conflictRe.MatchString(title)
}
