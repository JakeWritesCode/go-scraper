package main

import (
	"github.com/samber/lo"
	"net/http"
	"net/http/httptest"
	"time"
)

type PageReturn struct {
	HTML              string
	URL               string
	StatusCode        int
	DelayMilliseconds time.Duration
}

func startTestServerPages(pages []PageReturn) *httptest.Server {
	handler := http.NewServeMux()

	lo.ForEach(pages, func(page PageReturn, _ int) {
		handler.HandleFunc(page.URL, func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(page.DelayMilliseconds * time.Millisecond)
			w.WriteHeader(page.StatusCode)
			w.Write([]byte(page.HTML))
		})
	})
	handler.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	return httptest.NewServer(handler)
}
