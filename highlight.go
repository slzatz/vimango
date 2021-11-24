package main

import (
	"io"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
)

func Highlight(w io.Writer, source, lexer, formatter, style string) error {
	l := lexers.Get(lexer)

	// Commenting this out did not fix multiline comments enclosed by
	/* ... */
	l = chroma.Coalesce(l)

	f := formatters.Get(formatter)

	s := styles.Get(style)

	it, err := l.Tokenise(nil, source)
	if err != nil {
		return err
	}
	return f.Format(w, s, it)
}
