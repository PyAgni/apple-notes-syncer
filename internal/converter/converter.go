// Package converter provides HTML to Markdown conversion for Apple Notes content.
package converter

import (
	"fmt"
	"regexp"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
)

// MarkdownConverter converts HTML content to Markdown.
type MarkdownConverter interface {
	// Convert transforms an HTML string into Markdown.
	Convert(html string) (string, error)
}

// HTMLToMDConverter wraps the html-to-markdown library with Apple Notes
// specific preprocessing.
type HTMLToMDConverter struct {
	// stripTitleH1 removes the first <h1> tag if it duplicates the note title.
	stripTitleH1 bool
}

// NewHTMLToMDConverter creates a new converter. If stripTitleH1 is true,
// the first <h1> element is removed from the body since Apple Notes
// duplicates the note title as an <h1> in the body HTML.
func NewHTMLToMDConverter(stripTitleH1 bool) *HTMLToMDConverter {
	return &HTMLToMDConverter{
		stripTitleH1: stripTitleH1,
	}
}

// h1Regex matches the first <h1>...</h1> tag in the HTML.
var h1Regex = regexp.MustCompile(`(?i)<h1[^>]*>.*?</h1>`)

// multiNewlineRegex matches 3 or more consecutive newlines.
var multiNewlineRegex = regexp.MustCompile(`\n{3,}`)

// Convert transforms Apple Notes HTML into clean Markdown.
func (c *HTMLToMDConverter) Convert(html string) (string, error) {
	if html == "" {
		return "", nil
	}

	// Preprocess: strip the title <h1> if configured.
	processed := html
	if c.stripTitleH1 {
		processed = replaceFirst(h1Regex, processed, "")
	}

	// Convert to Markdown using the library.
	md, err := htmltomarkdown.ConvertString(processed)
	if err != nil {
		return "", fmt.Errorf("converting HTML to markdown: %w", err)
	}

	// Clean up excessive blank lines.
	md = cleanupMarkdown(md)

	return md, nil
}

// replaceFirst replaces only the first occurrence of the regex match.
func replaceFirst(re *regexp.Regexp, s string, repl string) string {
	loc := re.FindStringIndex(s)
	if loc == nil {
		return s
	}
	return s[:loc[0]] + repl + s[loc[1]:]
}

// cleanupMarkdown normalizes whitespace in the converted Markdown.
func cleanupMarkdown(md string) string {
	// Replace 3+ consecutive newlines with 2.
	md = multiNewlineRegex.ReplaceAllString(md, "\n\n")

	// Trim leading/trailing whitespace.
	md = strings.TrimSpace(md)

	// Ensure file ends with a newline.
	if md != "" && !strings.HasSuffix(md, "\n") {
		md += "\n"
	}

	return md
}
