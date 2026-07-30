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
            background-color: #f4f4f4;
            padding: 2px 4px;
            border-radius: 3px;
            font-family: ui-monospace, 'SF Mono', 'Menlo', 'Ubuntu Mono', monospace;
        }
        pre {
            color: var(--code-accent);
            background-color: #f4f4f4;
            padding: 15px;
            border-radius: 5px;
            overflow-x: auto;
        }
        blockquote {
            border-left: 4px solid var(--accent);
            margin: 0;
            padding-left: 20px;
            color: #7f8c8d;
        }
        li::marker {
            color: var(--accent);
        }
        dt {
            font-weight: bold;
            color: #2c3e50;
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
        table {
            border-collapse: collapse;
            width: 100%;
            margin: 20px 0;
        }
        th, td {
            border: 1px solid #ddd;
            padding: 12px;
            text-align: left;
        }
        th {
            background-color: #f2f2f2;
            font-weight: bold;
        }
        @media (prefers-color-scheme: dark) {
            body {
                color: #ddd;
                background-color: #1e1e1e;
            }
            h1, h2, h3, h4, h5, h6 {
                color: #e8ecf1;
            }
            code, pre {
                background-color: #2d2d2d;
            }
            blockquote {
                color: #9aa4ab;
            }
            dt {
                color: #e8ecf1;
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
