package main

import (
	"bytes"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/yuin/goldmark"
)

func TestWikiLinkRendering(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			"plain",
			"see [[other-note]] there",
			`see <span class="wikilink" data-target="other-note">other-note</span> there`,
		},
		{
			// No "|label" form: a bar is just part of the title, which
			// is what makes a title like "dmesg | grep ttyUSB" linkable.
			"a bar is part of the title",
			"see [[dmesg | grep ttyUSB]]",
			`<span class="wikilink" data-target="dmesg | grep ttyUSB">dmesg | grep ttyUSB</span>`,
		},
		{
			"so is a bracket",
			"[[a[b]]",
			`<span class="wikilink" data-target="a[b">a[b</span>`,
		},
		{
			"underscores are not emphasis",
			"[[feedback_skill_scope]]",
			`<span class="wikilink" data-target="feedback_skill_scope">feedback_skill_scope</span>`,
		},
		{
			"surrounding space is trimmed",
			"[[ a-note ]]",
			`<span class="wikilink" data-target="a-note">a-note</span>`,
		},
		{
			"escaped",
			"[[a<b & c]]",
			`<span class="wikilink" data-target="a&lt;b &amp; c">a&lt;b &amp; c</span>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := convertMarkdownToHTML(tc.in, false)
			if !strings.Contains(got, tc.want) {
				t.Errorf("got %q, want it to contain %q", got, tc.want)
			}
		})
	}
}

// The reason this is an inline parser and not a regex over the source: a
// note that documents the syntax has to keep its brackets.
func TestWikiLinkLeavesCodeAlone(t *testing.T) {
	cases := map[string]string{
		"code span":   "use `[[name]]` to refer",
		"fenced":      "```\n[[name]]\n```\n",
		"indented":    "    [[name]]\n",
		"inline html": "<pre>[[name]]</pre>\n",
	}
	for name, src := range cases {
		got := convertMarkdownToHTML(src, false)
		if strings.Contains(got, "wikilink") {
			t.Errorf("%s: brackets were consumed: %s", name, got)
		}
		if !strings.Contains(got, "[[name]]") {
			t.Errorf("%s: literal brackets lost: %s", name, got)
		}
	}
}

// Anything that is not a reference must render exactly as it does today.
func TestWikiLinkNonMatches(t *testing.T) {
	cases := map[string]string{
		"unclosed":        "[[name",
		"single bracket":  "[name]",
		"empty":           "[[]]",
		"whitespace only": "[[ ]]",
		"split by a line": "[[a\nb]]",
	}
	for name, src := range cases {
		got := convertMarkdownToHTML(src, false)
		if strings.Contains(got, "wikilink") {
			t.Errorf("%s: unexpectedly matched: %s", name, got)
		}
	}
}

// The parser sits ahead of goldmark's link parser, so it has to be shown
// not to have taken anything from it.
func TestWikiLinkDoesNotBreakRealLinks(t *testing.T) {
	cases := map[string]string{
		"inline link":    "[text](http://example.com)",
		"image":          "![alt](http://example.com/i.png)",
		"ref definition": "[text][ref]\n\n[ref]: http://example.com\n",
		"autolink":       "<http://example.com>",
		"bare url":       "http://example.com",
	}
	for name, src := range cases {
		got := convertMarkdownToHTML(src, false)
		if !strings.Contains(got, "example.com") {
			t.Errorf("%s: link lost: %s", name, got)
		}
		if !strings.Contains(got, "<a ") && !strings.Contains(got, "<img ") {
			t.Errorf("%s: did not render as a link/image: %s", name, got)
		}
	}
}

// With anchors on, the same references become links a host can intercept.
// The span form above is the default and stays the default; these two
// render paths differ in exactly one thing.
func TestWikiLinkAnchors(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			"plain",
			"see [[other-note]]",
			`<a class="wikilink" href="vimango://note/other-note" data-target="other-note">other-note</a>`,
		},
		{
			"a bar in the title is escaped in the href",
			"[[a|b]]",
			`<a class="wikilink" href="vimango://note/a%7Cb" data-target="a|b">a|b</a>`,
		},
		{
			// A title is arbitrary user text. Each of these means
			// something to a URL parser before the host ever sees it.
			"slash hash question and space are escaped in the href",
			"[[a/b#c?d e]]",
			`href="vimango://note/a%2Fb%23c%3Fd%20e"`,
		},
		{
			"ampersand and quote stay HTML-escaped in both",
			`[[a&b]]`,
			`data-target="a&amp;b"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := convertMarkdownToHTML(tc.in, true)
			if !strings.Contains(got, tc.want) {
				t.Errorf("got %q, want it to contain %q", got, tc.want)
			}
			if strings.Contains(got, "<span class=\"wikilink\"") {
				t.Errorf("still emitted a span: %s", got)
			}
		})
	}
}

// The flag decides markup and nothing else: the same text that is not a
// reference is not a reference either way, and code is still left alone.
func TestWikiLinkAnchorsChangeNothingElse(t *testing.T) {
	for _, src := range []string{
		"use `[[name]]` to refer",
		"```\n[[name]]\n```\n",
		"[[unclosed",
		"[[]]",
		"[text](http://example.com)",
		"plain paragraph with no references at all",
	} {
		spans := convertMarkdownToHTML(src, false)
		anchors := convertMarkdownToHTML(src, true)
		if spans != anchors {
			t.Errorf("%q differed between modes:\n span:   %s\n anchor: %s", src, spans, anchors)
		}
	}
}

func TestDetermineWikiLinks(t *testing.T) {
	if DetermineWikiLinks([]string{"vimango", "--render-html", "5"}) {
		t.Error("absent flag reported present")
	}
	if !DetermineWikiLinks([]string{"vimango", "--render-html", "5", "--wiki-links"}) {
		t.Error("present flag reported absent")
	}
}

// renderWikiOnly renders src with only the wiki-link extension, so a test
// can hand it a resolver; convertMarkdownToHTML takes the database's.
func renderWikiOnly(t *testing.T, src string, anchors bool, resolve titleForTID) string {
	t.Helper()
	md := goldmark.New(goldmark.WithExtensions(wikiLinkExtension{anchors: anchors, titleForTID: resolve}))
	var buf bytes.Buffer
	if err := md.Convert([]byte(src), &buf); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func fakeTitles(titles map[int64]string) titleForTID {
	return func(tid int64) (string, error) {
		if tid == 999 {
			return "", errors.New("database is locked")
		}
		if title, ok := titles[tid]; ok {
			return title, nil
		}
		return "", sql.ErrNoRows
	}
}

func TestWikiLinkTID(t *testing.T) {
	resolve := fakeTitles(map[int64]string{6546: "Sync <topology>"})
	cases := []struct {
		name    string
		in      string
		anchors bool
		want    string
	}{
		{
			"shows the current title",
			"[[tid:6546]]", true,
			`<a class="wikilink" href="vimango://tid/6546" data-tid="6546" data-target="tid:6546">Sync &lt;topology&gt;</a>`,
		},
		{
			"span form carries the tid too",
			"[[tid:6546]]", false,
			`<span class="wikilink" data-tid="6546" data-target="tid:6546">Sync &lt;topology&gt;</span>`,
		},
		{
			"gone is marked missing but stays clickable",
			"[[tid:42]]", true,
			`<a class="wikilink missing" href="vimango://tid/42" data-tid="42" data-target="tid:42">tid:42</a>`,
		},
		{
			"a lookup that failed claims nothing",
			"[[tid:999]]", true,
			`<a class="wikilink" href="vimango://tid/999" data-tid="999" data-target="tid:999">tid:999</a>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := renderWikiOnly(t, tc.in, tc.anchors, resolve)
			if !strings.Contains(got, tc.want) {
				t.Errorf("got %q, want it to contain %q", got, tc.want)
			}
		})
	}
}

// Without a database there is no title to show and no way to know a note
// is gone, so the reference shows what was written and claims nothing.
func TestWikiLinkTIDWithoutResolver(t *testing.T) {
	got := renderWikiOnly(t, "[[tid:6546]]", true, nil)
	want := `<a class="wikilink" href="vimango://tid/6546" data-tid="6546" data-target="tid:6546">tid:6546</a>`
	if !strings.Contains(got, want) {
		t.Errorf("got %q, want it to contain %q", got, want)
	}
}

// "tid:" is reserved: digits after it, or no reference at all -- never a
// quiet fall back to a title link. The text stays on screen as typed.
func TestWikiLinkTIDMalformed(t *testing.T) {
	for _, src := range []string{
		"[[tid:]]", "[[tid:abc]]", "[[tid:+5]]", "[[tid:-5]]", "[[tid:0]]",
		"[[tid:12x]]", "[[tid:6546|label]]", "[[tid: 6546]]",
	} {
		got := renderWikiOnly(t, src, true, fakeTitles(map[int64]string{6546: "t"}))
		if strings.Contains(got, "wikilink") {
			t.Errorf("%q should not be a reference: %s", src, got)
		}
		if !strings.Contains(got, "[[tid:") {
			t.Errorf("%q lost its text: %s", src, got)
		}
	}
}

// Anything not starting with exactly "tid:" is a title, digits and all.
func TestWikiLinkTIDLookalikesAreTitles(t *testing.T) {
	for _, src := range []string{"[[TID:5]]", "[[6546]]", "[[tids:5]]"} {
		got := renderWikiOnly(t, src, true, fakeTitles(nil))
		if strings.Contains(got, "data-tid") || !strings.Contains(got, `href="vimango://note/`) {
			t.Errorf("%q should be a title reference: %s", src, got)
		}
	}
	got := renderWikiOnly(t, "use `[[tid:6546]]` to refer", true, fakeTitles(nil))
	if !strings.Contains(got, "<code>[[tid:6546]]</code>") {
		t.Errorf("code span lost its brackets: %s", got)
	}
}
