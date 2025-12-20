package main

import (
	"github.com/alecthomas/chroma/v2"
	"google.golang.org/api/drive/v3"
)

type Session struct {
	activeEditor     *Editor
	editorMode       bool
	imagePreview     bool
	imgSizeY         int
	fts_search_terms string
	style            [8]string
	markdown_style   *chroma.Style
	styleIndex       int
	//Windows          []Window       //slice of Window interfaces (Output, Editor)
	Windows     []*Editor      //slice of Window interfaces (Output, Editor)
	googleDrive *drive.Service // Google Drive service for file operations
}

/*
func (s *Session) numberOfEditors() int {
	i := 0
	for _, w := range s.Windows {
		if _, ok := w.(*Editor); ok {
			i++
		}
	}
	return i
}

func (s *Session) editors() []*Editor {
	eds := []*Editor{}
	for _, w := range s.Windows {
		if e, ok := w.(*Editor); ok {
			eds = append(eds, e)
		}
	}
	return eds
}
*/
