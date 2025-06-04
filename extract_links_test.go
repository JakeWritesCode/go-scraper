package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExtractLinks_SimpleAnchors(t *testing.T) {
	html := `<html><body><a href="https://example.com">Example</a></body></html>`

	links, err := ExtractLinks(html)

	assert.NoError(t, err)
	assert.Equal(t, []string{"https://example.com"}, links)
}

func TestExtractLinks_MultipleAnchors(t *testing.T) {
	html := `
	<html>
		<body>
			<a href="https://foo.com">Foo</a>
			<a href="https://bar.com">Bar</a>
		</body>
	</html>
	`

	links, err := ExtractLinks(html)

	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"https://foo.com", "https://bar.com"}, links)
}

func TestExtractLinks_NoAnchors(t *testing.T) {
	html := `<html><body><p>No links here!</p></body></html>`

	links, err := ExtractLinks(html)

	assert.NoError(t, err)
	assert.Empty(t, links)
}

func TestExtractLinks_InvalidHTML(t *testing.T) {
	html := `<html><body><a href="incomplete`

	links, err := ExtractLinks(html)

	assert.NoError(t, err)
	assert.Empty(t, links)
}

func TestExtractLinks_AnchorWithoutHref(t *testing.T) {
	html := `<html><body><a>No href here</a></body></html>`

	links, err := ExtractLinks(html)

	assert.NoError(t, err)
	assert.Empty(t, links)
}

func TestExtractLinks_ComplexHTML(t *testing.T) {
	html := `
	<html>
		<head>
			<title>Test Page</title>
			<link rel="stylesheet" href="https://example.com/style.css">
		</head>
		<body>
			<a href="https://example.com/page1">Page 1</a>
			<a href="/page2">Page 2</a>
			<a href="https://example.com/page3#section">Page 3 Section</a>
			<a href="javascript:void(0)">No link</a>
		</body>
	</html>
	`

	expectedLinks := []string{
		"https://example.com/page1",
		"/page2",
		"https://example.com/page3#section",
		"javascript:void(0)",
	}

	links, err := ExtractLinks(html)

	assert.NoError(t, err)
	assert.ElementsMatch(t, expectedLinks, links)
}
