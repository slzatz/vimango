package main

import (
	"bytes"
	"net/url"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Wiki-style cross references:
//
//	see [[some-other-note]]
//	see [[some-other-note|what to call it here]]
//
// The memory docs use these to point at each other. Plain CommonMark has
// no such syntax, so they used to survive to the screen as literal
// brackets -- readable, but noise rather than a reference.
//
// A span by default, not an anchor, because the two programs that render
// this HTML disagree about whether there is a host at all: the TUI's own
// webview window has none, and an <a> there would be a click target that
// does nothing, or worse, a custom scheme handed to an unhandled
// navigation. So the default markup says what is true -- this is a named
// reference -- and the dotted rule rather than a link's solid one says it
// in CSS too.
//
// A caller that *is* a host says so, and only then does the reference
// become an anchor (anchors below). That is not a preference: it is what
// keeps the promise structural rather than remembered. The TUI cannot
// emit a link it has no way to handle, and the hybrid app's read-only
// peek window renders without it, so a peeked note's own references stay
// inert and one click can never become a pile of windows.
//
// The target rides in data-target either way -- it is the only place the
// target survives when a [[target|label]] renames it.
//
// An inline parser rather than a regex over the source, which is the
// whole reason this is worth writing: goldmark never runs inline parsers
// inside a code span or a fenced block, so a note that *documents* this
// syntax keeps its brackets. A preprocessing pass could not tell the two
// apart.

var kindWikiLink = ast.NewNodeKind("WikiLink")

type wikiLinkNode struct {
	ast.BaseInline
	// Target is what was written before any "|"; Label is what to show.
	// Equal unless the reference was relabeled.
	Target []byte
	Label  []byte
}

func (n *wikiLinkNode) Kind() ast.NodeKind { return kindWikiLink }

func (n *wikiLinkNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"Target": string(n.Target),
		"Label":  string(n.Label),
	}, nil)
}

type wikiLinkParser struct{}

func (wikiLinkParser) Trigger() []byte { return []byte{'['} }

func (wikiLinkParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	// PeekLine stops at the line end, which is the rule we want anyway: a
	// reference does not straddle lines.
	line, _ := block.PeekLine()
	if len(line) < 5 || line[0] != '[' || line[1] != '[' {
		return nil
	}
	end := bytes.Index(line[2:], []byte("]]"))
	if end <= 0 {
		// No close, or "[[]]" with nothing inside. Returning nil hands
		// the brackets back to the link parser below us at priority
		// 200, and failing that they stay on screen as text -- exactly
		// what happens today.
		return nil
	}
	inner := line[2 : 2+end]
	// A stray bracket inside means this is not one reference, and
	// guessing which one it is would be inventing syntax.
	if bytes.ContainsAny(inner, "[]") {
		return nil
	}

	target, label := inner, inner
	if i := bytes.IndexByte(inner, '|'); i >= 0 {
		target, label = inner[:i], inner[i+1:]
	}
	target = bytes.TrimSpace(target)
	label = bytes.TrimSpace(label)
	if len(target) == 0 {
		return nil
	}
	if len(label) == 0 {
		// "[[target|]]" -- a relabel that named nothing. The target is
		// still a perfectly good thing to show.
		label = target
	}

	block.Advance(2 + end + 2)
	return &wikiLinkNode{Target: target, Label: label}
}

// wikiLinkScheme is the URL scheme a host intercepts. Deliberately not a
// scheme anything else could claim, and deliberately hierarchical, so the
// target is a path component the receiver can percent-decode with the
// same URL parser it would use for any other link.
const wikiLinkScheme = "vimango://note/"

type wikiLinkRenderer struct{ anchors bool }

func (r wikiLinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindWikiLink, r.render)
}

func (r wikiLinkRenderer) render(w util.BufWriter, source []byte, node ast.Node,
	entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*wikiLinkNode)
	if r.anchors {
		// PathEscape, not EscapeHTML, for the href: a title may hold a
		// slash, a question mark or a hash, and each of those means
		// something to a URL parser before it ever reaches the host.
		// The result is then HTML-escaped like any attribute value.
		_, _ = w.WriteString(`<a class="wikilink" href="`)
		_, _ = w.Write(util.EscapeHTML([]byte(wikiLinkScheme + url.PathEscape(string(n.Target)))))
		_, _ = w.WriteString(`" data-target="`)
	} else {
		_, _ = w.WriteString(`<span class="wikilink" data-target="`)
	}
	_, _ = w.Write(util.EscapeHTML(n.Target))
	_, _ = w.WriteString(`">`)
	_, _ = w.Write(util.EscapeHTML(n.Label))
	if r.anchors {
		_, _ = w.WriteString(`</a>`)
	} else {
		_, _ = w.WriteString(`</span>`)
	}
	// The node has no children -- the label is not re-parsed as markdown,
	// because a reference is a name and not a place to put emphasis.
	return ast.WalkSkipChildren, nil
}

// wikiLinkExtension bundles the two halves so the call site registers one
// thing, the way the goldmark extensions beside it do. anchors is the
// caller saying "I am a host and I will handle a click"; see the note at
// the top of this file for why that is the caller's to say.
type wikiLinkExtension struct{ anchors bool }

func (e wikiLinkExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(
		// Between the code span parser (100) and the link parser (200).
		// Ahead of links so the brackets are never claimed as a link
		// label first; behind code spans for the same reason a code
		// span wins everywhere else.
		util.Prioritized(wikiLinkParser{}, 150),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(wikiLinkRenderer{anchors: e.anchors}, 500),
	))
}
