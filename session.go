package main

import (
	"github.com/alecthomas/chroma/v2"
	"google.golang.org/api/drive/v3"
)

type Session struct {
	// editor is the open note's editor, or nil when none is open: one note
	// at a time. It is also where keys go -- to the editor while there is
	// one, to the organizer otherwise. Nothing else records that.
	editor           *Editor
	editorOnly       bool // --editor boot mode: set once at startup, never cleared
	imgSizeY         int
	fts_search_terms string
	markdown_style   *chroma.Style
	googleDrive      *drive.Service // Google Drive service for file operations
}
