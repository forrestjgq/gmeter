package meter

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/golang/glog"

	"github.com/forrestjgq/gomark"
)

// runner is a single http sender
type runner struct {
	h       *http.Client
	provSrc providerSource
	c       consumer
	name    string
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
