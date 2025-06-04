package main

import (
	"context"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func startTestServer(html string, statusCode int, delayMilliseconds time.Duration) *httptest.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(delayMilliseconds * time.Millisecond)
		w.WriteHeader(statusCode)
		w.Write([]byte(html))
	})
	return httptest.NewServer(handler)
}

func TestFetchPage_ReturnsError_Non2XXStatus(t *testing.T) {
	t.Parallel()
	server := startTestServer("", http.StatusInternalServerError, 0)
	defer server.Close()

	serverUrl, _ := url.Parse(server.URL)
	_, err := FetchPage(context.Background(), serverUrl)
	assert.Error(t, err)
}

func TestFetchPage_Success_ReturnsBody(t *testing.T) {
	t.Parallel()
	server := startTestServer("<html><body>Test Page</body></html>", http.StatusOK, 0)
	defer server.Close()

	serverUrl, _ := url.Parse(server.URL)
	content, err := FetchPage(context.Background(), serverUrl)
	require.NoError(t, err)
	assert.Equal(t, "<html><body>Test Page</body></html>", content)
}

func TestFetchPage_ReturnsError_Timeout(t *testing.T) {
	t.Parallel()
	server := startTestServer("<html><body>Test Page</body></html>", http.StatusOK, 2000)
	defer server.Close()

	serverUrl, _ := url.Parse(server.URL)
	ctx, _ := context.WithTimeout(context.Background(), 100*time.Millisecond)
	_, err := FetchPage(ctx, serverUrl)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestFetchPage_RespectsContextShutdown(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(context.Background())
	server := startTestServer("<html><body>Test Page</body></html>", http.StatusOK, 2000)
	defer server.Close()

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	serverUrl, _ := url.Parse(server.URL)
	_, err := FetchPage(ctx, serverUrl)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context canceled")
}

func TestFetchPage_ReturnsError_InvalidURL(t *testing.T) {
	t.Parallel()

	serverUrl, _ := url.Parse("http://invalid-url")
	_, err := FetchPage(context.Background(), serverUrl)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no such host")
}
