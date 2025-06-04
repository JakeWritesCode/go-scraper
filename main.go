package main

import (
	"context"
	"log"
	"net/url"
	"sync"
	"sync/atomic"
)

type URLLoggingWithLinksPostProcessor struct {
	URLsCrawled    sync.Map
	PagesProcessed atomic.Int64
	LinksFound     atomic.Int64
}

func (s *URLLoggingWithLinksPostProcessor) Process(ctx context.Context, pageURL *url.URL, pageContent string) error {
	log.Printf("URLLoggingWithLinksPostProcessor processing page: %s", pageURL.String())
	urls, err := ExtractLinks(pageContent)
	if err != nil {
		return err
	}
	s.URLsCrawled.Store(pageURL.String(), urls)
	s.PagesProcessed.Add(1)
	s.LinksFound.Add(int64(len(urls)))
	return nil
}

func main() {
	baseUrl, _ := url.Parse("https://bbc.co.uk/")
	processor := &URLLoggingWithLinksPostProcessor{}
	logger := StdoutLogger{}
	ctx, cancel := context.WithCancel(context.Background())
	crawler, err := NewSiteCrawler(
		ctx,
		*baseUrl,
		&logger,
		5000,
		"Mozilla/5.0 (compatible; JakeBot/1.0; +https://jakesaunders.dev/bot)",
		2,
		[]PostProcessor{processor},
	)
	if err != nil {
		logger.Error("Failed to create site crawler: %v", err)
		return
	}
	err = crawler.Crawl(ctx)
	if err != nil {
		logger.Error("Failed to crawl site: %v", err)
	} else {
		logger.Info("Crawl completed successfully")
	}

	// print crawled urls as per specification
	logger.Info("-------------------- BEGIN SPECIFICATION OUTPUT --------------------")
	processor.URLsCrawled.Range(func(key, value interface{}) bool {
		logger.Info("Crawled URL: %s and found links:", key.(string))
		for _, link := range value.([]string) {
			logger.Info("     - %s", link)
		}
		return true
	})
	// print summary
	logger.Info("Total pages processed: %d", processor.PagesProcessed.Load())
	logger.Info("Total links found: %d", processor.LinksFound.Load())
	logger.Info("-------------------- END SPECIFICATION OUTPUT --------------------")

	cancel()
}
