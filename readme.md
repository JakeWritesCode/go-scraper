I wrote this crawler as a technical test for a Go position, but thought it was pretty neat so I'm sharing it here.

# Site Crawler

A concurrent, production-ready site crawler written in Go. Crawls all pages on a single subdomain, obeys robots.txt, and
supports pluggable post-processing for scalable data extraction.

## Features

- Pluggable post-processors — Add custom behavior per-page without modifying crawl logic.
- Buffered worker pools — Separate crawl and post-processing workers for efficiency.
- Timeouts and cancellation — Crawls are scoped with context timeouts to avoid hanging.
- Respects robots.txt — Uses a compliant parser and honors disallow rules.
- Sitemap bootstrapping — Crawls from `/sitemap.xml` if available.
- Link normalization and deduplication — Avoids redundant crawling via sync.Map.
- Configurable — Set timeouts, user-agent, worker pool sizes, and more.
- Logger abstraction — Swap in observability tools like OpenTelemetry with minimal changes.

## Usage
To run this example:

- Install go locally on your machine.
- `cd goTechTest`
- `go get`
- `go run .`

## Extensibility

Crawl behavior is decoupled from what you do with each page. To hook into results, implement the PostProcessor
interface:

```go
type PostProcessor interface {
Process(ctx context.Context, pageURL *url.URL, pageContent string) error
}
```

Processors run in parallel, one per page, and can:

- Save content to a datastore
- Send structured data to Kafka/SQS/etc.
- Extract structured data for search or indexing

> Example: URLLoggingWithLinksPostProcessor stores links found on each page and logs them. See main.go.

## Dependencies

This implementation almost entirely uses the go stdlib, with a few exceptions:

- `samber/lo`: This is possibly divisive but I find that adding some functional programming concepts into Go (such as
  `Filter`, `Map`, `Reduce`, `ForEach`) improves readability. The addition of generics in go 1.18 makes this possible
  while remaining type safe.
- `stretchr/testify`: Nicer assertions in tests, particularly `require.Never` and `require.Eventually` for concurrency
  testing.
- `temoto/robotstxt`: Parsing and querying of `robots.txt`. This may be abandonware (last commit 4 years ago), but its
  scope in this implementation is limited, and it's well abstracted so could be replaced easily.

## Design Trade-offs

### Crawl Queue Size

To avoid deadlocks on large or highly-connected sites, the crawl queue is buffered:

```go
CrawlQueue: make(chan func (), 100000)
```

This:

- Prevents blocking when pages emit thousands of links.
- Keeps I/O-bound crawls moving.
- ⚠️ Risks higher memory usage under pathological inputs.

With more time I would:

- Add instrumentation (queue depth, memory)
- Implement prioritization/backpressure
- Replace with a bounded scheduler if needed

## Testing

The crawler includes a comprehensive test suite covering:

- Context cancellation and timeout handling
- robots.txt compliance
- Sitemap crawling
- Link discovery and resolution
- Concurrency behavior
- Deduplication logic
- Deep recursive links
- Invalid/malformed HTML and sitemaps

Run tests with:

```bash
go test ./...
```

## Possible Improvements

- Crawl queue backpressure: The 100k buffer prevents stalling, but isn't ideal. A real implementation would track usage
  and apply limits or prioritization.
- No rate limiting: Politeness is enforced via max workers and timeouts, but not crawl-delay headers. Could be added
  with more time.
- No retry/backoff: Failures are logged and skipped. In production, you'd likely want retry logic.
- No observability hooks yet: Logger interface is abstracted. Metrics/tracing could be added via context-aware
  middleware.
- GET param handling: Query strings are preserved. This could result in duplicate pages being crawled, but it's possible
  that the query params could meaningfully change page content so I've opted not to strip them. In production, you'd
  optionally strip tracking params like utm_*.

