package meter

import (
	"io"
	"net/http"

	"github.com/forrestjgq/gomark"
)

// runner is a single http sender
type runner struct {
	h *http.Client
	p provider
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

	if p == nil || bg == nil {
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
		return p.processFailure(bg, err)
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
		return p.processFailure(bg, err)
	} else {
		if latency != nil {
			latency.Mark()
		}

		decision = p.processResponse(bg, rsp.StatusCode, rsp.Body)

		if rsp.Body != nil {
			_ = rsp.Body.Close()
		}
	}

	return decision
}

func makeRunner(h *http.Client, p provider) runnable {
	return &runner{
		h: h,
		p: p,
	}
}
