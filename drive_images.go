package main

import (
	"strings"

	"github.com/slzatz/vimango/internal/frontmatter"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/text"
)

// driveImageRef is one Google Drive image in a note's markdown: the bytes
// [start, end) of ![alt](url).
type driveImageRef struct {
	start, end int
	alt, url   string
}

// imagesToInline finds the note's Google Drive images: every driveImageRegex
// match that markdown reads as an image, and so not one inside a code span,
// a code block or raw HTML, where it is text. The image pass runs ahead of
// goldmark, so it has to ask: one in backticks was inlined anyway, its
// base64 then shown as the code span's text -- a 2MB photo is a 2.7-million
// character line, and WebKit stalled laying it out (2026-11-26).
func imagesToInline(markdown string) []driveImageRef {
	matches := driveImageRegex.FindAllStringSubmatchIndex(markdown, -1)
	if len(matches) == 0 {
		return nil
	}
	literal := literalRanges(markdown)
	var images []driveImageRef
	for _, m := range matches {
		if inRanges(literal, m[0]) {
			continue
		}
		images = append(images, driveImageRef{
			start: m[0], end: m[1],
			alt: markdown[m[2]:m[3]], url: markdown[m[4]:m[5]],
		})
	}
	return images
}

// literalRanges are the byte ranges of the markdown that goldmark shows as
// written rather than parsing for images: code spans, fenced and indented
// code blocks, HTML blocks and inline raw HTML. A YAML frontmatter block is
// left out of the parse -- goldmark would read it as a heading -- and so is
// never literal; the HTML render has split it off before the image pass in
// any case.
func literalRanges(markdown string) [][2]int {
	offset, body := 0, markdown
	if _, rest, ok := frontmatter.Split(markdown); ok && strings.HasSuffix(markdown, rest) {
		offset, body = len(markdown)-len(rest), rest
	}
	source := []byte(body)
	doc := goldmark.New(goldmark.WithExtensions(extension.GFM)).Parser().Parse(text.NewReader(source))

	var ranges [][2]int
	addSegments := func(segs *text.Segments) {
		for i := 0; i < segs.Len(); i++ {
			s := segs.At(i)
			ranges = append(ranges, [2]int{offset + s.Start, offset + s.Stop})
		}
	}
	ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := n.(type) {
		case *ast.CodeSpan:
			for c := n.FirstChild(); c != nil; c = c.NextSibling() {
				if t, ok := c.(*ast.Text); ok {
					ranges = append(ranges, [2]int{offset + t.Segment.Start, offset + t.Segment.Stop})
				}
			}
			return ast.WalkSkipChildren, nil
		case *ast.RawHTML:
			addSegments(n.Segments)
			return ast.WalkSkipChildren, nil
		case *ast.HTMLBlock:
			addSegments(n.Lines())
			if n.HasClosure() {
				ranges = append(ranges, [2]int{offset + n.ClosureLine.Start, offset + n.ClosureLine.Stop})
			}
			return ast.WalkSkipChildren, nil
		case *ast.FencedCodeBlock, *ast.CodeBlock:
			addSegments(n.Lines())
			return ast.WalkSkipChildren, nil
		}
		return ast.WalkContinue, nil
	})
	return ranges
}

func inRanges(ranges [][2]int, pos int) bool {
	for _, r := range ranges {
		if pos >= r[0] && pos < r[1] {
			return true
		}
	}
	return false
}
