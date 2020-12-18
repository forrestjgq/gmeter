package meter

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/forrestjgq/gomark"
)

// runner is a single http sender
type runner struct {
	h            *http.Client
	p            provider
	c            consumer
	preprocessor []segments
}

func (r *runner) run(bg *background) next {
	var (
		url      string
		body     string
		headers  map[string]string
		decision next
		req      *http.Request
		rsp      *http.Response
		err      error
	)

	p := r.p
	c := r.c

	if p == nil || c == nil || bg == nil {
		return nextAbortAll
	}

	if len(r.preprocessor) > 0 {
		for _, segs := range r.preprocessor {
			_, err = segs.compose(bg)
			if err != nil {
				if isEof(err) {
					return nextFinished
				}
				return c.processFailure(bg, err)
			}
		}
	}

	if decision = p.hasMore(bg); decision != nextContinue {
		return decision
	}

	if url, decision = p.getUrl(bg); decision != nextContinue {
		return decision
	}

	if headers, decision = p.getHeaders(bg); decision != nextContinue {
		return decision
	}

	if body, decision = p.getRequestBody(bg); decision != nextContinue {
		return decision
	}

	bg.setLocalEnv(KeyURL, url)
	bg.setLocalEnv(KeyRequest, body)
	bg.setLocalEnv(KeyURL, url)

	rd := strings.NewReader(body)
	debug := bg.getGlobalEnv(KeyDebug) == "true"

	method := p.getMethod(bg)
	if req, err = http.NewRequest(method, url, rd); err != nil {
		return c.processFailure(bg, err)
	}

	if len(headers) > 0 {
		for k, v := range headers {
			req.Header.Add(k, v)
		}
	}

	var latency *gomark.Latency
	if bg.lr != nil {
		latency = gomark.NewLatency(bg.lr)
	}
	if debug {
		fmt.Printf(`
--------Request-------------
URL: %s %s
Header: %v
Body: %s
`, method, url, headers, body)
	}

	if rsp, err = r.h.Do(req); err != nil {
		return c.processFailure(bg, err)
	} else {
		if latency != nil {
			latency.Mark()
		}

		b, err := ioutil.ReadAll(rsp.Body)
		_ = rsp.Body.Close()

		if debug {
			fmt.Printf(`

--------Response------------
Status: %d
Body: %s
`, rsp.StatusCode, string(b))
		}
		if err != nil {
			return c.processFailure(bg, err)
		}
		bg.setLocalEnv(KeyStatus, strconv.Itoa(rsp.StatusCode))
		bg.setLocalEnv(KeyResponse, string(b))
		decision = c.processResponse(bg)

	}

	return decision
}

// makeRunner will create a runner with valid provider.
// if http.Client h or consumer c is not provided, a default one will be used.
func makeRunner(p provider, h *http.Client, c consumer, preprocess ...string) (runnable, error) {
	if p == nil {
		return nil, errors.New("provider must be provided")
	}
	if h == nil {
		h = http.DefaultClient
	}
	if c == nil {
		c = defaultConsumer
	}
	var segs []segments
	for _, pp := range preprocess {
		if len(pp) > 0 {
			seg, err := makeSegments(pp)
			if err != nil {
				return nil, err
			}
			segs = append(segs, seg)
		}
	}
	return &runner{
		h:            h,
		p:            p,
		c:            c,
		preprocessor: segs,
	}, nil
}
