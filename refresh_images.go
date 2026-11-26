package main

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// DetermineRefreshImages returns the note id passed to --refresh-images, or
// -1 if the flag is absent.
func DetermineRefreshImages(args []string) int {
	return noteIDFlag(args, "--refresh-images")
}

// RunRefreshImages downloads note <id>'s Google Drive images again, replacing
// what the image cache holds for each, and returns a process exit code.
// Headless, as --render-html is.
//
// It exists because the cache never asks Drive whether a file changed: it is
// keyed by the file's id, so an image edited on Drive after it was cached --
// rotated, cropped, replaced -- shows as it was until it is evicted. Asking
// Drive on every render would cost a round trip per image per preview, and
// fail offline; asking when told to costs nothing.
func RunRefreshImages(id int) int {
	// as in RunRenderHTML: the HEIC worker outlives this process unless
	// it is killed, on a normal exit and on a signal alike
	defer ShutdownHEICDecoder()
	killHEICWorkerOnSignal()

	return refreshNoteImages(id, os.Stdout, os.Stderr)
}

// refreshNoteImages is --refresh-images itself. Its stdout is for the host:
// a first line "refreshed N of M" -- M the note's distinct Drive images, N
// those downloaded and cached anew -- and then one line per image that could
// not be, "<address>: <why>". An image that fails keeps its cached copy:
// a refresh made offline, or of a file no longer shared, loses nothing.
//
// The exit code is 1 only when the note itself could not be read; images
// that failed are a result, said on stdout.
func refreshNoteImages(id int, stdout, stderr io.Writer) int {
	config, err := loadConfig("config.json")
	if err != nil {
		fmt.Fprintf(stderr, "Error: could not open databases: %v\n", err)
		return 1
	}
	notes, err := openStore(config)
	if err != nil {
		fmt.Fprintf(stderr, "Error: could not open databases: %v\n", err)
		return 1
	}
	defer notes.Close()

	_, note, err := notes.TitleAndNote(id)
	if err != nil {
		fmt.Fprintf(stderr, "Error: no note with id %d: %v\n", id, err)
		return 1
	}

	var r htmlRenderer
	r.images, r.cache = driveForRender(stderr)
	refreshed, failures := r.refreshImages(note)
	fmt.Fprintf(stdout, "refreshed %d of %d\n", refreshed, refreshed+len(failures))
	for _, f := range failures {
		fmt.Fprintln(stdout, f)
	}
	return 0
}

// refreshImages downloads each distinct Drive image in the note and caches
// it in place of what was there. It returns how many it refreshed, and a line
// for each it could not.
func (r htmlRenderer) refreshImages(note string) (refreshed int, failures []string) {
	seen := map[string]bool{}
	for _, img := range imagesToInline(note) {
		url := img.url
		// One image may appear in both address forms, or twice: the cache
		// knows it by its file id, and so does this.
		key := url
		if fileID, err := ExtractFileID(url); err == nil {
			key = fileID
		}
		if seen[key] {
			continue
		}
		seen[key] = true

		if err := r.refreshImage(url); err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", url, err))
			continue
		}
		refreshed++
	}
	return refreshed, failures
}

// refreshImage downloads one image and caches it, replacing what the cache
// held only once the download has worked.
func (r htmlRenderer) refreshImage(url string) error {
	if r.cache == nil {
		return errors.New("there is no image cache to refresh")
	}
	dataURI, err := r.fetch(url)
	if err != nil {
		return err
	}
	return r.cache.StoreCachedImage(url, dataURI)
}
