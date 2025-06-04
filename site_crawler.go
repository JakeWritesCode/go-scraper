package main

import (
	"context"
	"github.com/samber/lo"
	"net/url"
	"sync"
	"time"
)

type SiteCrawler struct {
	RobotsChecker       *RobotsChecker
	BaseURL             url.URL
	TimeoutMilliseconds time.Duration
	Logger              Logger
	CrawlQueue          chan func()
	crawlWg             *sync.WaitGroup
	PostProcessQueue    chan func()
	postProcessWg       *sync.WaitGroup
	UserAgent           string
	WorkerPoolSize      int
	crawledPages        sync.Map
	postProcessors      []PostProcessor
}

// Crawl starts the crawling process for the site.
func (sc *SiteCrawler) Crawl(ctx context.Context) error {
	sc.Logger.Debug("Starting site crawler for %s", sc.BaseURL.String())

	sc.startCrawlWorkers(ctx)
	sc.startPostProcessingWorkers(ctx)

	if err := sc.CrawlFromSiteMap(ctx); err != nil {
		sc.Logger.Error("Failed to crawl from sitemap: %v", err)
		return err
	}

	sc.AddURLToCrawlQueue(ctx, &sc.BaseURL)

	sc.crawlWg.Wait()
	close(sc.CrawlQueue)
	sc.Logger.Debug("Crawl complete, waiting for post-processing tasks to finish")
	close(sc.PostProcessQueue)
	sc.postProcessWg.Wait()
	sc.Logger.Debug("All tasks complete")

	return nil
}

// CrawlPage fetches a page, extracts links, and adds them to the crawl queue.
// It also adds the page to the post-processing queue.
func (sc *SiteCrawler) CrawlPage(ctx context.Context, pageURL *url.URL) {
	select {
	case <-ctx.Done():
		sc.Logger.Warn("Crawl aborted for %s: %v", pageURL.String(), ctx.Err())
		return
	default:
	}

	sc.Logger.Debug("Crawling page: %s", pageURL.String())
	timeoutCtx, cancel := context.WithTimeout(ctx, sc.TimeoutMilliseconds*time.Millisecond)
	defer cancel()
	page, err := FetchPage(timeoutCtx, pageURL)
	if err != nil {
		sc.Logger.Warn("Failed to fetch page %s: %v", pageURL.String(), err)
		return
	}
	sc.Logger.Debug("Page fetched successfully: %s", pageURL.String())
	links, err := ExtractLinks(page)
	if err != nil {
		sc.Logger.Error("Failed to extract links from page %s: %v", pageURL.String(), err)
		return
	}
	for _, link := range links {
		parsedLink, err := ResolveAndCleanURL(&sc.BaseURL, link)
		if err != nil {
			sc.Logger.Warn("Skipping invalid link %s on page %s: %v", link, pageURL.String(), err)
			continue
		}
		sc.AddURLToCrawlQueue(ctx, parsedLink)
	}
	sc.AddURLToPostProcessQueue(ctx, pageURL, page)
}

// AddURLToCrawlQueue adds a URL to the crawl queue if it is allowed by robots.txt and matches the base URL host.
func (sc *SiteCrawler) AddURLToCrawlQueue(ctx context.Context, url *url.URL) {
	if !sc.RobotsChecker.IsAllowed(url.String(), sc.UserAgent) {
		sc.Logger.Warn("URL not allowed by robots.txt: %s", url.String())
		return
	}
	if url.Host != sc.BaseURL.Host {
		sc.Logger.Warn("URL host %s does not match base URL host %s, skipping: %s", url.Host, sc.BaseURL.Host, url.String())
		return
	}
	_, loaded := sc.crawledPages.LoadOrStore(url.String(), struct{}{})
	if loaded {
		sc.Logger.Debug("URL already crawled: %s", url.String())
		return
	}
	sc.Logger.Debug("Adding URL to crawl queue: %s", url.String())
	sc.crawlWg.Add(1)
	sc.CrawlQueue <- func() {
		defer sc.crawlWg.Done()
		sc.CrawlPage(ctx, url)
	}
}

// AddURLToPostProcessQueue adds a URL to the post-processing queue for further processing.
func (sc *SiteCrawler) AddURLToPostProcessQueue(ctx context.Context, pageURL *url.URL, pageContent string) {
	for _, processor := range sc.postProcessors {
		sc.postProcessWg.Add(1)
		sc.PostProcessQueue <- func() {
			sc.Logger.Debug("Processing page: %s", pageURL)
			defer sc.postProcessWg.Done()
			if err := processor.Process(ctx, pageURL, pageContent); err != nil {
				sc.Logger.Error("Failed to process page %s: %v", pageURL, err)
			}
		}
	}
}

// CrawlFromSiteMap fetches the sitemap, extracts URLs, and adds them to the crawl queue.
func (sc *SiteCrawler) CrawlFromSiteMap(ctx context.Context) error {
	siteMapUrl, err := sc.BaseURL.Parse("/sitemap.xml")
	if err != nil {
		sc.Logger.Error("Failed to parse sitemap URL: %v", err)
		return err
	}
	siteMap, err := FetchPage(ctx, siteMapUrl)
	if err != nil {
		sc.Logger.Warn("Failed to fetch sitemap: %v", err)
		return nil
	}
	siteMapUrls, err := ParseSitemapForUrls(siteMap)
	if err != nil {
		sc.Logger.Error("Failed to parse sitemap for URLs: %v", err)
		return nil
	}
	lo.ForEach(siteMapUrls, func(raw string, _ int) {
		parsed, err := ResolveAndCleanURL(&sc.BaseURL, raw)
		if err != nil {
			sc.Logger.Warn("Skipping invalid URL in sitemap: %s", raw)
			return
		}
		fullURL := sc.BaseURL.ResolveReference(parsed)
		sc.AddURLToCrawlQueue(ctx, fullURL)
	})
	return nil
}

// startCrawlWorkers starts a pool of workers that will process tasks from the crawl queue.
func (sc *SiteCrawler) startCrawlWorkers(ctx context.Context) {
	for i := 0; i < sc.WorkerPoolSize; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					sc.Logger.Debug("Crawl context cancelled")
					return
				case task, ok := <-sc.CrawlQueue:
					if !ok {
						return
					}
					task()
				}
			}
		}()
	}
}

// startPostProcessingWorkers starts a pool of workers that will process tasks from the post-processing queue.
func (sc *SiteCrawler) startPostProcessingWorkers(ctx context.Context) {
	for i := 0; i < sc.WorkerPoolSize; i++ {
		go func() {
			for {
				select {
				case <-ctx.Done():
					sc.Logger.Debug("Post-process context cancelled")
					return
				case task, ok := <-sc.PostProcessQueue:
					if !ok {
						return
					}
					task()
				}
			}
		}()
	}
}

// PostProcessor defines an interface for post-processing tasks that can be applied to crawled pages.
type PostProcessor interface {
	Process(ctx context.Context, pageURL *url.URL, pageContent string) error
}

// NewSiteCrawler creates a new SiteCrawler instance with the provided configuration.
func NewSiteCrawler(
	ctx context.Context,
	baseURL url.URL,
	logger Logger,
	pageLoadTimeoutMilliseconds time.Duration,
	userAgent string,
	workerPoolSize int,
	postProcessors []PostProcessor,
) (*SiteCrawler, error) {
	sc := &SiteCrawler{
		BaseURL:             baseURL,
		Logger:              logger,
		TimeoutMilliseconds: pageLoadTimeoutMilliseconds,
		UserAgent:           userAgent,
		CrawlQueue:          make(chan func(), 100000), // I/O bound, large link trees clog up the queue
		PostProcessQueue:    make(chan func(), 24),     // CPU bound, 12 cores (may need tweaking)
		WorkerPoolSize:      workerPoolSize,
		postProcessors:      postProcessors,
		crawlWg:             &sync.WaitGroup{},
		postProcessWg:       &sync.WaitGroup{},
	}

	robotsUrl, err := sc.BaseURL.Parse("/robots.txt")
	if err != nil {
		return nil, err
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, sc.TimeoutMilliseconds*time.Millisecond)
	robots, err := FetchPage(timeoutCtx, robotsUrl)
	defer cancel()
	robotsChecker, err := NewRobotsChecker(robots)
	if err != nil {
		return nil, err
	}

	sc.RobotsChecker = robotsChecker
	logger.Debug("New site crawler created for site %s", sc.BaseURL.String())
	return sc, nil
}
