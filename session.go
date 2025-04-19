package main

import (
	"github.com/alecthomas/chroma/v2"
)

type Session struct {
  activeEditor     *Editor
	editorMode       bool
	imagePreview     bool
	imgSizeY         int
	fts_search_terms string
	style            [8]string
  markdown_style   *chroma.Style
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

/*
func (s *Session) NewEditor() *Editor {
	 ae := &Editor{
		cx:                 0, //actual cursor x position (takes into account any scroll/offset)
		cy:                 0, //actual cursor y position ""
		fc:                 0, //'file' x position as defined by reading sqlite text into rows vector
		fr:                 0, //'file' y position ""
		lineOffset:         0, //the number of lines of text at the top scrolled off the screen
		mode:               NORMAL,
		command:            "",
		command_line:       "",
		firstVisibleRow:    0,
		highlightSyntax:    true, // applies to golang, c++ etc. and markdown
		numberLines:        true,
		redraw:             false,
		output:             nil,
		left_margin_offset: LEFT_MARGIN_OFFSET, // 0 if not syntax highlighting b/o synt high =>line numbers
		modified:           false,
    Screen:             a.Screen,
    Session:            a.Session,
    Database:           a.Database,
	}
  s.activeEditor = ae
  return ae
}
*/
