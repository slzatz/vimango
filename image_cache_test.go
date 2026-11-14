package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
