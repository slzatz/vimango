package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"html/template"
	"image/jpeg"
	"image/png"
	"log"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
)

// Default implementation that will be overridden by build-specific files
var isWebviewAvailableDefault = false

// Global image cache instance
var globalImageCache *ImageCache

// initImageCache initializes the global image cache
func initImageCache() {
	if globalImageCache == nil {
		var err error
		globalImageCache, err = NewImageCache()
		if err != nil {
			log.Printf("Warning: Failed to initialize image cache: %v", err)
			globalImageCache = nil
		}
	}
}

// IsWebviewAvailable returns true if webview is available
func IsWebviewAvailable() bool {
	return isWebviewAvailableDefault
}

// IsWebviewRunning returns true if a webview is currently running
// This is a stub that will be overridden by build-specific files
// Only declare this for non-CGO builds
var isWebviewRunning = false

// OpenNoteInWebview opens a note in a webview window or falls back to browser
// This function signature will be implemented by build-specific files
func openNoteInWebview(title, htmlContent string) error {
	if isWebviewAvailableDefault {
		return OpenNoteInWebview(title, htmlContent)
	}

	// Fallback - should not be reached due to build-specific implementations
	ShowWebviewUnavailableMessage()
	return fmt.Errorf("webview not available")
}

// defaultAccent is the blue used for the blockquote bar and list markers
// when config.json has no webview.accent entry.
const defaultAccent = "#3498db"

var accentHexRe = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

// previewAccent resolves config.json's webview.accent to a CSS color:
// "" → the default blue, "none"/"off" → currentColor (markers and the
// blockquote bar take the surrounding text color), any #rgb/#rrggbb →
// itself. Anything else falls back to the default rather than injecting
// arbitrary text into the stylesheet.
func previewAccent() string {
	if app == nil || app.Config == nil {
		return defaultAccent
	}
	return resolveAccent(app.Config.Webview.Accent, "webview.accent", defaultAccent)
}

// previewCodeAccent resolves config.json's webview.code_accent — the
// color of inline code and code blocks (fenced or indented). Its own
// key rather than a share of webview.accent: code and the
// blockquote/marker accent are different kinds of emphasis, and tuning
// one shouldn't drag the other. Unset it falls back to whatever the
// accent resolved to, so the two match until told otherwise.
func previewCodeAccent() string {
	accent := previewAccent()
	if app == nil || app.Config == nil {
		return accent
	}
	return resolveAccent(app.Config.Webview.CodeAccent, "webview.code_accent", accent)
}

// resolveAccent is the shared vocabulary behind both keys. `fallback`
// is what "" and an unusable value resolve to.
func resolveAccent(configured, key, fallback string) string {
	value := strings.TrimSpace(configured)
	switch strings.ToLower(value) {
	case "":
		return fallback
	case "none", "off":
		return "currentColor"
	}
	if accentHexRe.MatchString(value) {
		return value
	}
	log.Printf("Warning: ignoring invalid %s %q (want #rgb, #rrggbb, or \"none\")", key, value)
	return fallback
}

// RenderNoteAsHTML converts a note's markdown content to HTML for webview display.
// standalone selects the document style. The TUI webview window is a
// standalone document: <h1> title (the window has no other title chrome)
// and a centered reading column. The hybrid app's --render-html preview is
// an embedded pane: no <h1> (the host shows the title natively — it is
// metadata, not part of the note) and left-aligned content, since
// auto-centering inside a pane reads as a large wasted indent.
func RenderNoteAsHTML(title, markdownContent string, standalone bool) (string, error) {
	// Pre-process markdown to handle Google Drive images
	processedMarkdown, err := preprocessMarkdownImages(markdownContent)
	if err != nil {
		return "", fmt.Errorf("failed to preprocess markdown images: %v", err)
	}

	// Convert markdown to HTML using goldmark
	htmlContent := convertMarkdownToHTML(processedMarkdown)

	// Wrap in basic HTML template
	htmlTemplate := `<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>{{.Title}}</title>
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            line-height: 1.6;
            color: #333;
            max-width: 800px;
            margin: {{if .Standalone}}0 auto{{else}}0{{end}};
            padding: {{if .Standalone}}20px{{else}}4px 20px 20px{{end}};
            background-color: #fff;
        }
        {{if not .Standalone}}/* Embedded pane: the host's native title bar sits directly
           above, so drop the first element's default top margin too. */
        #content > :first-child {
            margin-top: 0;
        }
        {{end}}
        h1, h2, h3, h4, h5, h6 {
            color: #2c3e50;
            /* Headings are large; body's 1.6 leading makes wrapped
               headlines look double-spaced. */
            line-height: 1.20;
            /* Default heading margin-bottom is ~21px at h1, and it
               collapses against the following block's own top margin
               (16px on p/ul) — the larger wins, so both halves have to
               come down to tighten the gap. Space *above* a heading is
               left alone: the asymmetry is what groups a heading with
               the text it introduces. */
            margin-bottom: 0.5rem;
        }
        h1 + *, h2 + *, h3 + *, h4 + *, h5 + *, h6 + * {
            margin-top: 0;
        }
        /* The UA ladder runs h4/h5/h6 at 1em/0.83em/0.67em -- h4 exactly
           body size, the last two smaller than the text they head. That
           left h4 leaning entirely on weight and color to read as a
           heading, and in dark mode it has neither to spare: headings
           and bold share #e8ecf1 below, so an h4 line and a bold line
           differed by one weight step (UA bold 700 vs the 800 on
           strong) and nothing else. Sizes only -- the weights stay as
           they are, so bold body text remains the heavier of the two.
           h3 comes up from its own 1.17em to keep the steps apart:
           everything below it has to fit between it and body size, and
           0.17em will not hold three of them. */
        h3 {
            font-size: 1.30em;
        }
        h4 {
            font-size: 1.15em;
        }
        h5 {
            font-size: 1.08em;
        }
        h6 {
            font-size: 1.02em;
        }
        :root {
            --accent: {{.Accent}};
            --code-accent: {{.CodeAccent}};
        }
        /* --code-accent covers all three markdown spellings of code:
           inline, fenced and indented. goldmark renders both block
           forms as pre > code, so the color rides on the code rule and
           pre carries it for any stray text outside the inner element.
           (No backticks in this comment: the whole template is a Go raw
           string literal and one would end it.) */
        code {
            color: var(--code-accent);
            font-family: ui-monospace, 'SF Mono', 'Menlo', 'Ubuntu Mono', monospace;
        }
        pre {
            color: var(--code-accent);
            overflow-x: auto;
        }
        /* Bold is the one kind of emphasis with no accent of its own, so
           it earns its weight twice over: a real 800 (the system font's
           variable weight axis has the range, so this is SF Heavy, not a
           synthetic bold) plus a color that pulls away from the body
           text. The dark branch below lowers the body rather than
           raising the bold -- #ddd sat close enough to white that there
           was nowhere left to go. */
        strong, b {
            font-weight: 800;
            color: #000;
        }
        /* Emphasis stands down wherever a block owns its own color.
           Inside a heading it would both fight the heading color and say
           nothing (a heavier weight inside an already-bold heading);
           inside a quote it would punch out of the quote it belongs to. */
        h1 strong, h2 strong, h3 strong, h4 strong, h5 strong, h6 strong,
        h1 b, h2 b, h3 b, h4 b, h5 b, h6 b {
            color: inherit;
            font-weight: inherit;
        }
        blockquote strong, blockquote b {
            color: inherit;
        }
        blockquote {
            border-left: 4px solid var(--accent);
            margin: 0;
            padding-left: 20px;
            color: #7f8c8d;
        }
        /* Alert callouts. The parser strips the marker line and hangs
           callout + callout-KIND on the blockquote (callout.go); all of
           this is presentation. Each kind sets one hue on --callout and
           both the label and the tint read from it, so a kind is a
           three-line block and a recolor is one line.
           The label is generated by the stylesheet rather than injected
           into the HTML: it restates what the class already says, so it
           belongs to presentation -- and it stays out of copied text and
           out of the goldmark-pdf path that shares this markdown but not
           this sheet. A callout overrides the quote gray above: this is
           body text being pointed at, not text held at a distance. */
        blockquote.callout {
            /* No bar and no indent, unlike GitHub's alerts and unlike
               the blockquote above: the tinted ground and the colored
               label carry the block on their own, so neither the bar
               nor the 20px it stood in is doing work. Both are set
               explicitly rather than omitted, because the blockquote
               rule above supplies both.
               What is left is a band the width of the content column
               with its text on the body's own left edge -- an alert is
               body text being pointed at, and now it lines up like it.
               Indent stays the blockquote's alone, saying the one thing
               it means: this text is held at a distance.
               Which leaves the first glyph of every line starting
               exactly where the color does, and reading crushed
               against it. The fix is not to push the text inward --
               that would hand back the alignment this rule just won --
               but to push the tint outward: 4px of negative margin and
               4px of padding cancel, so the text sits on the body's
               left edge as if there were no padding at all, and the
               band starts 4px outside the content column. Clearance
               and alignment, rather than a trade between them.
               (1px was tried first. It is real in the box model and
               invisible on screen: one column of a 12% wash differs
               from the ground by too little to resolve.)
               The bleed is safe because both the standalone and the
               embedded body carry 20px of horizontal padding for it to
               grow into, at every width -- the padding does not shrink
               with the viewport, so this cannot reach the edge and
               raise a horizontal scrollbar. Symmetric, so a line long
               enough to reach the right edge clears it the same way. */
            border-left: none;
            background-color: var(--callout-bg);
            color: inherit;
            margin: 16px -4px;
            padding: 5px 4px;
            border-radius: 4px;
        }
        /* The label is a heading in everything but name, so it takes
           heading leading rather than the body's 1.6 -- at one word the
           extra half-line is pure gap above and below it. */
        blockquote.callout::before {
            content: var(--callout-label);
            display: block;
            color: var(--callout);
            font-weight: 700;
            line-height: 1.25;
            /* Negative on purpose. Roughly 7px of the visible gap is
               half-leading, not margin -- the label's line box is 1.25x
               its font size and the body's is 1.6x, and each centers its
               text, so there is empty box under the label and above the
               first body line before any margin is involved. Zero would
               still read loose; this pulls some of that back. */
            margin-bottom: -2px;
        }
        /* The label is a block box, so its bottom margin collapses
           against the first paragraph's 16px top margin and the larger
           wins -- without this the margin above is dead code and the gap
           is whatever the paragraph says. Zeroing it here makes the gap
           the label's to set, and the two rules the only place it lives. */
        blockquote.callout > :first-child {
            margin-top: 0;
        }
        /* The padding owns the bottom gap; the last child's margin would
           double it. */
        blockquote.callout > :last-child {
            margin-bottom: 0;
        }
        blockquote.callout-note {
            --callout: #0969da;
            --callout-bg: rgba(9, 105, 218, 0.08);
            --callout-label: "Note";
        }
        blockquote.callout-tip {
            --callout: #1a7f37;
            --callout-bg: rgba(26, 127, 55, 0.08);
            --callout-label: "Tip";
        }
        blockquote.callout-important {
            --callout: #8250df;
            --callout-bg: rgba(130, 80, 223, 0.08);
            --callout-label: "Important";
        }
        blockquote.callout-warning {
            --callout: #9a6700;
            --callout-bg: rgba(154, 103, 0, 0.10);
            --callout-label: "Warning";
        }
        blockquote.callout-caution {
            --callout: #cf222e;
            --callout-bg: rgba(207, 34, 46, 0.08);
            --callout-label: "Caution";
        }
        /* "[!]" -- ours, not GitHub's: a tint and nothing else, for
           pointing at a passage without naming a category. The label
           keyword suppresses the ::before entirely; an empty string
           would still generate the box and lay out a line of its
           leading. So --callout goes unread here and the tint is the
           whole design, which is why it runs a step stronger than the
           labeled kinds -- it is the only signal this block has. It is
           also neutral on purpose: a hue with no word to explain it
           reads as a warning someone forgot to name. */
        blockquote.callout-plain {
            --callout-bg: rgba(27, 31, 36, 0.06);
            --callout-label: none;
        }
        li::marker {
            color: var(--accent);
        }
        dt {
            color: var(--accent);
        }
        dd {
            margin: 0 0 8px 24px;
        }
        img {
            max-width: 100%;
            height: auto;
            border-radius: 5px;
        }
        .img-placeholder {
            display: inline-block;
            padding: 10px 14px;
            border: 1px dashed #bbb;
            border-radius: 5px;
            background-color: #f9f9f9;
            color: #7f8c8d;
            font-size: 0.9em;
        }
        /* Tables are scanned, not read: the eye tracks across a row and
           down a column, and the borders already mark where each cell
           begins. body's 1.6 is leading sized for 800px-wide prose
           lines and buys a table nothing -- it made every row tall and
           charged again for each line of a cell that wrapped, which is
           the line that sets the row's height. Setting line-height here
           is an addition rather than an override; nothing else in the
           sheet touches it. The step below body size is the other half
           of the ~36% a row lost. margin here is a ceiling, not a gap:
           adjacent margins collapse to the larger and p carries 16px,
           so 12px is invisible against a paragraph (and a table after a
           heading is already at 0, from the h1 + * rule above) -- what
           it guarantees is that a table never claims more room than the
           prose around it, whichever neighbor it lands next to. Only
           two adjacent tables ever see the number itself. */
        table {
            border-collapse: collapse;
            width: 100%;
            margin: 12px 0;
            line-height: 1.20;
            font-size: 0.88em;
        }
        th, td {
            border: 1px solid #ddd;
            padding: 7px 8px;
            text-align: left;
        }
        th {
            background-color: #f2f2f2;
            font-weight: bold;
        }
        @media (prefers-color-scheme: dark) {
            body {
                /* Dimmer than the #ddd this used to be. Back then bold
                   had no color rule at all, so it inherited this value
                   and separated by weight alone; dropping to #b8b8b8
                   opened 18.4 points of perceptual lightness for it to
                   use, and glared less besides. #c4c4c4 hands 4.4 of
                   those back -- #b8b8b8 read dimmer than wanted for
                   plain reading -- which still leaves bold 14.1 points
                   and every one of them more than it had before.
                   9.6:1 against the ground. */
                color: #c4c4c4;
                background-color: #1e1e1e;
            }
            /* Same tone as the headings below, deliberately: it keeps
               one bright value in the dark sheet instead of two, and
               stops bold from out-shining the headings it sits under
               (#fff is L* 100 against their 93.2). Costs ~7 points of
               separation from the body, leaving 14.1 -- exactly what
               the headings themselves live on. Dark only: in light mode
               bold is #000, already darker than the #2c3e50 headings,
               and matching them there would collapse its separation
               from 21.3 points to 5.1. */
            strong, b {
                color: #e8ecf1;
            }
            h1, h2, h3, h4, h5, h6 {
                color: #e8ecf1;
            }
            blockquote {
                color: #9aa4ab;
            }
            /* Hues only: the labels and the geometry above are
               theme-independent. Each is the light hue lifted toward the
               #c4c4c4 body so it still reads as its color at 9.6:1
               against #1e1e1e, and the tints go up a little because a
               dark ground swallows an 8% wash. */
            blockquote.callout-note {
                --callout: #4493f8;
                --callout-bg: rgba(68, 147, 248, 0.12);
            }
            blockquote.callout-tip {
                --callout: #3fb950;
                --callout-bg: rgba(63, 185, 80, 0.12);
            }
            blockquote.callout-important {
                --callout: #ab7df8;
                --callout-bg: rgba(171, 125, 248, 0.12);
            }
            blockquote.callout-warning {
                --callout: #d29922;
                --callout-bg: rgba(210, 153, 34, 0.12);
            }
            blockquote.callout-caution {
                --callout: #f85149;
                --callout-bg: rgba(248, 81, 73, 0.12);
            }
            /* A white wash rather than the light sheet's dark one, and
               at 0.09 it lands near the #2d2d2d the table headers and
               image placeholders already use -- one neutral surface
               tone in the dark sheet, not two. */
            blockquote.callout-plain {
                --callout-bg: rgba(255, 255, 255, 0.09);
            }
            .img-placeholder {
                border-color: #555;
                background-color: #2d2d2d;
                color: #9aa4ab;
            }
            th, td {
                border-color: #444;
            }
            th {
                background-color: #2d2d2d;
            }
            a {
                color: #6cb6ff;
            }
        }
    </style>
</head>
<body>
    {{if .Standalone}}<h1>{{.Title}}</h1>
    {{end}}<div id="content">
        {{.Content}}
    </div>
</body>
</html>`

	tmpl, err := template.New("note").Parse(htmlTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML template: %v", err)
	}

	var buf strings.Builder
	err = tmpl.Execute(&buf, struct {
		Title      string
		Content    template.HTML
		Standalone bool
		Accent     template.CSS
		CodeAccent template.CSS
	}{
		Title:      title,
		Content:    template.HTML(htmlContent),
		Standalone: standalone,
		// template.CSS is safe here: both resolvers only return
		// regex-validated hex or fixed keywords.
		Accent:     template.CSS(previewAccent()),
		CodeAccent: template.CSS(previewCodeAccent()),
	})

	if err != nil {
		return "", fmt.Errorf("failed to execute HTML template: %v", err)
	}

	return buf.String(), nil
}

// detectImageFormat detects image format from base64 data magic bytes
func detectImageFormat(base64Data string) string {
	// PNG signature: iVBORw0KG (base64 of 89 50 4E 47)
	if strings.HasPrefix(base64Data, "iVBORw0KG") {
		return "png"
	}
	// JPEG signature: /9j/ (base64 of FF D8 FF)
	if strings.HasPrefix(base64Data, "/9j/") {
		return "jpeg"
	}
	// GIF signature: R0lGOD (base64 of 47 49 46 38)
	if strings.HasPrefix(base64Data, "R0lGOD") {
		return "gif"
	}
	// Default to png if can't detect (most common format)
	return "png"
}

// imagePlaceholderHTML renders a visible stand-in for a Google Drive image
// that could not be inlined (not signed in, offline, undecodable) instead of
// leaving a dead gdrive: URL that browsers show as a broken-image icon. The
// full error is tucked into the title attribute (hover to read it). Raw HTML
// survives the markdown conversion because goldmark runs WithUnsafe.
func imagePlaceholderHTML(altText string, err error) string {
	label := altText
	if label == "" {
		label = "image"
	}
	reason := "could not be loaded"
	if errors.Is(err, ErrGoogleDriveNotConfigured) {
		reason = "sign in to Google Drive to view it (run <code>vimango --gdrive-auth</code> in a terminal)"
	}
	return fmt.Sprintf(`<span class="img-placeholder" title="%s">&#128444;&#65039; %s &mdash; %s</span>`,
		template.HTMLEscapeString(err.Error()), template.HTMLEscapeString(label), reason)
}

// preprocessMarkdownImages processes Google Drive images in markdown before HTML conversion
func preprocessMarkdownImages(markdown string) (string, error) {
	// Initialize cache if needed
	initImageCache()

	// Regular expression to find Google Drive URLs in markdown image syntax
	// Matches both full URLs and gdrive: format
	// ![alt text](https://drive.google.com/...) or ![alt text](gdrive:ID)
	googleDriveRegex := regexp.MustCompile(`!\[([^\]]*)\]\(((?:https://drive\.google\.com/file/d/[^)]+)|(?:gdrive:[a-zA-Z0-9_-]+))\)`)

	// Find all Google Drive image references
	matches := googleDriveRegex.FindAllStringSubmatch(markdown, -1)

	processedMarkdown := markdown

	for _, match := range matches {
		fullMatch := match[0] // Full match: ![alt](url)
		altText := match[1]   // Alt text
		googleURL := match[2] // Google Drive URL

		var dataURI string
		var err error

		// Try cache first
		if globalImageCache != nil {
			if cachedData, found := globalImageCache.GetCachedImage(googleURL); found {
				// Check if cached data is already a data URI or raw base64
				if strings.HasPrefix(cachedData, "data:") {
					// Already a proper data URI, use as-is
					dataURI = cachedData
				} else {
					// Raw base64 from cache - convert to data URI
					format := detectImageFormat(cachedData)
					dataURI = fmt.Sprintf("data:image/%s;base64,%s", format, cachedData)
				}
			} else {
				// Cache miss - download and convert to data URI
				dataURI, err = convertGoogleDriveImageToDataURI(googleURL)
				if err != nil {
					log.Printf("Warning: Could not convert Google Drive image %s: %v", googleURL, err)
					processedMarkdown = strings.Replace(processedMarkdown, fullMatch, imagePlaceholderHTML(altText, err), 1)
					continue
				}

				// Store in cache for future use
				if cacheErr := globalImageCache.StoreCachedImage(googleURL, dataURI); cacheErr != nil {
					log.Printf("Warning: Could not cache image %s: %v", googleURL, cacheErr)
					// Continue anyway - we have the data URI
				}
			}
		} else {
			// No cache available - fallback to direct conversion
			dataURI, err = convertGoogleDriveImageToDataURI(googleURL)
			if err != nil {
				log.Printf("Warning: Could not convert Google Drive image %s: %v", googleURL, err)
				processedMarkdown = strings.Replace(processedMarkdown, fullMatch, imagePlaceholderHTML(altText, err), 1)
				continue
			}
		}

		// Replace the Google Drive URL with the data URI
		newImageTag := fmt.Sprintf("![%s](%s)", altText, dataURI)
		processedMarkdown = strings.Replace(processedMarkdown, fullMatch, newImageTag, 1)
	}

	return processedMarkdown, nil
}

// convertGoogleDriveImageToDataURI downloads a Google Drive image and converts it to a data URI
func convertGoogleDriveImageToDataURI(googleURL string) (string, error) {
	// Download the image using the existing loadGoogleImage function
	// Note: We'll use reasonable defaults for max width/height for web display
	img, imgFmt, err := loadGoogleImage(googleURL, 1200, 800)
	if err != nil {
		return "", fmt.Errorf("failed to load Google Drive image: %w", err)
	}

	// Convert image to bytes
	var buf bytes.Buffer
	switch imgFmt {
	case "png":
		err = png.Encode(&buf, img)
	case "jpeg", "jpg":
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	case "heic":
		// HEIC decodes to a standard image.Image but browsers can't
		// display image/heic; re-encode as JPEG for the data URI.
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
		imgFmt = "jpeg"
	default:
		return "", fmt.Errorf("unsupported image format: %s", imgFmt)
	}

	if err != nil {
		return "", fmt.Errorf("failed to encode image: %v", err)
	}

	// Convert to base64
	base64Data := base64.StdEncoding.EncodeToString(buf.Bytes())

	// Create data URI
	dataURI := fmt.Sprintf("data:image/%s;base64,%s", imgFmt, base64Data)

	return dataURI, nil
}

// convertMarkdownToHTML converts markdown to HTML using goldmark
func convertMarkdownToHTML(markdown string) string {
	// Configure goldmark with common extensions
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,            // GitHub Flavored Markdown
			extension.Table,          // Tables
			extension.Strikethrough,  // Strikethrough text
			extension.Linkify,        // Auto-link URLs
			extension.TaskList,       // Task lists
			extension.DefinitionList, // Definition lists (PHP Markdown Extra syntax)
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(), // Auto-generate heading IDs
			// GitHub-style alert callouts (callout.go). A paragraph
			// transformer, so it sees the marker as source rather than
			// as already-parsed inlines.
			parser.WithParagraphTransformers(
				util.Prioritized(calloutTransformer{}, 100),
			),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(), // Allow raw HTML (needed for some markdown features)
		),
	)

	var buf bytes.Buffer
	if err := md.Convert([]byte(markdown), &buf); err != nil {
		// Fallback to plain text if goldmark fails
		return fmt.Sprintf("<pre>%s</pre>", markdown)
	}

	return buf.String()
}

// ShowWebviewNotAvailableMessage displays a user-friendly message
func ShowWebviewNotAvailableMessage() string {
	return fmt.Sprintf("%sWebview not available - opening in default browser%s", YELLOW_BG, RESET)
}
