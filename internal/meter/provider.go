package meter

import (
	"strings"

	"github.com/pkg/errors"
)

// provider should be a component that provides information runner requires
type provider interface {
	getMethod(bg *background) string
	getUrl(bg *background) (string, next)
	getHeaders(bg *background) (map[string]string, next)
	getRequestBody(bg *background) (string, next)
}

// providerSource provide a capability to dynamically generating provider.
// This makes concurrent generating of provider possible.
type providerSource interface {
	getProvider(bg *background) (provider, next)
	close()
}

////////////////////////////////////////////////////////////////////////////////
// Static Provider
////////////////////////////////////////////////////////////////////////////////

// staticProvider can be run for specified times with static url/header/method/body
type staticProvider struct {
	url, method string
	headers     map[string]string
	body        string
}

////////////////////////////////////////////////////////////////////////////////
// as provider
////////////////////////////////////////////////////////////////////////////////

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

func (s *staticProvider) check() error {
	if len(s.url) > 0 && len(s.method) > 0 {
		return nil
	}
	return errors.New("invalid provider")
}

//func makeStaticProvider(method, url string, body string, count uint64) (*staticProvider, error) {
//	s := &staticProvider{
//		url:    url,
//		method: method,
//		body:   body,
//		count:  count,
//	}
//	if err := s.check(); err != nil {
//		return nil, err
//	}
//	return s, nil
//}

////////////////////////////////////////////////////////////////////////////////
// Feed Provider as providerSource
////////////////////////////////////////////////////////////////////////////////

// feedProvider
type feedProvider struct {
	feeder feeder
}

func (f *feedProvider) close() {
	f.feeder.close()
}

func (f *feedProvider) getProvider(bg *background) (provider, next) {
	var err error
	var prov *staticProvider

	if prov, err = f.feed(bg); err != nil {
		if isEof(err) {
			return nil, nextFinished
		} else {
			bg.setError(err)
			return nil, nextAbortPlan
		}
	}

	if err = prov.check(); err != nil {
		bg.setError(err)
		return nil, nextAbortPlan
	}
	return prov, nextContinue
}

func (f *feedProvider) feed(bg *background) (*staticProvider, error) {
	c, err := f.feeder.feed(bg)
	if err != nil {
		return nil, err
	}
	s := &staticProvider{}
	if c != nil {
		oldHdr := s.headers
		s.headers = make(map[string]string)
		for k, v := range c {
			switch k {
			case catBody:
				s.body = v
			case catMethod:
				s.method = strings.TrimSpace(v)
			case catURL:
				s.url = strings.TrimSpace(v)
			default:
				s.headers[string(k)] = v
			}
		}
		if len(s.headers) == 0 {
			s.headers = oldHdr
		}
	}
	return s, nil
}

func makeFeedProvider(feeder feeder) (providerSource, error) {
	if feeder == nil {
		return nil, errors.New("invalid feed provider")
	}
	p := &feedProvider{
		feeder: feeder,
	}

	return p, nil
}
