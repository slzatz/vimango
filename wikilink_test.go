package main

import (
	"strings"
	"testing"
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
			"relabeled",
			"see [[other-note|that one]]",
			`<span class="wikilink" data-target="other-note">that one</span>`,
		},
		{
			"underscores are not emphasis",
			"[[feedback_skill_scope]]",
			`<span class="wikilink" data-target="feedback_skill_scope">feedback_skill_scope</span>`,
		},
		{
			"empty label falls back to target",
			"[[a-note|]]",
			`<span class="wikilink" data-target="a-note">a-note</span>`,
		},
		{
			"escaped",
			"[[a<b|x & y]]",
			`<span class="wikilink" data-target="a&lt;b">x &amp; y</span>`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := convertMarkdownToHTML(tc.in)
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
		got := convertMarkdownToHTML(src)
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
		"nested bracket":  "[[a[b]]",
	}
	for name, src := range cases {
		got := convertMarkdownToHTML(src)
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
		got := convertMarkdownToHTML(src)
		if !strings.Contains(got, "example.com") {
			t.Errorf("%s: link lost: %s", name, got)
		}
		if !strings.Contains(got, "<a ") && !strings.Contains(got, "<img ") {
			t.Errorf("%s: did not render as a link/image: %s", name, got)
		}
	}
}
