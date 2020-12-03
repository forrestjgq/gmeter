package meter

import (
	"bytes"
	"errors"
	"io"
)

// provider should be a component that provides information runner requires
type provider interface {
	hasMore(bg *background) next
	getMethod(bg *background) string
	getUrl(bg *background) (string, next)
	getHeaders(bg *background) (map[string]string, next)
	getRequestBody(bg *background) (io.ReadCloser, next)
	processResponse(bg *background, status int, body io.Reader) next
	processFailure(bg *background, err error) next
}

////////////////////////////////////////////////////////////////////////////////
// Static Provider
////////////////////////////////////////////////////////////////////////////////

// staticProvider can be run for specified times with static url/header/method/body
type staticProvider struct {
	url, method string
	headers     map[string]string
	count       int64

	body *bytes.Reader
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
func (s *staticProvider) hasMore(bg *background) next {
	if bg.seq < s.count {
		return nextContinue
	}
	return nextFinished
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
func (s *staticProvider) check() error {
	if len(s.url) > 0 && len(s.method) > 0 && s.body.Size() > 0 {
		return nil
	}
	return errors.New("invalid provider")
}

func makeStaticProvider(method, url string, body string, count int64) (*staticProvider, error) {
	s := &staticProvider{
		url:    url,
		method: method,
		body:   bytes.NewReader([]byte(body)),
		count:  count,
	}
	if err := s.check(); err != nil {
		return nil, err
	}
	return s, nil
}

////////////////////////////////////////////////////////////////////////////////
// Feed Provider
////////////////////////////////////////////////////////////////////////////////

type category string
type content map[category]string

const (
	catMethod category = "_mtd_"
	catURL    category = "_url_"
	catHeader category = "_hdr_"
	catBody   category = "_bdy_"
	catEnd    category = "_eof_"
)

type feeder func(seq int64) content

// feedProvider
type feedProvider struct {
	s   *staticProvider
	f   feeder
	end bool
}

func (f *feedProvider) hasMore(bg *background) next {
	f.feed(bg)
	if err := f.s.check(); err != nil {
		bg.report(err)
		return nextAbortPlan
	}
	if f.end {
		return nextFinished
	}
	return nextContinue
}

func (f *feedProvider) getMethod(bg *background) string {
	return f.s.getMethod(bg)
}

func (f *feedProvider) getUrl(bg *background) (string, next) {
	return f.s.getUrl(bg)
}

func (f *feedProvider) getHeaders(bg *background) (map[string]string, next) {
	return f.s.getHeaders(bg)
}

func (f *feedProvider) getRequestBody(bg *background) (io.ReadCloser, next) {
	return f.s.getRequestBody(bg)
}

func (f *feedProvider) processResponse(bg *background, status int, body io.Reader) next {
	return f.s.processResponse(bg, status, body)
}

func (f *feedProvider) processFailure(bg *background, err error) next {
	return f.s.processFailure(bg, err)
}

func (f *feedProvider) feed(bg *background) {
	c := f.f(bg.seq)
	s := f.s
	if c != nil {
		oldHdr := s.headers
		s.headers = make(map[string]string)
		for k, v := range c {
			switch k {
			case catBody:
				s.body = bytes.NewReader([]byte(v))
			case catMethod:
				s.method = v
			case catURL:
				s.url = v
			case catEnd:
				f.end = true
				return
			default:
				s.headers[string(k)] = v
			}
		}
		if len(s.headers) == 0 {
			s.headers = oldHdr
		}
	}
}

func makeFeedProvider(s *staticProvider, f feeder) (provider, error) {
	if s == nil || f == nil {
		return nil, errors.New("invalid feed provider")
	}
	p := &feedProvider{
		s: s,
		f: f,
	}

	return p, nil
}
