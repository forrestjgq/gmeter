package meter

import (
	"io"
	"net/http"

	"github.com/golang/glog"

	"github.com/forrestjgq/gomark"
)

// runner is a single http sender
type runner struct {
	h *http.Client
	p provider
	c consumer
}

func (r *runner) run(bg *background) next {
	var (
		url      string
		body     io.ReadCloser
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

	if req, err = http.NewRequest(p.getMethod(bg), url, body); err != nil {
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

	if rsp, err = r.h.Do(req); err != nil {
		return c.processFailure(bg, err)
	} else {
		if latency != nil {
			latency.Mark()
		}

		decision = c.processResponse(bg, rsp.StatusCode, rsp.Body)

		if rsp.Body != nil {
			_ = rsp.Body.Close()
		}
	}

	return decision
}

// makeRunner will create a runner with valid provider.
// if http.Client h or consumer c is not provided, a default one will be used.
func makeRunner(p provider, h *http.Client, c consumer) runnable {
	if p == nil {
		glog.Error("provider must be provided")
		return nil
	}
	if h == nil {
		h = http.DefaultClient
	}
	if c == nil {
		c = defaultConsumer
	}
	return &runner{
		h: h,
		p: p,
		c: c,
	}
}
