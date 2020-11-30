package gmeter

import (
	"io"
	"net/http"

	"github.com/forrestjgq/gomark"
	"github.com/forrestjgq/gomark/gmi"
)

type next int

const (
	nextContinue next = iota
	nextAbortPlan
	nextAbortRunner
	nextAbortAll
)

type variable string

// container to store environment
type env interface {
	get(key string) variable
	put(key string, value variable)
	delete(key string)
	has(key string)
}
type background struct {
	name          string
	seq           int
	local, global env
	lr            gmi.Marker
}

func (bg *background) report() {

}

type urlProvider interface {
	get(bg *background) (string, next)
}
type headerProvider interface {
	get(bg *background) (map[string]string, next)
}
type bodyProvider interface {
	get(bg *background) (io.ReadCloser, next)
}
type rspReceiver interface {
	processResponse(bg *background, status int, body io.Reader) next
}
type failProcessor interface {
	processFailure(bg *background, err error) next
}

type plan struct {
	h      *http.Client
	method string
	url    urlProvider
	header headerProvider
	body   bodyProvider
	rsp    rspReceiver
	fail   failProcessor
}

func (p *plan) run(bg *background) next {
	var (
		url      string
		body     io.ReadCloser
		headers  map[string]string
		decision next
	)

	if p.fail == nil || p.url == nil {
		return nextAbortAll
	}

	url, decision = p.url.get(bg)
	if decision != nextContinue {
		return decision
	}

	if p.header != nil {
		headers, decision = p.header.get(bg)
		if decision != nextContinue {
			return decision
		}
	}

	if p.body != nil {
		body, decision = p.body.get(bg)
		if decision != nextContinue {
			return decision
		}
	}

	msg, err := http.NewRequest(p.method, url, body)
	if err != nil {
		return p.fail.processFailure(bg, err)
	}

	if len(headers) > 0 {
		for k, v := range headers {
			msg.Header.Add(k, v)
		}
	}

	var latency *gomark.Latency
	if bg.lr != nil {
		latency = gomark.NewLatency(bg.lr)
	}
	if rsp, err1 := p.h.Do(msg); err1 != nil {
		return p.fail.processFailure(bg, err1)
	} else {
		if latency != nil {
			latency.Mark()
		}

		if p.rsp != nil {
			decision = p.rsp.processResponse(bg, rsp.StatusCode, rsp.Body)
		}

		if rsp.Body != nil {
			_ = rsp.Body.Close()
		}
	}

	return decision
}

type runner struct {
}
