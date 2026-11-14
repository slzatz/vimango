package atomicfile

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// Writers racing on one file leave one writer's data, whole, and no temporary
// files. With the single ".tmp" name used before, two writers shared the
// temporary file and the result could be part of each.
func TestWriteUnderContention(t *testing.T) {
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
				if err := Write(path, data, 0600); err != nil {
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
	RemoveStale(dir)
	for name, want := range map[string]bool{"cache_index.json.123.tmp": false, "fresh.456.tmp": true, "abc.b64": true} {
		if _, err := os.Stat(filepath.Join(dir, name)); (err == nil) != want {
			t.Errorf("%s: kept=%v, want %v", name, err == nil, want)
		}
	}
}
