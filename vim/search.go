package vim

/*
#include "libvim_api.h"
*/
import "C"

import "unsafe"

// SearchMatch is one match of the current search pattern. Lines are 1-based;
// columns are byte offsets, and EndCol is the offset just past the match. A
// pattern with \n in it can match across lines.
type SearchMatch struct {
	Line, Col       int
	EndLine, EndCol int
}

// SearchMatches returns the matches of the last search pattern in lines
// first..last (1-based, inclusive), in order. It returns them whether or not
// highlighting is on; SearchHighlighting says whether to show them.
//
// void vimSearchGetHighlights(buf_T *buf, linenr_T start_lnum, linenr_T end_lnum,
//
//	int *num_highlights, searchHighlight_T **highlights);
func (b *Buffer) SearchMatches(first, last int) []SearchMatch {
	var n C.int
	var hl *C.searchHighlight_T
	C.vimSearchGetHighlights(b.buf, C.linenr_T(first), C.linenr_T(last), &n, &hl)
	if hl == nil {
		return nil
	}
	defer C.vim_free(unsafe.Pointer(hl))

	matches := make([]SearchMatch, 0, int(n))
	for _, h := range unsafe.Slice(hl, int(n)) {
		matches = append(matches, SearchMatch{int(h.start.lnum), int(h.start.col), int(h.end.lnum), int(h.end.col)})
	}
	return matches
}

// SearchPattern returns the last search pattern, "" if there has been none.
//
// char_u *vimSearchGetPattern();
func SearchPattern() string {
	p := C.vimSearchGetPattern()
	if p == nil {
		return ""
	}
	return goString(p)
}

// SearchHighlighting reports whether vim would be highlighting the matches of
// the last search right now: 'hlsearch' is set, and :nohlsearch has not hidden
// them since the last search command. It is vim's own state (what v:hlsearch
// reports), so "/", "n", "*", ":noh" and ":set nohlsearch" all move it without
// the application tracking any of them.
//
// This function and the fix that makes SearchMatches return every match came
// in the same libvim commit (fork a036ecafe), so a libvim.a from before it
// fails to link here rather than quietly highlighting every other match.
//
// int vimSearchGetHighlightEnabled(void);
func SearchHighlighting() bool {
	return C.vimSearchGetHighlightEnabled() != 0
}
