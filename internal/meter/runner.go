package meter

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/forrestjgq/glog"

	"github.com/forrestjgq/gomark"
)

// runner is a single http sender
type runner struct {
	h       *http.Client
	provSrc providerSource
	c       consumer
	name    string
}

func (r *runner) close() {
	r.provSrc.close()
	r.h.CloseIdleConnections()
}

func (r *runner) do(bg *background, req *http.Request) (*http.Response, error) {
	if bg.fc != nil {
		defer bg.fc.wait().cancel()
	}
	var latency *gomark.Latency
	if bg.perf != nil {
		latency = gomark.NewLatency(bg.perf.lr)
		if bg.perf.adder != nil {
			bg.perf.adder.Mark(1)
		}
	}

	rsp, err := r.h.Do(req)
	if err != nil {
		if e, ok := err.(*url.Error); ok && e.Err == io.EOF {
			rsp, err = r.h.Do(req)
		}
	}

	if bg.perf != nil && bg.perf.adder != nil {
		bg.perf.adder.Mark(-1)
	}
	// only successful request count latency
	if err == nil && latency != nil {
		latency.Mark()
		bg.reportLatency(latency.Latency())
	}
	return rsp, err
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
		p        provider
	)

	c := r.c

	if r.provSrc == nil || c == nil || bg == nil {
		glog.Error("invalid runner")
		return nextAbortAll
	}
	bg.setLocalEnv(KeyTest, r.name)

	if p, decision = r.provSrc.getProvider(bg); decision != nextContinue {
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

	rd := strings.NewReader(body)
	debug := bg.getGlobalEnv(KeyDebug) == "true"

	method := p.getMethod(bg)

	if debug {
		fmt.Printf(`
--------Request %s-%s -------------
URL: %s %s
Header: %v
Body: %s
`, bg.getLocalEnv(KeyRoutine), bg.getLocalEnv(KeySequence), method, url, headers, body)
	}

	if req, err = http.NewRequest(method, url, rd); err != nil {
		return c.processFailure(bg, errors.Wrap(err, "create http request"))
	}

	if len(headers) > 0 {
		for k, v := range headers {
			req.Header.Add(k, v)
		}
	}

	rsp, err = r.do(bg, req)
	if err != nil {
		return c.processFailure(bg, errors.Wrap(err, "execute http request"))
	}

	b, err := ioutil.ReadAll(rsp.Body)
	_ = rsp.Body.Close()

	if debug {
		fmt.Printf(`

--------Response %s-%s ------------
Status: %d
Body: %s
`, bg.getLocalEnv(KeyRoutine), bg.getLocalEnv(KeySequence), rsp.StatusCode, string(b))
	}
	if err != nil {
		return c.processFailure(bg, errors.Wrap(err, "read body"))
	}
	bg.setLocalEnv(KeyStatus, strconv.Itoa(rsp.StatusCode))
	bg.setLocalEnv(KeyResponse, string(b))
	decision = c.processResponse(bg)
	return decision
}

// makeRunner will create a runner with valid provider.
// if http.Client h or consumer c is not provided, a default one will be used.
func makeRunner(name string, provSrc providerSource, h *http.Client, c consumer) (runnable, error) {
	if provSrc == nil {
		return nil, errors.New("provider must be provided")
	}
	if h == nil {
		h = http.DefaultClient
	}
	if c == nil {
		c = defaultConsumer
	}
	return &runner{
		name:    name,
		h:       h,
		provSrc: provSrc,
		c:       c,
	}, nil
}
