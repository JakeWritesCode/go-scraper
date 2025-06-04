package main

import (
	"github.com/stretchr/testify/assert"
	"net/url"
	"testing"
)

func TestResolveAndCleanURL(t *testing.T) {
	base, _ := url.Parse("https://example.com/base/path/")

	tests := []struct {
		name     string
		href     string
		expected string
	}{
		{
			name:     "absolute URL preserved and cleaned",
			href:     "https://example.com/foo//bar#section",
			expected: "https://example.com/foo/bar",
		},
		{
			name:     "relative URL resolved and cleaned",
			href:     "../foo//bar?x=1#section",
			expected: "https://example.com/base/foo/bar?x=1",
		},
		{
			name:     "relative path with ./ and ..",
			href:     "./../baz/./qux",
			expected: "https://example.com/base/baz/qux",
		},
		{
			name:     "root-relative URL",
			href:     "/foo//bar/../baz#frag",
			expected: "https://example.com/foo/baz",
		},
		{
			name:     "just fragment",
			href:     "#frag",
			expected: "https://example.com/base/path",
		},
		{
			name:     "query without path",
			href:     "?a=1&b=2",
			expected: "https://example.com/base/path?a=1&b=2",
		},
		{
			name:     "path with double slashes",
			href:     "foo////bar",
			expected: "https://example.com/base/path/foo/bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ResolveAndCleanURL(base, tt.href)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result.String())
		})
	}
}
