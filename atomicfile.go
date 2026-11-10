package main

import (
	"os"
	"path/filepath"
	"time"
)

// writeFileAtomic replaces the file at path with data: a reader, in this
// process or another, sees the old contents or the new, never part of one.
// The data goes to a temporary file beside path, which is then renamed over
// it.
//
// The temporary file's name is unique to this write. Several vimango
// processes share these files -- the TUI, an --editor per pop-out, a
// --render-html per preview -- and with one fixed name ("x.tmp", as it was
// until 2026-11-10) two of them writing at once wrote into the same temporary
// file, and the rename could publish a mixture of the two.
func writeFileAtomic(path string, data []byte, perm os.FileMode) (err error) {
	f, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(f.Name())
		}
	}()
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Chmod(perm); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}

// removeStaleTempFiles deletes what writeFileAtomic left in dir when its
// process was killed between creating the temporary file and renaming it:
// anything named *.tmp and older than an hour, which no write still owns.
func removeStaleTempFiles(dir string) {
	names, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	for _, name := range names {
		if info, err := os.Stat(name); err == nil && time.Since(info.ModTime()) > time.Hour {
			os.Remove(name)
		}
	}
}
