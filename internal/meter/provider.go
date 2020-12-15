package meter

import (
	"errors"
	"io"
)

// provider should be a component that provides information runner requires
type provider interface {
	hasMore(bg *background) next
	getMethod(bg *background) string
	getUrl(bg *background) (string, next)
	getHeaders(bg *background) (map[string]string, next)
	getRequestBody(bg *background) (string, next)
}

////////////////////////////////////////////////////////////////////////////////
// Static Provider
////////////////////////////////////////////////////////////////////////////////

// staticProvider can be run for specified times with static url/header/method/body
type staticProvider struct {
	url, method string
	headers     map[string]string
	count       uint64
	body        string
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

func (s *staticProvider) getRequestBody(bg *background) (string, next) {
	return s.body, nextContinue
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
	if len(s.url) > 0 && len(s.method) > 0 {
		return nil
	}
	return errors.New("invalid provider")
}

func makeStaticProvider(method, url string, body string, count uint64) (*staticProvider, error) {
	s := &staticProvider{
		url:    url,
		method: method,
		body:   body,
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

// feedProvider
type feedProvider struct {
	s      *staticProvider
	feeder feeder
}

func (f *feedProvider) hasMore(bg *background) next {
	if err := f.feed(bg); err != nil {
		if isEof(err) {
			return nextFinished
		} else {
			bg.report(err)
			return nextAbortPlan
		}
	}
	if err := f.s.check(); err != nil {
		bg.report(err)
		return nextAbortPlan
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

func (f *feedProvider) getRequestBody(bg *background) (string, next) {
	return f.s.getRequestBody(bg)
}

func (f *feedProvider) processResponse(bg *background, status int, body io.Reader) next {
	return f.s.processResponse(bg, status, body)
}

func (f *feedProvider) processFailure(bg *background, err error) next {
	return f.s.processFailure(bg, err)
}

func (f *feedProvider) feed(bg *background) error {
	c, err := f.feeder.feed(bg)
	if err != nil {
		return err
	}
	f.s = &staticProvider{}
	s := f.s
	if c != nil {
		oldHdr := s.headers
		s.headers = make(map[string]string)
		for k, v := range c {
			switch k {
			case catBody:
				s.body = v
			case catMethod:
				s.method = v
			case catURL:
				s.url = v
			default:
				s.headers[string(k)] = v
			}
		}
		if len(s.headers) == 0 {
			s.headers = oldHdr
		}
	}
	return nil
}

func makeFeedProvider(feeder feeder) (provider, error) {
	if feeder == nil {
		return nil, errors.New("invalid feed provider")
	}
	p := &feedProvider{
		s:      &staticProvider{},
		feeder: feeder,
	}

	return p, nil
}
