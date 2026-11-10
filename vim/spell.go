package vim

/*
#include "src/libvim.h"

extern void goSpellSuggestCallback(void);
*/
import "C"

import "unsafe"

// SpellErrorType mirrors libvim's spellErrorType_T (vim's SpellBad, SpellCap,
// SpellRare and SpellLocal highlight groups).
type SpellErrorType int

const (
	SpellBad   SpellErrorType = C.SPELL_ERROR_BAD   // word not recognized
	SpellCaps  SpellErrorType = C.SPELL_ERROR_CAPS  // should start with a capital
	SpellRare  SpellErrorType = C.SPELL_ERROR_RARE  // rarely used word
	SpellLocal SpellErrorType = C.SPELL_ERROR_LOCAL // word from another region
)

// SpellError is one misspelling. Line is 1-based; Col and Len are in bytes,
// so line[Col:Col+Len] is the word.
type SpellError struct {
	Line int
	Col  int
	Len  int
	Type SpellErrorType
}

// void vimSpellGetErrors(buf_T *buf, linenr_T start, linenr_T end, int *num, spellError_T **errors);
//
// SpellErrors returns the misspellings in lines start..end (1-based,
// inclusive) of the buffer. Like vim's own highlighting it finds nothing
// unless the buffer is current, 'spell' is set and a dictionary is loaded.
func (b *Buffer) SpellErrors(start, end int) []SpellError {
	var n C.int
	var errs *C.spellError_T
	C.vimSpellGetErrors(b.buf, C.linenr_T(start), C.linenr_T(end), &n, &errs)
	if errs == nil {
		return nil
	}
	defer C.vim_free(unsafe.Pointer(errs))

	result := make([]SpellError, 0, int(n))
	for _, e := range unsafe.Slice(errs, int(n)) {
		result = append(result, SpellError{
			Line: int(e.lnum),
			Col:  int(e.col),
			Len:  int(e.len),
			Type: SpellErrorType(e._type),
		})
	}
	return result
}

// int vimSpellGetSuggestions(char_u **badWord, int *num_suggestions, char_u ***suggestions);
//
// GetSpellSuggestions returns the word z= would change (the misspelling under
// or after the cursor, else the word under it) and the replacements it
// offers, best first; "{n}z=" applies suggestions[n-1]. ok is false when
// spell checking is off or there is no word at the cursor.
func GetSpellSuggestions() (word string, suggestions []string, ok bool) {
	var bad *C.char_u
	var n C.int
	var sugs **C.char_u
	if C.vimSpellGetSuggestions(&bad, &n, &sugs) != C.OK {
		return "", nil, false
	}
	if bad != nil {
		word = C.GoString((*C.char)(unsafe.Pointer(bad)))
		C.vim_free(unsafe.Pointer(bad))
	}
	if sugs != nil {
		for _, s := range unsafe.Slice(sugs, int(n)) {
			suggestions = append(suggestions, C.GoString((*C.char)(unsafe.Pointer(s))))
			C.vim_free(unsafe.Pointer(s))
		}
		C.vim_free(unsafe.Pointer(sugs))
	}
	return word, suggestions, true
}

// void vimSetSpellSuggestCallback(VoidCallback callback);
//
// enableSpellSuggestCallback makes z= without a count set the request read by
// TakeSpellSuggestRequest, instead of beeping. vim itself would show a
// numbered prompt, which blocks; the application shows the list instead.
// InitializeVim calls it.
func enableSpellSuggestCallback() {
	C.vimSetSpellSuggestCallback(C.VoidCallback(C.goSpellSuggestCallback))
}
