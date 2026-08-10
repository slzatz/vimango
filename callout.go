package main

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
)

// GitHub-style alert callouts:
//
//	> [!NOTE]
//	> Useful information the reader should not miss.
//
// The syntax is deliberately just a blockquote whose first line is a
// marker, so nothing here invents a block type: the marker line is
// stripped and the surviving blockquote gets a class the stylesheet
// knows (webview.go). Renderers that ignore the class -- the
// goldmark-pdf path in editor_cmd_line.go -- still get a clean quote
// rather than a stray "[!NOTE]" line.
//
// This runs as a ParagraphTransformer, not the more obvious
// ASTTransformer, because goldmark parses inlines *before* AST
// transformers run: by then "[!NOTE]" has been chewed into a run of
// Text nodes (the link parser claims the bracket) and is far messier
// to excise than one line of source. Paragraph transformers run at
// paragraph close, while Lines() still points at raw source.

var calloutMarker = regexp.MustCompile(`^\[!([A-Za-z]+)\]\s*$`)

// calloutKinds is the closed set GitHub recognizes. Anything else --
// "[!BOGUS]", or a quote that merely opens with a bracket -- is left
// alone and renders as the plain blockquote it is, which is also what
// GitHub does.
var calloutKinds = map[string]bool{
	"note":      true,
	"tip":       true,
	"important": true,
	"warning":   true,
	"caution":   true,
}

type calloutTransformer struct{}

func (calloutTransformer) Transform(p *ast.Paragraph, reader text.Reader, pc parser.Context) {
	// Only the *first* paragraph of a blockquote can carry the marker;
	// a "[!NOTE]" line further down is prose.
	bq, ok := p.Parent().(*ast.Blockquote)
	if !ok || bq.FirstChild() != p {
		return
	}
	lines := p.Lines()
	if lines.Len() == 0 {
		return
	}
	seg := lines.At(0) // addressable: Value has a pointer receiver
	m := calloutMarker.FindSubmatch(bytes.TrimRight(seg.Value(reader.Source()), " \t\r\n"))
	if m == nil {
		return
	}
	kind := strings.ToLower(string(m[1]))
	if !calloutKinds[kind] {
		return
	}

	bq.SetAttributeString("class", []byte("callout callout-"+kind))
	if lines.Len() == 1 {
		// The marker was a paragraph of its own (a blank quoted line
		// followed it). Drop the now-empty paragraph rather than
		// render an empty <p>.
		bq.RemoveChild(bq, p)
		return
	}
	lines.SetSliced(1, lines.Len())
}
