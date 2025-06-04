package main

import (
	"net/url"
	"path"
	"strings"
)

// ResolveAndCleanURL resolves href against base and sanitizes it:
// - Makes it absolute
// - Strips fragments (#...)
// - Normalizes path (removes duplicate slashes)
func ResolveAndCleanURL(base *url.URL, href string) (*url.URL, error) {
	parsedHref, err := url.Parse(href)
	if err != nil {
		return nil, err
	}

	// Resolve relative to base if needed
	var resolved *url.URL
	if parsedHref.IsAbs() {
		resolved = parsedHref
	} else {
		resolved = base.ResolveReference(parsedHref)
	}

	// Strip fragment
	resolved.Fragment = ""

	// Normalize path: remove duplicate slashes
	resolved.Path = cleanPath(resolved.Path)

	return resolved, nil
}

// cleanPath collapses repeated slashes and uses path.Clean for dot segments
func cleanPath(p string) string {
	collapsed := strings.ReplaceAll(p, "//", "/")
	return path.Clean(collapsed)
}
