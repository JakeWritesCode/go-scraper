package main

import (
	"encoding/xml"
	"github.com/samber/lo"
)

type UrlSet struct {
	URLs []UrlEntry `xml:"url"`
}

type UrlEntry struct {
	Loc string `xml:"loc"`
}

// ParseSitemapForUrls takes a sitemap string and extracts all URLs from it.
func ParseSitemapForUrls(sitemap string) ([]string, error) {
	var urlSet UrlSet
	err := xml.Unmarshal([]byte(sitemap), &urlSet)
	if err != nil {
		return nil, err
	}

	return lo.Reduce(urlSet.URLs, func(acc []string, entry UrlEntry, _ int) []string {
		if entry.Loc != "" {
			return append(acc, entry.Loc)
		}
		return acc
	}, []string{}), nil
}
