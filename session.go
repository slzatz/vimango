package main

import (
	"github.com/alecthomas/chroma/v2"
	"google.golang.org/api/drive/v3"
)

type Session struct {
	activeEditor     *Editor
	editorMode       bool
	editorOnly       bool // --editor boot mode: set once at startup, never cleared
	imagePreview     bool
	imgSizeY         int
	fts_search_terms string
	style            [8]string
	markdown_style   *chroma.Style
	styleIndex       int
	Editors          []*Editor      //slice of all active Editor
	googleDrive      *drive.Service // Google Drive service for file operations
}
