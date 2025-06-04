package main

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type DoNothingPostProcessor struct{}

func (p *DoNothingPostProcessor) Process(ctx context.Context, pageURL *url.URL, content string) error {
	return nil
}

func TestNewSiteCrawler_SetsBaseValues(t *testing.T) {

	baseUrl, err := url.Parse("https://example.com")
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := &StdoutLogger{}
	crawler, err := NewSiteCrawler(
		ctx,
		*baseUrl,
		logger,
		1000,
		"Crawler",
		20,
		[]PostProcessor{&DoNothingPostProcessor{}},
	)
	require.NoError(t, err)
	require.NotNil(t, crawler)
	assert.Equal(t, *baseUrl, crawler.BaseURL)
	assert.Equal(t, logger, crawler.Logger)
	assert.Equal(t, "Crawler", crawler.UserAgent)
	assert.Equal(t, time.Duration(1000), crawler.TimeoutMilliseconds)
	assert.Equal(t, 20, crawler.WorkerPoolSize)
	assert.NotNil(t, crawler.RobotsChecker)
	assert.NotNil(t, crawler.crawlWg)
	assert.NotNil(t, crawler.CrawlQueue)
	assert.NotNil(t, crawler.postProcessWg)
	assert.NotNil(t, crawler.PostProcessQueue)
	assert.Equal(t, []PostProcessor{&DoNothingPostProcessor{}}, crawler.postProcessors)
}

func TestNewSiteCrawler_ParsesAndReadsRobotsTxt(t *testing.T) {

	testPages := []PageReturn{
		{
			URL:               "/robots.txt",
			HTML:              "User-agent: *\nDisallow: /",
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
	}
	server := startTestServerPages(testPages)
	defer server.Close()

	baseUrl, err := url.Parse(server.URL)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := &StdoutLogger{}
	crawler, err := NewSiteCrawler(
		ctx,
		*baseUrl,
		logger,
		1000,
		"Crawler",
		20,
		[]PostProcessor{&DoNothingPostProcessor{}},
	)
	require.NoError(t, err)
	require.NotNil(t, crawler)

	assert.False(t, crawler.RobotsChecker.IsAllowed("/some/path", "Google"))
}

func TestNewSiteCrawler_HandlesNoRobotsTxt(t *testing.T) {

	testPages := []PageReturn{
		{
			URL:               "/robots.txt",
			HTML:              "",
			StatusCode:        404,
			DelayMilliseconds: 0,
		},
	}
	server := startTestServerPages(testPages)
	defer server.Close()

	baseUrl, err := url.Parse(server.URL)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	logger := &StdoutLogger{}
	crawler, err := NewSiteCrawler(
		ctx,
		*baseUrl,
		logger,
		1000,
		"Crawler",
		20,
		[]PostProcessor{&DoNothingPostProcessor{}},
	)
	require.NoError(t, err)
	require.NotNil(t, crawler)

	assert.True(t, crawler.RobotsChecker.IsAllowed("/some/path", "Google"))
}

type SpyProcessor struct {
	PageData  sync.Map
	CallCount atomic.Int32
}

func (s *SpyProcessor) Process(ctx context.Context, pageURL *url.URL, pageContent string) error {
	log.Printf("SpyProcessor processing page: %s", pageURL.String())
	s.CallCount.Add(1)
	s.PageData.Store(pageURL.String(), pageContent)
	return nil
}

func TestSiteCrawler_CrawlPage_SendsSuccessfulGETForPostProcessing(t *testing.T) {

	testPages := []PageReturn{
		{
			URL:               "/beans",
			HTML:              "Hello, World!",
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
	}
	server := startTestServerPages(testPages)
	defer server.Close()

	baseUrl, err := url.Parse(server.URL)
	require.NoError(t, err)

	spy := &SpyProcessor{}
	logger := &StdoutLogger{}

	ctx, cancel := context.WithCancel(context.Background())
	crawler, err := NewSiteCrawler(
		ctx,
		*baseUrl,
		logger,
		1000,
		"Crawler",
		20,
		[]PostProcessor{spy},
	)
	require.NoError(t, err)

	go crawler.startCrawlWorkers(context.Background())
	go crawler.startPostProcessingWorkers(context.Background())
	beansUrl := baseUrl.ResolveReference(&url.URL{Path: "/beans"})
	crawler.CrawlPage(context.Background(), beansUrl)

	require.Eventually(t, func() bool {
		_, ok := spy.PageData.Load(beansUrl.String())
		return ok
	}, 2*time.Second, 10*time.Millisecond)
	loadedContent, ok := spy.PageData.Load(beansUrl.String())
	assert.True(t, ok, "expected page data to be stored in spy processor")
	assert.Equal(t, "Hello, World!", loadedContent, "expected page content to match")
	cancel()
}

func TestSiteCrawler_CrawlPage_ExitsGracefullyOnCtxClose(t *testing.T) {

	testPages := []PageReturn{
		{
			URL:               "/beans",
			HTML:              "Hello, World!",
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
		{
			URL:               "/toast",
			HTML:              "Hello, World!",
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
	}
	server := startTestServerPages(testPages)
	defer server.Close()

	baseUrl, err := url.Parse(server.URL)
	require.NoError(t, err)

	spy := &SpyProcessor{}
	ctx, cancel := context.WithCancel(context.Background())

	logger := &StdoutLogger{}
	crawler, err := NewSiteCrawler(
		ctx,
		*baseUrl,
		logger,
		1000,
		"Crawler",
		20,
		[]PostProcessor{spy},
	)
	require.NoError(t, err)

	go crawler.startCrawlWorkers(context.Background())
	go crawler.startPostProcessingWorkers(context.Background())

	beansUrl := baseUrl.ResolveReference(&url.URL{Path: "/beans"})
	crawler.CrawlPage(ctx, beansUrl)
	require.Eventually(t, func() bool {
		_, ok := spy.PageData.Load(beansUrl.String())
		return ok
	}, 2*time.Second, 10*time.Millisecond)

	cancel()
	toastUrl := baseUrl.ResolveReference(&url.URL{Path: "/toast"})
	crawler.CrawlPage(ctx, toastUrl)

	require.Never(t, func() bool {
		_, ok := spy.PageData.Load(toastUrl.String())
		return ok
	}, 200*time.Millisecond, 10*time.Millisecond, "unexpected task was enqueued")
}

func TestSiteCrawler_CrawlPage_SkipsProcessingFor404Urls(t *testing.T) {

	testPages := []PageReturn{
		{
			URL:               "/beans",
			HTML:              "Hello, World!",
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
		{
			URL:               "/toast",
			HTML:              "Page not found",
			StatusCode:        404,
			DelayMilliseconds: 0,
		},
	}
	server := startTestServerPages(testPages)
	defer server.Close()

	baseUrl, err := url.Parse(server.URL)
	require.NoError(t, err)

	spy := &SpyProcessor{}
	ctx, cancel := context.WithCancel(context.Background())

	logger := &StdoutLogger{}
	crawler, err := NewSiteCrawler(
		ctx,
		*baseUrl,
		logger,
		1000,
		"Crawler",
		20,
		[]PostProcessor{spy},
	)
	require.NoError(t, err)

	go crawler.startCrawlWorkers(ctx)
	go crawler.startPostProcessingWorkers(ctx)

	beansUrl := baseUrl.ResolveReference(&url.URL{Path: "/beans"})
	crawler.CrawlPage(ctx, beansUrl)
	require.Eventually(t, func() bool {
		_, ok := spy.PageData.Load(beansUrl.String())
		return ok
	}, 2*time.Second, 10*time.Millisecond)

	toastUrl := baseUrl.ResolveReference(&url.URL{Path: "/toast"})
	crawler.CrawlPage(ctx, toastUrl)

	require.Never(t, func() bool {
		_, ok := spy.PageData.Load(toastUrl.String())
		return ok
	}, 200*time.Millisecond, 10*time.Millisecond, "unexpected task was enqueued")
	cancel()
}

func TestSiteCrawler_CrawlPage_EnqueuesAdditionalFoundPagesForCrawling(t *testing.T) {

	testPages := []PageReturn{
		{
			URL:               "/beans",
			HTML:              "<body><a href=\"/toast\">Toast</a></body>",
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
		{
			URL:               "/toast",
			HTML:              "Page not found",
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
	}
	server := startTestServerPages(testPages)
	defer server.Close()

	baseUrl, err := url.Parse(server.URL)
	require.NoError(t, err)

	spy := &SpyProcessor{}
	ctx, cancel := context.WithCancel(context.Background())

	logger := &StdoutLogger{}
	crawler, err := NewSiteCrawler(
		ctx,
		*baseUrl,
		logger,
		1000,
		"Crawler",
		20,
		[]PostProcessor{spy},
	)
	require.NoError(t, err)

	go crawler.startCrawlWorkers(ctx)
	go crawler.startPostProcessingWorkers(ctx)

	beansUrl := baseUrl.ResolveReference(&url.URL{Path: "/beans"})
	crawler.CrawlPage(ctx, beansUrl)
	require.Eventually(t, func() bool {
		_, ok := spy.PageData.Load(beansUrl.String())
		return ok
	}, 2*time.Second, 10*time.Millisecond)

	toastUrl := baseUrl.ResolveReference(&url.URL{Path: "/toast"})
	require.Eventually(t, func() bool {
		_, ok := spy.PageData.Load(toastUrl.String())
		return ok
	}, 1000*time.Millisecond, 10*time.Millisecond, "toast url was not found in spy processor")
	cancel()
}

func TestSiteCrawler_AddURLToCrawlQueue_EnqueuesPageForCrawling(t *testing.T) {

	testPages := []PageReturn{
		{
			URL:               "/beans",
			HTML:              "Hello world!",
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
		{
			URL:               "/toast",
			HTML:              "Page not found",
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
	}
	server := startTestServerPages(testPages)
	defer server.Close()

	baseUrl, err := url.Parse(server.URL)
	require.NoError(t, err)

	spy := &SpyProcessor{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := &StdoutLogger{}
	crawler, err := NewSiteCrawler(
		ctx,
		*baseUrl,
		logger,
		1000,
		"Crawler",
		20,
		[]PostProcessor{spy},
	)
	require.NoError(t, err)

	go crawler.startCrawlWorkers(ctx)
	go crawler.startPostProcessingWorkers(ctx)

	beansUrl := baseUrl.ResolveReference(&url.URL{Path: "/beans"})
	crawler.AddURLToCrawlQueue(ctx, beansUrl)
	require.Eventually(t, func() bool {
		_, ok := spy.PageData.Load(beansUrl.String())
		return ok
	}, 2*time.Second, 10*time.Millisecond)
}

func TestSiteCrawler_AddURLToCrawlQueue_WillNotCrawlIfRobotsDisallow(t *testing.T) {

	testPages := []PageReturn{
		{
			URL:               "/beans",
			HTML:              "Super secret page, not to be called.",
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
		{
			URL:        "/robots.txt",
			HTML:       "User-agent: *\nDisallow: /beans",
			StatusCode: 200,
		},
		{
			URL:               "/toast",
			HTML:              "Hello, World!",
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
	}
	server := startTestServerPages(testPages)
	defer server.Close()

	baseUrl, err := url.Parse(server.URL)
	require.NoError(t, err)

	spy := &SpyProcessor{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := &StdoutLogger{}
	crawler, err := NewSiteCrawler(
		ctx,
		*baseUrl,
		logger,
		1000,
		"Crawler",
		20,
		[]PostProcessor{spy},
	)
	require.NoError(t, err)

	go crawler.startCrawlWorkers(ctx)
	go crawler.startPostProcessingWorkers(ctx)

	beansUrl := baseUrl.ResolveReference(&url.URL{Path: "/beans"})
	toastUrl := baseUrl.ResolveReference(&url.URL{Path: "/toast"})
	crawler.AddURLToCrawlQueue(ctx, beansUrl)
	crawler.AddURLToCrawlQueue(ctx, toastUrl)

	require.Eventually(t, func() bool {
		_, ok := spy.PageData.Load(toastUrl.String())
		return ok
	}, 2*time.Second, 10*time.Millisecond, "toast url was not found in spy processor")

	require.Never(t, func() bool {
		_, ok := spy.PageData.Load("/beans")
		return ok
	}, 200*time.Millisecond, 10*time.Millisecond, "unexpected task was enqueued")
}

func TestSiteCrawler_AddURLToCrawlQueue_WillNotCrawlSamePageTwice(t *testing.T) {

	testPages := []PageReturn{
		{
			URL:               "/beans",
			HTML:              "Super secret page, not to be called.",
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
	}
	server := startTestServerPages(testPages)
	defer server.Close()

	baseUrl, err := url.Parse(server.URL)
	require.NoError(t, err)

	spy := &SpyProcessor{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := &StdoutLogger{}
	crawler, err := NewSiteCrawler(
		ctx,
		*baseUrl,
		logger,
		1000,
		"Crawler",
		20,
		[]PostProcessor{spy},
	)
	require.NoError(t, err)

	go crawler.startCrawlWorkers(ctx)
	go crawler.startPostProcessingWorkers(ctx)

	beansUrl := baseUrl.ResolveReference(&url.URL{Path: "/beans"})
	crawler.AddURLToCrawlQueue(ctx, beansUrl)
	crawler.AddURLToCrawlQueue(ctx, beansUrl)
	crawler.AddURLToCrawlQueue(ctx, beansUrl)

	require.Eventually(t, func() bool {
		_, ok := spy.PageData.Load(beansUrl.String())
		return ok
	}, 2*time.Second, 10*time.Millisecond, "toast url was not found in spy processor")

	require.Equal(t, int32(1), spy.CallCount.Load(), "expected page to be processed only once")
}

func TestSiteCrawler_AddURLToCrawlQueue_DoesNotAddPageFromAnotherDomain(t *testing.T) {
	baseUrl, err := url.Parse("https://example.com")
	require.NoError(t, err)

	spy := &SpyProcessor{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := &StdoutLogger{}
	crawler, err := NewSiteCrawler(
		ctx,
		*baseUrl,
		logger,
		1000,
		"Crawler",
		20,
		[]PostProcessor{spy},
	)
	require.NoError(t, err)

	externalUrl, _ := url.Parse("https://external.com/beans")
	crawler.AddURLToCrawlQueue(ctx, externalUrl)

	require.Never(t, func() bool {
		return spy.CallCount.Load() > 0
	}, 200*time.Millisecond, 10*time.Millisecond, "unexpected task was enqueued for external URL")
}

func TestSiteCrawler_AddURLToPostProcessQueue_AddsOneTaskPerProcessor(t *testing.T) {

	testPages := []PageReturn{
		{
			URL:               "/beans",
			HTML:              "Hello, World!",
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
	}
	server := startTestServerPages(testPages)
	defer server.Close()

	baseUrl, err := url.Parse(server.URL)
	require.NoError(t, err)

	spy := &SpyProcessor{}
	spy2 := &SpyProcessor{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := &StdoutLogger{}
	crawler, err := NewSiteCrawler(
		ctx,
		*baseUrl,
		logger,
		1000,
		"Crawler",
		20,
		[]PostProcessor{spy, spy2},
	)
	require.NoError(t, err)

	go crawler.startCrawlWorkers(ctx)
	go crawler.startPostProcessingWorkers(ctx)

	pageUrl := baseUrl.ResolveReference(&url.URL{Path: "/beans"})
	crawler.AddURLToPostProcessQueue(ctx, pageUrl, "Hello, World!")

	require.Eventually(t, func() bool {
		return spy.CallCount.Load() == 1
	}, 2*time.Second, 10*time.Millisecond)

	content, ok := spy.PageData.Load(pageUrl.String())
	require.True(t, ok)
	assert.Equal(t, "Hello, World!", content)

	require.Eventually(t, func() bool {
		return spy2.CallCount.Load() == 1
	}, 2*time.Second, 10*time.Millisecond)
	content2, ok := spy2.PageData.Load(pageUrl.String())
	require.True(t, ok)
	assert.Equal(t, "Hello, World!", content2)
}

func TestSiteCrawler_CrawlFromSiteMap_CrawlsAllSitemapUrls(t *testing.T) {

	testPages := []PageReturn{
		{
			URL: "/sitemap.xml",
			HTML: fmt.Sprintf(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
				<url>
					<loc>/beans</loc>
				</url>
				<url>
					<loc>/toast</loc>
				</url>
				<url>
					<loc>/eggs</loc>
				</url>
			</urlset>`),
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
		{
			URL:        "/beans",
			HTML:       "Hello, World!",
			StatusCode: 200,
		},
		{
			URL:        "/toast",
			HTML:       "Hello, World!",
			StatusCode: 200,
		},
		{
			URL:        "/eggs",
			HTML:       "Eggs are great!",
			StatusCode: 200,
		},
	}
	server := startTestServerPages(testPages)
	defer server.Close()

	baseUrl, err := url.Parse(server.URL)
	require.NoError(t, err)

	spy := &SpyProcessor{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := &StdoutLogger{}
	crawler, err := NewSiteCrawler(
		ctx,
		*baseUrl,
		logger,
		1000,
		"Crawler",
		20,
		[]PostProcessor{spy},
	)
	require.NoError(t, err)

	err = crawler.Crawl(ctx)
	require.NoError(t, err)

	require.Equal(t, int32(3), spy.CallCount.Load(), "expected 3 pages to be processed")

	beansUrl := baseUrl.ResolveReference(&url.URL{Path: "/beans"})
	contentBeans, ok := spy.PageData.Load(beansUrl.String())
	require.True(t, ok)
	assert.Equal(t, "Hello, World!", contentBeans)

	toastUrl := baseUrl.ResolveReference(&url.URL{Path: "/toast"})
	contentToast, ok := spy.PageData.Load(toastUrl.String())
	require.True(t, ok)
	assert.Equal(t, "Hello, World!", contentToast)

	eggsUrl := baseUrl.ResolveReference(&url.URL{Path: "/eggs"})
	contentEggs, ok := spy.PageData.Load(eggsUrl.String())
	require.True(t, ok)
	assert.Equal(t, "Eggs are great!", contentEggs)
}

func TestSiteCrawler_CrawlFromSiteMap_HandlesEmptySitemap(t *testing.T) {
	testPages := []PageReturn{
		{
			URL:               "/sitemap.xml",
			HTML:              `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"></urlset>`,
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
	}
	server := startTestServerPages(testPages)
	defer server.Close()

	baseUrl, err := url.Parse(server.URL)
	require.NoError(t, err)

	spy := &SpyProcessor{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := &StdoutLogger{}
	crawler, err := NewSiteCrawler(
		ctx,
		*baseUrl,
		logger,
		1000,
		"Crawler",
		20,
		[]PostProcessor{spy},
	)
	require.NoError(t, err)

	err = crawler.Crawl(ctx)
	require.NoError(t, err)

	require.Equal(t, int32(0), spy.CallCount.Load(), "expected no pages to be processed")
}

func TestSiteCrawler_CrawlFromSiteMap_HandlesInvalidSitemap(t *testing.T) {
	testPages := []PageReturn{
		{
			URL:               "/sitemap.xml",
			HTML:              `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>invalid-url</loc></url></urlset>`,
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
	}
	server := startTestServerPages(testPages)
	defer server.Close()

	baseUrl, err := url.Parse(server.URL)
	require.NoError(t, err)

	spy := &SpyProcessor{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := &StdoutLogger{}
	crawler, err := NewSiteCrawler(
		ctx,
		*baseUrl,
		logger,
		1000,
		"Crawler",
		20,
		[]PostProcessor{spy},
	)
	require.NoError(t, err)

	err = crawler.Crawl(ctx)
	require.NoError(t, err, "expected error due to invalid URL in sitemap")
	assert.Equal(t, int32(0), spy.CallCount.Load(), "expected no pages to be processed")
}

func TestSiteCrawler_Crawl_ExampleSite(t *testing.T) {
	testPages := []PageReturn{
		{
			URL: "/sitemap.xml",
			HTML: `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
				<url>
					<loc>/beans</loc>
				</url>
				<url>
					<loc>/toast</loc>
				</url>
				<url>
					<loc>/eggs</loc>
				</url>
			</urlset>`,
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
		{
			URL:               "/robots.txt",
			HTML:              "User-agent: *\nDisallow: /eggs/bacon",
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
		{
			URL:               "/beans",
			HTML:              "<body><a href=\"/we-love-breakfast\">We love breakfast!</a></body>",
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
		{
			URL:               "/toast",
			HTML:              `Bacon!`,
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
		{
			URL:               "/eggs",
			HTML:              `<body><a href="/this-page-is-dead">Dead page</a></body>`,
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
		{
			URL:               "/eggs/bacon",
			HTML:              `DO NOT CRAWL ME!`,
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
		{
			URL:               "/we-love-breakfast",
			HTML:              `This page is not on the sitemap but is linked from beans.`,
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
		{
			URL:               "/this-page-is-dead",
			HTML:              `Not found`,
			StatusCode:        404,
			DelayMilliseconds: 0,
		},
	}
	server := startTestServerPages(testPages)
	defer server.Close()

	baseUrl, err := url.Parse(server.URL)
	require.NoError(t, err)

	spy := &SpyProcessor{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := &StdoutLogger{}
	crawler, err := NewSiteCrawler(
		ctx,
		*baseUrl,
		logger,
		1000,
		"Crawler",
		20,
		[]PostProcessor{spy},
	)
	require.NoError(t, err)

	err = crawler.Crawl(ctx)
	require.NoError(t, err, "expected no error during crawl")

	require.Equal(t, int32(4), spy.CallCount.Load(), "expected 5 pages to be processed")
	beansUrl := baseUrl.ResolveReference(&url.URL{Path: "/beans"})
	contentBeans, ok := spy.PageData.Load(beansUrl.String())
	require.True(t, ok, "expected beans page to be processed")
	assert.Equal(t, "<body><a href=\"/we-love-breakfast\">We love breakfast!</a></body>", contentBeans, "expected beans page content to match")

	toastUrl := baseUrl.ResolveReference(&url.URL{Path: "/toast"})
	contentToast, ok := spy.PageData.Load(toastUrl.String())
	require.True(t, ok, "expected toast page to be processed")
	assert.Equal(t, "Bacon!", contentToast, "expected toast page content to match")

	eggsUrl := baseUrl.ResolveReference(&url.URL{Path: "/eggs"})
	contentEggs, ok := spy.PageData.Load(eggsUrl.String())
	require.True(t, ok, "expected eggs page to be processed")
	assert.Equal(t, `<body><a href="/this-page-is-dead">Dead page</a></body>`, contentEggs, "expected eggs page content to match")

	weLoveBreakfastUrl := baseUrl.ResolveReference(&url.URL{Path: "/we-love-breakfast"})
	contentWeLoveBreakfast, ok := spy.PageData.Load(weLoveBreakfastUrl.String())
	require.True(t, ok, "expected we-love-breakfast page to be processed")
	assert.Equal(t, `This page is not on the sitemap but is linked from beans.`, contentWeLoveBreakfast, "expected we-love-breakfast page content to match")

	thisPageIsDeadUrl := baseUrl.ResolveReference(&url.URL{Path: "/this-page-is-dead"})
	_, ok = spy.PageData.Load(thisPageIsDeadUrl.String())
	require.False(t, ok, "expected this-page-is-dead page not to be processed, becasue it is a 404")

	eggsBaconUrl := baseUrl.ResolveReference(&url.URL{Path: "/eggs/bacon"})
	_, ok = spy.PageData.Load(eggsBaconUrl.String())
	require.False(t, ok, "expected eggs/bacon page not to be processed, because it is disallowed by robots.txt")
}

func TestSiteCrawler_Crawl_DeeplyNestedURLS(t *testing.T) {
	testPages := []PageReturn{
		{
			URL: "/sitemap.xml",
			HTML: `<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
				<url>
					<loc>/beans</loc>
				</url>
			</urlset>`,
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
		{
			URL:               "/beans",
			HTML:              "<body><a href=\"/toast\">Toast</a></body>",
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
		{
			URL:               "/toast",
			HTML:              `<body><a href="/eggs">Eggs</a></body>`,
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
		{
			URL:               "/eggs",
			HTML:              `<body><a href="/hash-brown">Hash Brown</a></body>`,
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
		{
			URL:               "/hash-brown",
			HTML:              `<body><a href="/black-pudding">Black Pudding</a></body>`,
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
		{
			URL:               "/black-pudding",
			HTML:              `<body><a href="/orange-juice">Orange Juice</a></body>`,
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
		{
			URL:               "/orange-juice",
			HTML:              `You found me, nice work!`,
			StatusCode:        200,
			DelayMilliseconds: 0,
		},
	}
	server := startTestServerPages(testPages)
	defer server.Close()

	baseUrl, err := url.Parse(server.URL)
	require.NoError(t, err)

	spy := &SpyProcessor{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := &StdoutLogger{}
	crawler, err := NewSiteCrawler(
		ctx,
		*baseUrl,
		logger,
		1000,
		"Crawler",
		20,
		[]PostProcessor{spy},
	)
	require.NoError(t, err)

	err = crawler.Crawl(ctx)
	require.NoError(t, err, "expected no error during crawl")

	require.Equal(t, int32(6), spy.CallCount.Load(), "expected 6 pages to be processed")
	orangeJuiceUrl := baseUrl.ResolveReference(&url.URL{Path: "/orange-juice"})
	contentOrangeJuice, ok := spy.PageData.Load(orangeJuiceUrl.String())
	require.True(t, ok, "expected orange juice page to be processed")
	assert.Equal(t, `You found me, nice work!`, contentOrangeJuice, "expected orange juice page content to match")
}
