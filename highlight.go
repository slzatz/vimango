package main

// chroma is being used for syntax highlighting

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
)

// MDFormatter is the formatter name to hand Highlight2 for markdown. It is
// chroma's terminal16m plus the one attribute chroma cannot express: SGR 9,
// real strikethrough for ~~struck~~ text.
//
// chroma's StyleEntry carries only bold/italic/underline, and its terminal
// formatters emit only those, so no style XML can ask for strikethrough --
// which is why GenericDeleted in gruvbox_mod.xml used to fake it with a red
// background. ghostty draws SGR 9 natively (its terminfo advertises
// smxx=\E[9m, rmxx=\E[29m), and ghostty is what runs the plain terminal,
// vimango_ghostty, and the hybrid app's embedded surface.
const MDFormatter = "terminal16m-md"

const (
	strikeSGR   = "\x1b[9m"
	strikeDelim = "~~"
)

func init() {
	formatters.Register(MDFormatter, chroma.FormatterFunc(formatMarkdownTTY))
}

var crOrCRLF = regexp.MustCompile(`\r?\n`)

// clearStyleBackground mirrors chroma's unexported formatters.clearBackground:
// the terminal owns its own background, so the style's global one is dropped.
// Per-token backgrounds are untouched.
func clearStyleBackground(style *chroma.Style) *chroma.Style {
	builder := style.Builder()
	bg := builder.Get(chroma.Background)
	bg.Background = 0
	bg.NoInherit = true
	builder.AddEntry(chroma.Background, bg)
	cleared, err := builder.Build()
	if err != nil {
		return style
	}
	return cleared
}

// writeStyledToken resets at the end of every line and re-applies formatting
// on the next one, as chroma's terminal16m does. drawCodeRows splits this
// output on "\n" and draws each line with its own cursor positioning, so no
// line may depend on styling left open by the line above it.
func writeStyledToken(w io.Writer, formatting, text string) {
	if formatting == "" {
		fmt.Fprint(w, text)
		return
	}
	afterLastNewline := 0
	for _, indices := range crOrCRLF.FindAllStringIndex(text, -1) {
		newlineStart, afterNewline := indices[0], indices[1]
		fmt.Fprint(w, formatting, text[afterLastNewline:newlineStart], "\x1b[0m", text[newlineStart:afterNewline])
		afterLastNewline = afterNewline
	}
	if afterLastNewline < len(text) {
		fmt.Fprint(w, formatting, text[afterLastNewline:], "\x1b[0m")
	}
}

// formatMarkdownTTY is chroma's trueColourFormatter with strikethrough added
// for GenericDeleted, which is what the markdown lexer tags ~~text~~ as. The
// editor shows raw markdown, so the tildes stay on screen, but the line stops
// at them: they delimit the struck run rather than belonging to it.
func formatMarkdownTTY(w io.Writer, style *chroma.Style, it chroma.Iterator) error {
	style = clearStyleBackground(style)
	for token := it(); token != chroma.EOF; token = it() {
		entry := style.Get(token.Type)
		struck := token.Type == chroma.GenericDeleted
		if entry.IsZero() && !struck {
			fmt.Fprint(w, token.Value)
			continue
		}

		formatting := ""
		if entry.Bold == chroma.Yes {
			formatting += "\x1b[1m"
		}
		if entry.Underline == chroma.Yes {
			formatting += "\x1b[4m"
		}
		if entry.Italic == chroma.Yes {
			formatting += "\x1b[3m"
		}
		if entry.Colour.IsSet() {
			formatting += fmt.Sprintf("\x1b[38;2;%d;%d;%dm", entry.Colour.Red(), entry.Colour.Green(), entry.Colour.Blue())
		}
		if entry.Background.IsSet() {
			formatting += fmt.Sprintf("\x1b[48;2;%d;%d;%dm", entry.Background.Red(), entry.Background.Green(), entry.Background.Blue())
		}

		if struck {
			// The delimiters mark the text up, they are not struck text
			// themselves, so the line stops at the tildes on both ends.
			if inner, ok := splitStrikeDelimiters(token.Value); ok {
				writeStyledToken(w, formatting, strikeDelim)
				writeStyledToken(w, formatting+strikeSGR, inner)
				writeStyledToken(w, formatting, strikeDelim)
				continue
			}
			// Not shaped like ~~text~~ after all; strike the lot rather
			// than silently dropping the attribute.
			formatting += strikeSGR
		}

		writeStyledToken(w, formatting, token.Value)
	}
	return nil
}

// splitStrikeDelimiters peels the ~~ off a GenericDeleted token, whose value
// the markdown lexer hands over with the delimiters included.
func splitStrikeDelimiters(value string) (inner string, ok bool) {
	if len(value) <= 2*len(strikeDelim) {
		return "", false
	}
	if !strings.HasPrefix(value, strikeDelim) || !strings.HasSuffix(value, strikeDelim) {
		return "", false
	}
	return value[len(strikeDelim) : len(value)-len(strikeDelim)], true
}

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
