package main

import (
	"context"
	"io"
	"net/http"
	"net/url"
)

// FetchPage fetches the HTML content of a given page.
// It expects a 2XX response, returning an error if the page is unreachable.
func FetchPage(ctx context.Context, url *url.URL) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url.String(), nil)
	if err != nil {
		return "", err
	}

	client := http.Client{}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", &httpError{StatusCode: resp.StatusCode, URL: url.String()}
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// httpError represents an error that occurs when an HTTP request fails with a non-2XX status code.
type httpError struct {
	StatusCode int
	URL        string
}

// Error implements the error interface for httpError.
func (e *httpError) Error() string {
	return "HTTP error: " + http.StatusText(e.StatusCode) + " from " + e.URL
}
