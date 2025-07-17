package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"image/png"
	"image/jpeg"
	"regexp"
	"strings"
	
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// Default implementation that will be overridden by build-specific files
var isWebviewAvailableDefault = false

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

// RenderNoteAsHTML converts a note's markdown content to HTML for webview display
func RenderNoteAsHTML(title, markdownContent string) (string, error) {
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
            margin: 0 auto;
            padding: 20px;
            background-color: #fff;
        }
        h1, h2, h3, h4, h5, h6 {
            color: #2c3e50;
        }
        code {
            background-color: #f4f4f4;
            padding: 2px 4px;
            border-radius: 3px;
            font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
        }
        pre {
            background-color: #f4f4f4;
            padding: 15px;
            border-radius: 5px;
            overflow-x: auto;
        }
        blockquote {
            border-left: 4px solid #3498db;
            margin: 0;
            padding-left: 20px;
            color: #7f8c8d;
        }
        img {
            max-width: 100%;
            height: auto;
            border-radius: 5px;
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
    </style>
</head>
<body>
    <h1>{{.Title}}</h1>
    <div id="content">
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
		Title   string
		Content template.HTML
	}{
		Title:   title,
		Content: template.HTML(htmlContent),
	})
	
	if err != nil {
		return "", fmt.Errorf("failed to execute HTML template: %v", err)
	}
	
	return buf.String(), nil
}

// preprocessMarkdownImages processes Google Drive images in markdown before HTML conversion
func preprocessMarkdownImages(markdown string) (string, error) {
	// Regular expression to find Google Drive URLs in markdown image syntax
	// Matches ![alt text](google drive url) or ![alt text](google drive url "title")
	googleDriveRegex := regexp.MustCompile(`!\[([^\]]*)\]\((https://drive\.google\.com/file/d/[^)]+)\)`)
	
	// Find all Google Drive image references
	matches := googleDriveRegex.FindAllStringSubmatch(markdown, -1)
	
	processedMarkdown := markdown
	
	for _, match := range matches {
		fullMatch := match[0]  // Full match: ![alt](url)
		altText := match[1]    // Alt text
		googleURL := match[2]  // Google Drive URL
		
		// Download and convert to data URI
		dataURI, err := convertGoogleDriveImageToDataURI(googleURL)
		if err != nil {
			// If we can't convert, leave the original URL
			fmt.Printf("Warning: Could not convert Google Drive image %s: %v\n", googleURL, err)
			continue
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
		return "", fmt.Errorf("failed to load Google Drive image: %v", err)
	}
	
	// Convert image to bytes
	var buf bytes.Buffer
	switch imgFmt {
	case "png":
		err = png.Encode(&buf, img)
	case "jpeg", "jpg":
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	default:
		// Default to PNG for unknown formats
		err = png.Encode(&buf, img)
		imgFmt = "png"
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
			extension.GFM,        // GitHub Flavored Markdown
			extension.Table,      // Tables
			extension.Strikethrough, // Strikethrough text
			extension.Linkify,    // Auto-link URLs
			extension.TaskList,   // Task lists
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