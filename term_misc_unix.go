//go:build !windows

package main

import (
	"io"
	"os"
	"syscall"
)

func findPtyDevByStat(pStat *syscall.Stat_t) (string, error) {

	for _, devDir := range []string{"/dev/pts", "/dev"} {

		fd, E := os.Open(devDir)
		if os.IsNotExist(E) {
			continue
		} else if E != nil {
			return "", E
		}
		defer fd.Close()

		for {

			fis, e2 := fd.Readdir(256)
			for _, fi := range fis {

				if fi.IsDir() {
					continue
				}

				if s, ok := fi.Sys().(*syscall.Stat_t); ok && (pStat.Dev == s.Dev) && (pStat.Rdev == s.Rdev) && (pStat.Ino == s.Ino) {
					return devDir + "/" + fi.Name(), nil
				}
			}

			if e2 == io.EOF {
				break
			}

			if e2 != nil {
				return "", e2
			}
		}
	}

	return "", os.ErrNotExist
}

func GetTtyPath(pF *os.File) (string, error) {

	info, E := pF.Stat()
	if E != nil {
		return "", E
	}

	if sys, ok := info.Sys().(*syscall.Stat_t); ok {

		if path, e := findPtyDevByStat(sys); e == nil {
			return path, nil
		} else if os.IsNotExist(e) {
			return "", E_NON_TTY
		} else {
			return "", e
		}
	}

	return "", nil
}