package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Writers racing on one file leave one writer's data, whole, and no temporary
// files. With the single ".tmp" name used before, two writers shared the
// temporary file and the result could be part of each.
func TestWriteFileAtomicUnderContention(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "registers.json")
	const writers, rounds = 8, 40

	var wg sync.WaitGroup
	for w := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// each writer's data has its own length and its own byte
			data := bytes.Repeat([]byte{byte('a' + w)}, 50_000+w*10_000)
			for range rounds {
				if err := writeFileAtomic(path, data, 0600); err != nil {
					t.Error(err)
					return
				}
			}
		}()
	}
	// a reader, all the while, never sees a partial file
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	whole := func(data []byte) bool {
		if len(data) == 0 {
			return false
		}
		w := int(data[0] - 'a')
		return w >= 0 && w < writers && len(data) == 50_000+w*10_000 && bytes.Count(data, data[:1]) == len(data)
	}
	for reading := true; reading; {
		select {
		case <-done:
			reading = false
		default:
		}
		if data, err := os.ReadFile(path); err == nil && !whole(data) {
			t.Fatalf("read a file that is no writer's data: %d bytes starting %q", len(data), data[:min(len(data), 1)])
		}
	}

	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("%d files left in the directory, want only the one written", len(entries))
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0600 {
		t.Errorf("the file's mode is %v (%v), want 0600", info.Mode().Perm(), err)
	}
}

// newTestImageCache is an image cache in a directory of its own.
func newTestImageCache(t *testing.T) *ImageCache {
	t.Helper()
	dir := t.TempDir()
	return &ImageCache{
		cacheDir:   dir,
		indexFile:  filepath.Join(dir, "cache_index.json"),
		maxEntries: maxCacheEntries,
		maxBytes:   maxCacheBytes,
		index:      CacheIndex{Version: 1, Entries: map[string]CacheEntry{}},
	}
}

// A cache hit does not rewrite the index, unless the last use on record is old
// enough to matter to eviction. Every hit used to: once per image per preview,
// from each process.
func TestCacheHitRewritesTheIndexOnlyWhenStale(t *testing.T) {
	c := newTestImageCache(t)
	const url = "gdrive:abcdefghijklmnopqrstuvwxyz0123"
	if err := c.StoreCachedImageData(url, "aGVsbG8=", 10, 10); err != nil {
		t.Fatal(err)
	}
	// an index file nothing has touched since: any rewrite gives it a new time
	untouched := time.Now().Add(-24 * time.Hour)
	if err := os.Chtimes(c.indexFile, untouched, untouched); err != nil {
		t.Fatal(err)
	}
	rewritten := func() bool {
		t.Helper()
		info, err := os.Stat(c.indexFile)
		if err != nil {
			t.Fatal(err)
		}
		return !info.ModTime().Equal(untouched)
	}

	for range 3 {
		if data, _, _, ok := c.GetCachedImageData(url); !ok || data != "aGVsbG8=" {
			t.Fatalf("cache miss on an image just stored (%q)", data)
		}
	}
	if rewritten() {
		t.Error("a hit on an entry used a moment ago rewrote the index")
	}

	// last used two hours ago, as a later process finds it
	key := c.generateCacheKey(url)
	entry := c.index.Entries[key]
	entry.LastAccessed = time.Now().Add(-2 * time.Hour)
	c.index.Entries[key] = entry

	if _, _, _, ok := c.GetCachedImageData(url); !ok {
		t.Fatal("cache miss")
	}
	if !rewritten() {
		t.Fatal("a hit on an entry last used two hours ago did not update the index")
	}
	var onDisk CacheIndex
	data, _ := os.ReadFile(c.indexFile)
	if err := json.Unmarshal(data, &onDisk); err != nil {
		t.Fatal(err)
	}
	if age := time.Since(onDisk.Entries[key].LastAccessed); age > time.Minute {
		t.Errorf("the index records a last use %v ago, want now", age)
	}
}

// A process killed mid-write leaves its temporary file; the next start clears
// those, and only those.
func TestStaleTempFilesAreRemoved(t *testing.T) {
	dir := t.TempDir()
	old := time.Now().Add(-2 * time.Hour)
	for name, stale := range map[string]bool{"cache_index.json.123.tmp": true, "fresh.456.tmp": false, "abc.b64": true} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		if stale {
			os.Chtimes(p, old, old)
		}
	}
	removeStaleTempFiles(dir)
	for name, want := range map[string]bool{"cache_index.json.123.tmp": false, "fresh.456.tmp": true, "abc.b64": true} {
		if _, err := os.Stat(filepath.Join(dir, name)); (err == nil) != want {
			t.Errorf("%s: kept=%v, want %v", name, err == nil, want)
		}
	}
}
