package meter

import (
	"bytes"
	"io"
	"sync/atomic"
)

// provider should be a component that provides information runner requires
type provider interface {
	hasMore() bool
	getMethod(bg *background) string
	getUrl(bg *background) (string, next)
	getHeaders(bg *background) (map[string]string, next)
	getRequestBody(bg *background) (io.ReadCloser, next)
	processResponse(bg *background, status int, body io.Reader) next
	processFailure(bg *background, err error) next
}

// static provider
type staticProvider struct {
	url, method string
	headers map[string]string
	count, seq int64

	body io.Reader
}

// as ReaderCloser
////////////////////////////////////////////////////////////////////////////////
func (s *staticProvider) Read(p []byte) (n int, err error) {
	if s.body != nil {
		return s.body.Read(p)
	}
	return 0, io.EOF
}

func (s *staticProvider) Close() error {
	return nil
}

// as provider
////////////////////////////////////////////////////////////////////////////////
func (s *staticProvider) hasMore() bool {
	return atomic.AddInt64(&s.seq, 1) <= s.count
}

func (s *staticProvider) getMethod(bg *background) string {
	return s.method
}

func (s *staticProvider) getUrl(bg *background) (string, next) {
	return s.url, nextContinue
}

func (s *staticProvider) getHeaders(bg *background) (map[string]string, next) {
	return s.headers, nextContinue
}

func (s *staticProvider) getRequestBody(bg *background) (io.ReadCloser, next) {
	return s, nextContinue
}

func (s *staticProvider) processResponse(bg *background, status int, body io.Reader) next {
	if status != 200 {
		return nextAbortPlan
	}
	return nextContinue
}

func (s *staticProvider) processFailure(bg *background, err error) next {
	return nextAbortPlan
}

func makeStaticProvider(method, url string, body string, count int64) *staticProvider {
	return &staticProvider{
		url:    url,
		method: method,
		body:   bytes.NewReader([]byte(body)),
		count: count,
	}
}