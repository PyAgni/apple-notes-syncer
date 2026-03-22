package converter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTMLToMDConverter_Convert_BasicHTML(t *testing.T) {
	c := NewHTMLToMDConverter(false)

	md, err := c.Convert("<p>Hello world</p>")
	require.NoError(t, err)
	assert.Contains(t, md, "Hello world")
}

func TestHTMLToMDConverter_Convert_EmptyInput(t *testing.T) {
	c := NewHTMLToMDConverter(false)

	md, err := c.Convert("")
	require.NoError(t, err)
	assert.Empty(t, md)
}

func TestHTMLToMDConverter_Convert_AppleNotesHTML(t *testing.T) {
	// Typical Apple Notes HTML structure.
	html := `<div><h1>My Note Title</h1></div><div><br></div><div>This is the body of my note.</div><div><br></div><div>Second paragraph.</div>`

	c := NewHTMLToMDConverter(false)
	md, err := c.Convert(html)
	require.NoError(t, err)

	assert.Contains(t, md, "My Note Title")
	assert.Contains(t, md, "This is the body of my note.")
	assert.Contains(t, md, "Second paragraph.")
}

func TestHTMLToMDConverter_Convert_StripTitleH1(t *testing.T) {
	html := `<div><h1>My Note Title</h1></div><div>Body content here.</div>`

	c := NewHTMLToMDConverter(true)
	md, err := c.Convert(html)
	require.NoError(t, err)

	assert.NotContains(t, md, "# My Note Title")
	assert.Contains(t, md, "Body content here.")
}

func TestHTMLToMDConverter_Convert_PreservesLinks(t *testing.T) {
	html := `<p>Visit <a href="https://example.com">Example</a> for more info.</p>`

	c := NewHTMLToMDConverter(false)
	md, err := c.Convert(html)
	require.NoError(t, err)

	assert.Contains(t, md, "[Example](https://example.com)")
}

func TestHTMLToMDConverter_Convert_PreservesBold(t *testing.T) {
	html := `<p>This is <b>bold</b> text.</p>`

	c := NewHTMLToMDConverter(false)
	md, err := c.Convert(html)
	require.NoError(t, err)

	assert.Contains(t, md, "**bold**")
}

func TestHTMLToMDConverter_Convert_PreservesItalic(t *testing.T) {
	html := `<p>This is <i>italic</i> text.</p>`

	c := NewHTMLToMDConverter(false)
	md, err := c.Convert(html)
	require.NoError(t, err)

	assert.Contains(t, md, "*italic*")
}

func TestHTMLToMDConverter_Convert_UnorderedList(t *testing.T) {
	html := `<ul><li>Item 1</li><li>Item 2</li><li>Item 3</li></ul>`

	c := NewHTMLToMDConverter(false)
	md, err := c.Convert(html)
	require.NoError(t, err)

	assert.Contains(t, md, "- Item 1")
	assert.Contains(t, md, "- Item 2")
	assert.Contains(t, md, "- Item 3")
}

func TestHTMLToMDConverter_Convert_OrderedList(t *testing.T) {
	html := `<ol><li>First</li><li>Second</li></ol>`

	c := NewHTMLToMDConverter(false)
	md, err := c.Convert(html)
	require.NoError(t, err)

	assert.Contains(t, md, "1. First")
	assert.Contains(t, md, "2. Second")
}

func TestHTMLToMDConverter_Convert_MultipleH1OnlyStripsFirst(t *testing.T) {
	html := `<h1>Title</h1><p>Body</p><h1>Section</h1><p>More</p>`

	c := NewHTMLToMDConverter(true)
	md, err := c.Convert(html)
	require.NoError(t, err)

	// The first H1 should be stripped, but the second should remain.
	assert.Contains(t, md, "# Section")
	assert.Contains(t, md, "Body")
	assert.Contains(t, md, "More")
}

func TestHTMLToMDConverter_Convert_EndsWithNewline(t *testing.T) {
	c := NewHTMLToMDConverter(false)
	md, err := c.Convert("<p>Hello</p>")
	require.NoError(t, err)
	assert.True(t, len(md) > 0)
	assert.Equal(t, "\n", string(md[len(md)-1]))
}

func TestCleanupMarkdown_CollapseNewlines(t *testing.T) {
	input := "Line 1\n\n\n\n\nLine 2"
	result := cleanupMarkdown(input)
	assert.Equal(t, "Line 1\n\nLine 2\n", result)
}

func TestCleanupMarkdown_TrimWhitespace(t *testing.T) {
	input := "  \n\nHello\n\n  "
	result := cleanupMarkdown(input)
	assert.Equal(t, "Hello\n", result)
}

func TestCleanupMarkdown_EmptyInput(t *testing.T) {
	assert.Equal(t, "", cleanupMarkdown(""))
	assert.Equal(t, "", cleanupMarkdown("   "))
}

func TestReplaceFirst(t *testing.T) {
	re := h1Regex
	assert.Equal(t, "<p>body</p><h1>second</h1>", replaceFirst(re, "<h1>title</h1><p>body</p><h1>second</h1>", ""))
	assert.Equal(t, "no match", replaceFirst(re, "no match", ""))
}
