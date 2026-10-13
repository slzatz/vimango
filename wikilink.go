package main

import (
	"bytes"
	"database/sql"
	"errors"
	"net/url"
	"strconv"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Wiki-style cross references, in exactly two forms:
//
//	see [[some-other-note]]   a note by title
//	see [[tid:6546]]          a note by its server id
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
// The target rides in data-target either way, so a host or a script can
// read what was written without parsing the href back apart.
//
// A tid reference names the note by the server id every vimango client
// shares and that a rename never touches. A title reference breaks the
// moment its note is renamed, and cannot say which of several notes with
// the same title it meant; a tid reference does neither. It shows the
// note's title as of this render, looked up through titleForTID, so even
// the displayed name follows a rename.
//
// There is no third form, and no "|label": the note's title is its name,
// and words about why a link is there go in the sentence around it. So
// everything between the brackets is the title -- "|", "[" and anything
// else included -- except that "tid:" is reserved. "tid:" with anything
// but digits after it is no reference at all and stays on screen as
// typed, where the mistake can be seen, rather than turning quietly into
// a link to a title nobody has. The prefix exists because a title can be
// all digits. The one thing a title reference cannot hold is "]]", which
// ends it, or a line break.
//
// An inline parser rather than a regex over the source, which is the
// whole reason this is worth writing: goldmark never runs inline parsers
// inside a code span or a fenced block, so a note that *documents* this
// syntax keeps its brackets. A preprocessing pass could not tell the two
// apart.

var kindWikiLink = ast.NewNodeKind("WikiLink")

type wikiLinkNode struct {
	ast.BaseInline
	// Target is everything between the brackets, trimmed.
	Target []byte
	// TID is the note's server id when Target is "tid:<digits>", else 0.
	TID int64
}

func (n *wikiLinkNode) Kind() ast.NodeKind { return kindWikiLink }

func (n *wikiLinkNode) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"Target": string(n.Target),
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
	target := bytes.TrimSpace(line[2 : 2+end])
	if len(target) == 0 {
		return nil
	}
	var tid int64
	if digits, ok := bytes.CutPrefix(target, []byte("tid:")); ok {
		// Reserved: a tid reference or nothing. Returning nil leaves the
		// brackets on screen as typed.
		if tid = parseTID(digits); tid == 0 {
			return nil
		}
	}

	block.Advance(2 + end + 2)
	return &wikiLinkNode{Target: target, TID: tid}
}

// parseTID returns the id in the digits after "tid:", or 0 if they are not
// a positive id. Digits only: ParseInt alone would also take "+5" and
// "-5", and neither is an id anyone wrote on purpose.
func parseTID(digits []byte) int64 {
	if len(digits) == 0 {
		return 0
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return 0
		}
	}
	tid, err := strconv.ParseInt(string(digits), 10, 64)
	if err != nil || tid <= 0 {
		return 0
	}
	return tid
}

// wikiLinkScheme is the URL scheme a host intercepts. Deliberately not a
// scheme anything else could claim, and deliberately hierarchical, so the
// target is a path component the receiver can percent-decode with the
// same URL parser it would use for any other link.
const wikiLinkScheme = "vimango://note/"

// wikiLinkTIDScheme is the same thing for a tid reference. Its own host
// component, so the receiver never has to guess whether "6546" is an id or
// a note that happens to be titled with digits.
const wikiLinkTIDScheme = "vimango://tid/"

// titleForTID resolves a tid reference to the note's current title. It
// returns sql.ErrNoRows when there is no such live note -- the reference
// is then marked missing -- and any other error when it could not tell,
// in which case nothing is claimed either way.
type titleForTID func(tid int64) (string, error)

// dbTitleForTID resolves against the open main database, or is nil when
// there is none (tests, or a render before InitDatabases). A nil resolver
// shows "tid:N" and never marks a reference missing.
func dbTitleForTID() titleForTID {
	if app == nil || app.Database == nil || app.Database.MainDB == nil {
		return nil
	}
	db := app.Database.MainDB
	return func(tid int64) (string, error) {
		var title string
		err := db.QueryRow("SELECT title FROM task WHERE tid=? AND deleted=0;", tid).Scan(&title)
		return title, err
	}
}

type wikiLinkRenderer struct {
	anchors     bool
	titleForTID titleForTID
}

func (r wikiLinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(kindWikiLink, r.render)
}

func (r wikiLinkRenderer) render(w util.BufWriter, source []byte, node ast.Node,
	entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*wikiLinkNode)
	if n.TID != 0 {
		r.renderTID(w, n)
		return ast.WalkSkipChildren, nil
	}
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
	_, _ = w.Write(util.EscapeHTML(n.Target))
	if r.anchors {
		_, _ = w.WriteString(`</a>`)
	} else {
		_, _ = w.WriteString(`</span>`)
	}
	// The node has no children -- the title is not re-parsed as markdown,
	// because a reference is a name and not a place to put emphasis.
	return ast.WalkSkipChildren, nil
}

// renderTID writes a tid reference: the note's current title, or "tid:N"
// when there is none to show. Same markup as a title reference, plus
// data-tid, and a "missing" class when the resolver says the note is gone
// -- still an anchor then, so a click reaches the host and it can say so,
// the same as a title that names nothing.
func (r wikiLinkRenderer) renderTID(w util.BufWriter, n *wikiLinkNode) {
	tid := strconv.FormatInt(n.TID, 10)
	label, missing := n.Target, false
	if r.titleForTID != nil {
		title, err := r.titleForTID(n.TID)
		switch {
		case err == nil && title != "":
			label = []byte(title)
		case errors.Is(err, sql.ErrNoRows):
			missing = true
		}
	}
	class := "wikilink"
	if missing {
		class = "wikilink missing"
	}
	if r.anchors {
		_, _ = w.WriteString(`<a class="` + class + `" href="` + wikiLinkTIDScheme + tid + `" data-tid="` + tid + `" data-target="`)
	} else {
		_, _ = w.WriteString(`<span class="` + class + `" data-tid="` + tid + `" data-target="`)
	}
	_, _ = w.Write(util.EscapeHTML(n.Target))
	_, _ = w.WriteString(`">`)
	_, _ = w.Write(util.EscapeHTML(label))
	if r.anchors {
		_, _ = w.WriteString(`</a>`)
	} else {
		_, _ = w.WriteString(`</span>`)
	}
}

// wikiLinkExtension bundles the two halves so the call site registers one
// thing, the way the goldmark extensions beside it do. anchors is the
// caller saying "I am a host and I will handle a click"; see the note at
// the top of this file for why that is the caller's to say.
type wikiLinkExtension struct {
	anchors     bool
	titleForTID titleForTID
}

func (e wikiLinkExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(
		// Between the code span parser (100) and the link parser (200).
		// Ahead of links so the brackets are never claimed as a link
		// label first; behind code spans for the same reason a code
		// span wins everywhere else.
		util.Prioritized(wikiLinkParser{}, 150),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(wikiLinkRenderer{anchors: e.anchors, titleForTID: e.titleForTID}, 500),
	))
}
