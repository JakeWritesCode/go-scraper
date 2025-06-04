package main

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestParseSitemapForUrls_ReturnsErrorOnInvalidXML(t *testing.T) {
	t.Parallel()
	sitemap := `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>http://example.com</loc></url></urlset`
	_, err := ParseSitemapForUrls(sitemap)
	assert.Error(t, err)
}

func TestParseSitemapForUrls_ReturnsUrls(t *testing.T) {
	t.Parallel()
	sitemap := `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url>
		<loc>http://example.com</loc>
	</url>
	<url>
		<loc>http://example.com/about</loc>
	</url>
	<url>
		<loc>http://example.com/contact</loc>
	</url>
	</urlset>`
	urls, err := ParseSitemapForUrls(sitemap)
	require.NoError(t, err)
	assert.Equal(t, 3, len(urls))
	assert.Contains(t, urls, "http://example.com")
	assert.Contains(t, urls, "http://example.com/about")
	assert.Contains(t, urls, "http://example.com/contact")
}

func TestParseSitemapForUrls_EmptySitemap(t *testing.T) {
	t.Parallel()
	sitemap := `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"></urlset>`
	urls, err := ParseSitemapForUrls(sitemap)
	require.NoError(t, err)
	assert.Equal(t, 0, len(urls))
}

func TestParseSitemapForUrls_NoLocTags(t *testing.T) {
	t.Parallel()
	sitemap := `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
	<url>
		<lastmod>2023-10-01</lastmod>
	</url>
	<url>
		<changefreq>daily</changefreq>
	</url>
	</urlset>`
	urls, err := ParseSitemapForUrls(sitemap)
	require.NoError(t, err)
	assert.Equal(t, 0, len(urls))
}
