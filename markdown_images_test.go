package main

import (
	"reflect"
	"testing"
)

// The TUI transmits the images extractImageURLs lists and hands them to
// glamour by position, so the list must be exactly the images glamour draws:
// in order, and none that markdown shows as text. One in backticks used to be
// listed, and every image after it got its predecessor's picture.
func TestExtractImageURLsSkipsCode(t *testing.T) {
	note := "---\nmarp: true\n---\n" +
		"# Slide\n![one](https://example.com/1.png)\n" +
		"`![ticked](gdrive:tickedTICKEDtickedTICKED01)`\n" +
		"![two](gdrive:twoTWOtwoTWOtwoTWOtwoTWO02)\n\n" +
		"```\n![fenced](https://example.com/f.png)\n```\n\n" +
		"    ![indented](https://example.com/i.png)\n\n" +
		"<!-- ![commented](https://example.com/c.png) -->\n\n" +
		"Text <span>![inline html](x.png)</span> ![three](https://example.com/3.png)\n"
	got := extractImageURLs(note)
	want := []string{"https://example.com/1.png", "gdrive:twoTWOtwoTWOtwoTWOtwoTWO02", "x.png", "https://example.com/3.png"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("extractImageURLs:\n got %q\nwant %q", got, want)
	}
	if got := extractImageURLs("no images here"); len(got) != 0 {
		t.Errorf("a note with no images: %q", got)
	}
}
