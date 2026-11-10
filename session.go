package main

import (
	"github.com/alecthomas/chroma/v2"
	"google.golang.org/api/drive/v3"
)

type Session struct {
	editor           *Editor // the open note's editor; nil when none is open. One note at a time
	editorMode       bool
	editorOnly       bool // --editor boot mode: set once at startup, never cleared
	imgSizeY         int
	fts_search_terms string
	markdown_style   *chroma.Style
	googleDrive      *drive.Service // Google Drive service for file operations
}
