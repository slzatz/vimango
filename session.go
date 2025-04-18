package main

import (
	"github.com/alecthomas/chroma/v2"
)

type Session struct {
	editorMode       bool
	imagePreview     bool
	imgSizeY         int
	fts_search_terms string
	style      [8]string
  markdown_style *chroma.Style
	styleIndex int
}

func (s *Session) numberOfEditors() int {
	i := 0
	for _, w := range app.Windows {
		if _, ok := w.(*Editor); ok {
			i++
		}
	}
	return i
}

func (s *Session) editors() []*Editor {
	eds := []*Editor{}
	for _, w := range app.Windows {
		if e, ok := w.(*Editor); ok {
			eds = append(eds, e)
		}
	}
	return eds
}

