package main

// chroma is being used for syntax highlighting

import (
	"io"
	"os"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

func selectMDStyle(style string) (*chroma.Style, error) {
	//	style, ok := styles.Registry[cli.Style]
	//	if ok {
	//		return style, nil
	//	}
	r, err := os.Open(style)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return chroma.NewXMLStyle(r)
}

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

func Highlight2(w io.Writer, source string, lexer string, formatter string, style *chroma.Style) error {
	l := lexers.Get(lexer)

	// Commenting this out did not fix multiline comments enclosed by
	/* ... */
	l = chroma.Coalesce(l)

	f := formatters.Get(formatter)

	//s := styles.Get(style)

	it, err := l.Tokenise(nil, source)
	if err != nil {
		return err
	}
	return f.Format(w, style, it)
}
