package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

// --refresh-images replaces a cached image with what Drive has now, once per
// distinct image whatever form the note names it in, and leaves the cached
// copy of one it could not download alone.
func TestRefreshImages(t *testing.T) {
	const a, b, c = "gdrive:aaaaAAAAaaaaAAAAaaaaAAAAaaaa01", "gdrive:bbbbBBBBbbbbBBBBbbbbBBBBbbbb02", "gdrive:ccccCCCCccccCCCCccccCCCCcccc03"
	const aLink = "https://drive.google.com/file/d/aaaaAAAAaaaaAAAAaaaaAAAAaaaa01/view"
	cache := newTestImageCache(t)
	for _, url := range []string{a, c} {
		if err := cache.StoreCachedImage(url, "data:image/png;base64,OLD"); err != nil {
			t.Fatal(err)
		}
	}

	var asked []string
	r := htmlRenderer{cache: cache, download: func(url string) (string, error) {
		asked = append(asked, url)
		if url == c {
			return "", errors.New("offline")
		}
		return "data:image/jpeg;base64,NEW", nil
	}}
	note := "![one](" + a + ")\n![again](" + aLink + ")\n![two](" + b + ")\n![three](" + c + ")\n![one more](" + a + ")\n"
	refreshed, failures := r.refreshImages(note)

	if refreshed != 2 || len(failures) != 1 || failures[0] != c+": offline" {
		t.Errorf("refreshed %d, failures %q; want 2, and c offline", refreshed, failures)
	}
	if len(asked) != 3 {
		t.Errorf("downloaded %q; want each image once", asked)
	}
	for url, want := range map[string]string{a: "NEW", b: "NEW", c: "OLD"} {
		got, ok := cache.GetCachedImage(url)
		if !ok || !strings.HasSuffix(got, want) {
			t.Errorf("%s cached as %q, %v; want ...%s", url, got, ok, want)
		}
	}

	// with no cache nothing is refreshed, and nothing is downloaded either
	asked = nil
	r.cache = nil
	if refreshed, failures := r.refreshImages(note); refreshed != 0 || len(failures) != 3 || len(asked) != 0 {
		t.Errorf("no cache: refreshed %d, failures %q, downloaded %q", refreshed, failures, asked)
	}
}

// A full cache replacing its least recently used entry: that entry is the
// one being stored, so the eviction must not take it, and the file just
// written with it.
func TestStoreReplacesAnEntryInAFullCache(t *testing.T) {
	const a, b = "gdrive:aaaaAAAAaaaaAAAAaaaaAAAAaaaa01", "gdrive:bbbbBBBBbbbbBBBBbbbbBBBBbbbb02"
	cache := newTestImageCache(t)
	cache.maxEntries = 2
	for _, url := range []string{a, b} {
		if err := cache.StoreCachedImage(url, "OLD"); err != nil {
			t.Fatal(err)
		}
	}
	if err := cache.StoreCachedImage(a, "NEW"); err != nil {
		t.Fatal(err)
	}
	if got, ok := cache.GetCachedImage(a); !ok || got != "NEW" {
		t.Errorf("a: %q, %v; want NEW", got, ok)
	}
	if got, ok := cache.GetCachedImage(b); !ok || got != "OLD" {
		t.Errorf("b: %q, %v; want OLD, still cached", got, ok)
	}
}

// What the host reads: the count line and a line per failure on stdout, exit
// 0 however many images failed, and exit 1 with nothing on stdout only when
// the note could not be read. Here nobody is signed in to Drive, so the image
// fails, and keeps what was cached.
func TestRefreshNoteImages(t *testing.T) {
	const a = "gdrive:aaaaAAAAaaaaAAAAaaaaAAAAaaaa01"
	id := renderWorkspace(t, "", "A note", "![one]("+a+")\n")
	cache, err := NewImageCache()
	if err != nil {
		t.Fatal(err)
	}
	if err := cache.StoreCachedImage(a, "data:image/png;base64,OLD"); err != nil {
		t.Fatal(err)
	}

	var stdout, stderr bytes.Buffer
	if code := refreshNoteImages(id, &stdout, &stderr); code != 0 {
		t.Fatalf("exit code %d; stderr: %s", code, stderr.String())
	}
	lines := strings.Split(strings.TrimSuffix(stdout.String(), "\n"), "\n")
	if len(lines) != 2 || lines[0] != "refreshed 0 of 1" || !strings.HasPrefix(lines[1], a+": ") || !strings.Contains(lines[1], "not configured") {
		t.Errorf("stdout: %q", stdout.String())
	}
	reread, err := NewImageCache()
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := reread.GetCachedImage(a); !ok || got != "data:image/png;base64,OLD" {
		t.Errorf("the cached copy: %q, %v; want it kept", got, ok)
	}

	stdout.Reset()
	if code := refreshNoteImages(renderWorkspace(t, "", "None", "no images"), &stdout, &stderr); code != 0 || stdout.String() != "refreshed 0 of 0\n" {
		t.Errorf("a note with no images: exit %d, stdout %q", code, stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := refreshNoteImages(id+1000, &stdout, &stderr); code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "no note with id") {
		t.Errorf("a note that is not there: exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
}

func TestRefreshImagesFlag(t *testing.T) {
	args := []string{"vimango", "--refresh-images", "5"}
	if got := DetermineRefreshImages(args); got != 5 {
		t.Errorf("--refresh-images 5: note %d", got)
	}
	if DetermineRenderHTML(args) != -1 || DetermineRenderMarkdown(args) != -1 {
		t.Error("--refresh-images read as a render flag")
	}
}
