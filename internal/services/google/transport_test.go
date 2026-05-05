package google

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
)

// paginatedTransport returns different responses based on the pageToken query
// parameter. If pages is provided with 2 entries, empty pageToken returns
// pages[0] and any non-empty pageToken returns pages[1]. If urls is non-nil,
// all request URLs are recorded.
type paginatedTransport struct {
	pages []string
	urls  *[]string
}

func (p *paginatedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if p.urls != nil {
		*p.urls = append(*p.urls, req.URL.String())
	}

	pageToken := req.URL.Query().Get("pageToken")
	var body string
	if pageToken == "" {
		body = p.pages[0]
	} else {
		body = p.pages[1]
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    req,
	}, nil
}

// contextCancellingTransport wraps an inner transport and calls cancel after
// the first RoundTrip, simulating a context cancellation between pagination pages.
type contextCancellingTransport struct {
	inner  http.RoundTripper
	cancel context.CancelFunc
	once   sync.Once
}

func (c *contextCancellingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	c.once.Do(c.cancel)
	return c.inner.RoundTrip(req)
}
