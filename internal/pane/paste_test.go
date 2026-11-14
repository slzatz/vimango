package pane

import "testing"

func TestPastedTextIsCleaned(t *testing.T) {
	for raw, want := range map[string]string{
		"a\rb\r":             "a\nb\n",
		"a\r\nb\nc":          "a\nb\nc",
		"\tcafé 日本":          "\tcafé 日本",
		"a\x1b:q!\rb":        "a:q!\nb",
		"a\x00\x07\x7fb":     "ab",
		"x\u0085y\u009bz":    "xyz", // C1 controls
		"":                   "",
		"\x1b[201~":          "[201~",
		"- one\r  - two\r\r": "- one\n  - two\n\n",
	} {
		if got := PastedText(raw); got != want {
			t.Errorf("PastedText(%q) = %q, want %q", raw, got, want)
		}
	}
	if got := OnOneLine("\n  a\n\tb\n\n"); got != "  a  b" {
		t.Errorf("OnOneLine = %q", got)
	}
}
