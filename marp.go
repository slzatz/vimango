package main

import (
	"bytes"
	"html"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/util"
)

// A Marp deck's instructions to Marp, in the preview. A deck -- a note whose
// frontmatter says "marp: true" -- gives them in the text itself, as HTML:
//
//	<!-- _backgroundPosition: top left -->   a directive
//	<style scoped>...</style>                CSS for one slide
//
// Neither is part of what the note says, and neither used to show in the
// preview. So the only way to know one was there was to edit the note; and
// the CSS was worse than unseen, because the renderer runs WithUnsafe and
// browsers have dropped "scoped": a rule meant for one slide (h1, img)
// restyled the whole preview.
//
// In a deck, each directive is now a small dim label, and a style block is
// a label saying only that it is there, with its CSS in the tooltip. The CSS
// never reaches the page. In any other note HTML passes through as ever:
// these are instructions only to Marp, and only a deck is given to it on
// purpose.
//
// **A label means Marp reads it**, so the rules here are Marpit's
// (lib/markdown/comment.js, style/parse.js and directives/yaml.js; checked
// against marp-cli 4.5.1). A comment Marp has no use for is the slide's
// presenter notes, and stays as invisible here as on the slide. That makes a
// misspelled directive visible by its absence.
//
// A node renderer rather than a pass over the source, for wikilink.go's
// reason: goldmark never takes HTML in a code span or a code block for
// HTML, so a note that documents this keeps its text.

// marpDeckRe is "marp: true" at the top level of a frontmatter block. Not
// asked of frontmatter.Parse, which declines a block it cannot display: the
// CSS must not leak because the frontmatter holds a YAML comment.
var marpDeckRe = regexp.MustCompile(`(?m)^marp:[ \t]+true[ \t]*(?:#.*)?\r?$`)

// isMarpDeck reports whether a block frontmatter.Split returned makes its
// note a Marp deck.
func isMarpDeck(frontmatter string) bool {
	return marpDeckRe.MatchString(frontmatter)
}

// marpBuiltin are the directives of Marpit itself, whose values it reads
// loosely: anything that does not open with a YAML special character is a
// string, "#" and ": " included. The second group sets something of a
// slide's, and each also has a "_" form, for one slide only.
var marpBuiltin = map[string]bool{
	"headingDivider": true, "lang": true, "style": true, "theme": true,

	"backgroundColor": true, "backgroundImage": true,
	"backgroundPosition": true, "backgroundRepeat": true,
	"backgroundSize": true, "class": true, "color": true, "footer": true,
	"header": true, "paginate": true,
}

// marpAdded are the directives marp-core and marp-cli add.
var marpAdded = map[string]bool{
	"math": true, "size": true,
	"author": true, "description": true, "image": true, "keywords": true,
	"title": true, "url": true,
	"transition": true,
}

// marpGlobal are the directives that are the whole deck's, and so have no
// "_" form.
var marpGlobal = map[string]bool{
	"headingDivider": true, "lang": true, "style": true, "theme": true,
	"math": true, "size": true,
	"author": true, "description": true, "image": true, "keywords": true,
	"title": true, "url": true,
}

func isMarpDirective(key string) bool {
	if local, ok := strings.CutPrefix(key, "_"); ok {
		return (marpBuiltin[local] || marpAdded[local]) && !marpGlobal[local]
	}
	return marpBuiltin[key] || marpAdded[key]
}

// marpLabel is one label: its text, and what its tooltip says ("" for none).
type marpLabel struct {
	text, tooltip string
}

var marpPairRe = regexp.MustCompile(`^(?:([A-Za-z_][A-Za-z0-9_-]*)|"([^"]+)"|'([^']+)')[ \t]*:(?:[ \t]+(.*))?$`)
var marpItemRe = regexp.MustCompile(`^[ \t]*-[ \t]+(\S.*)$`)

// marpDirectives reads a comment's content (without its <!-- and -->, and
// trimmed, as Marp trims it) as directives: a label for each one Marp knows,
// its value as written, or none if the comment is not YAML -- one line that
// is not "key: value" and Marp reads the whole comment as presenter notes.
//
// The value of a block scalar ("style: |") is the tooltip; the label is the
// key alone. A list is its items, comma joined, as in the frontmatter card.
func marpDirectives(content string) []marpLabel {
	type pair struct {
		key, value string
		rest       []string // the indented lines under it
	}
	var pairs []pair
	seen := map[string]bool{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimRight(line, " \t\r")
		last := len(pairs) - 1
		// a block scalar's lines are its own, "#" and blank ones too
		inBlock := last >= 0 && strings.ContainsAny(firstByte(pairs[last].value), "|>")
		indented := strings.TrimLeft(line, " \t") != line
		switch {
		case inBlock && (indented || line == ""):
			pairs[last].rest = append(pairs[last].rest, line)
			continue
		case line == "" || strings.HasPrefix(strings.TrimLeft(line, " \t"), "#"):
			continue
		case indented || marpItemRe.MatchString(line):
			// a list or a map under "key:", and nothing under a key
			// that already has its value
			if last < 0 || pairs[last].value != "" {
				return nil
			}
			pairs[last].rest = append(pairs[last].rest, line)
			continue
		}
		m := marpPairRe.FindStringSubmatch(line)
		if m == nil {
			return nil
		}
		key, value := m[1]+m[2]+m[3], strings.TrimSpace(m[4])
		if seen[key] || !marpValueOK(key, value) {
			return nil
		}
		seen[key] = true
		pairs = append(pairs, pair{key: key, value: value})
	}

	var labels []marpLabel
	for _, p := range pairs {
		if !isMarpDirective(p.key) {
			continue
		}
		label := marpLabel{text: p.key + ": " + p.value}
		if len(p.rest) > 0 {
			var items []string
			for _, line := range p.rest {
				if m := marpItemRe.FindStringSubmatch(line); m != nil {
					items = append(items, m[1])
				}
			}
			if p.value == "" && len(items) == len(p.rest) {
				label.text = p.key + ": " + strings.Join(items, ", ")
			} else {
				label = marpLabel{text: p.key, tooltip: dedent(p.rest)}
			}
		}
		labels = append(labels, label)
	}
	return labels
}

// marpValueOK reports whether what follows "key:" on its line is a value to
// Marp, or an error that makes the comment presenter notes. Far from all of
// YAML: an alias or a tag where a value would be ("_footer: **bold**"), and,
// for a directive not read loosely, a second ": ".
func marpValueOK(key, value string) bool {
	if value == "" {
		return true
	}
	if strings.ContainsAny(firstByte(value), "*&") {
		return false
	}
	if marpBuiltin[strings.TrimPrefix(key, "_")] || strings.ContainsAny(firstByte(value), "\"'{[|>") {
		return true
	}
	return !strings.ContainsAny(firstByte(value), "!%@`") && !strings.Contains(value, ": ")
}

func firstByte(s string) string {
	if s == "" {
		return ""
	}
	return s[:1]
}

// dedent is a block's lines with the first one's indentation taken off each.
func dedent(lines []string) string {
	indent := ""
	for _, line := range lines {
		if line != "" {
			indent = line[:len(line)-len(strings.TrimLeft(line, " \t"))]
			break
		}
	}
	out := make([]string, len(lines))
	for i, line := range lines {
		out[i] = strings.TrimPrefix(line, indent)
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}

// marpExtension is the extension a deck is rendered with. Priority ahead of
// goldmark's own HTML renderer (1000), which it takes the place of for the
// two nodes that are raw HTML.
type marpExtension struct{}

func (marpExtension) Extend(m goldmark.Markdown) {
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(marpRenderer{}, 500),
	))
}

type marpRenderer struct{}

func (r marpRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindHTMLBlock, r.renderHTMLBlock)
	reg.Register(ast.KindRawHTML, r.renderRawHTML)
}

func (marpRenderer) renderHTMLBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.HTMLBlock)
	var raw bytes.Buffer
	for i := range n.Lines().Len() {
		line := n.Lines().At(i)
		raw.Write(line.Value(source))
	}
	if n.HasClosure() {
		raw.Write(n.ClosureLine.Value(source))
	}
	_, _ = w.WriteString(marpBlockHTML(raw.String()))
	return ast.WalkContinue, nil
}

func (marpRenderer) renderRawHTML(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*ast.RawHTML)
	var raw bytes.Buffer
	for i := range n.Segments.Len() {
		segment := n.Segments.At(i)
		raw.Write(segment.Value(source))
	}
	text := raw.String()
	if m := marpCommentRe.FindStringSubmatch(text); m != nil {
		if labels := marpDirectives(m[1]); labels != nil {
			_, _ = w.WriteString(marpLabelsHTML(labels))
			return ast.WalkSkipChildren, nil
		}
	}
	_, _ = w.WriteString(escapeStyleTags(text))
	return ast.WalkSkipChildren, nil
}

var (
	marpCommentRe = regexp.MustCompile(`(?s)^\s*<!--+\s*(.*?)\s*--+>`)
	marpStyleRe   = regexp.MustCompile(`(?is)^\s*<style(\s[^>]*)?>(.*?)</style>`)
	marpScopedRe  = regexp.MustCompile(`(?i)\bscoped\b`)
	styleTagRe    = regexp.MustCompile(`(?i)<(/?style\b)`)
)

// marpBlockHTML is a block of raw HTML as a deck's preview shows it. A
// directive or a style is the block's first thing or it is not one: what
// Marp finds further in is not read as either.
func marpBlockHTML(raw string) string {
	if loc := marpStyleRe.FindStringSubmatchIndex(raw); loc != nil {
		label := marpLabel{text: "style", tooltip: strings.TrimSpace(raw[loc[4]:loc[5]])}
		if loc[2] >= 0 && marpScopedRe.MatchString(raw[loc[2]:loc[3]]) {
			label.text = "style scoped"
		}
		return marpLabelBlock([]marpLabel{label}) + escapeStyleTags(raw[loc[1]:])
	}
	if loc := marpCommentRe.FindStringSubmatchIndex(raw); loc != nil {
		if labels := marpDirectives(raw[loc[2]:loc[3]]); labels != nil {
			return marpLabelBlock(labels) + escapeStyleTags(raw[loc[1]:])
		}
	}
	return escapeStyleTags(raw)
}

// escapeStyleTags makes a style tag text. Marp reads a style block only where
// a block of HTML starts with one; anywhere else it shows the tag on the
// slide as text, and so does the preview, rather than let the CSS in.
func escapeStyleTags(raw string) string {
	return styleTagRe.ReplaceAllString(raw, "&lt;$1")
}

func marpLabelBlock(labels []marpLabel) string {
	return `<div class="marp">` + marpLabelsHTML(labels) + "</div>"
}

func marpLabelsHTML(labels []marpLabel) string {
	var b strings.Builder
	for i, l := range labels {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(`<span class="marp-label"`)
		if l.tooltip != "" {
			b.WriteString(` title="` + html.EscapeString(l.tooltip) + `"`)
		}
		b.WriteString(">" + html.EscapeString(l.text) + "</span>")
	}
	return b.String()
}
