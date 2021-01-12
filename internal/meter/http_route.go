package meter

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/gorilla/mux"

	"github.com/forrestjgq/gmeter/config"
)

type header struct {
	name   string
	value  segments
	static bool
}

type route struct {
	cfg       config.Route
	headers   map[string]*header
	src       *background
	request   *dynamicConsumer
	responses map[string]composable
}

func (rt *route) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	bg := rt.src.dup()
	bg.cleanup()

	for k, v := range rt.cfg.Env {
		bg.setLocalEnv(k, v)
	}

	// route variables
	vars := mux.Vars(r)
	for k, v := range vars {
		bg.setLocalEnv(k, v)
	}

	// URL query parameters
	q := r.URL.Query()
	for k, v := range q {
		if len(v) == 1 {
			bg.setLocalEnv(k, v[0])
		} else {
			bg.setLocalEnv(k, strings.Join(v, ";"))
		}
	}

	bg.setLocalEnv(KeyURL, r.URL.String())

	b, err := ioutil.ReadAll(r.Body)
	if err == nil && len(b) > 0 {
		bg.setLocalEnv(KeyRequest, string(b))
	}

	for k, h := range rt.headers {
		s := r.Header.Get(k)
		s = strings.ToLower(s)

		// header value as input
		bg.setInput(s)

		sd, err := h.value.compose(bg)
		if err != nil {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(fmt.Sprintf("head %s process fail: %+v", k, err)))
			return
		}

		// if head processor is static string instead of command, compare them
		if h.static {
			sd = strings.ToLower(sd)
			if s != sd {
				w.WriteHeader(400)
				_, _ = w.Write([]byte(fmt.Sprintf("head %s not match: %s != %s", k, s, sd)))
				return
			}
		}
	}

	if rt.request != nil {
		n := rt.request.process(bg, KeyRequest)
		if n != nextContinue {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(fmt.Sprintf("invalid request: %+v", bg.getError())))
			return
		}
	}

	st := bg.getLocalEnv(KeyStatus)
	if len(st) == 0 {
		st = "200"
	}

	sti, err := strconv.Atoi(st)
	if err != nil {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(fmt.Sprintf("STATUS %s invali", st)))
		return
	}
	w.WriteHeader(sti)

	rsp := bg.getLocalEnv(KeyResponse)
	if len(rsp) == 0 {
		rsp = "-"
	}
	crsp := rt.responses[rsp]
	if crsp != nil {
		s, err := crsp.compose(bg)
		if err != nil {
			w.WriteHeader(500)
			_, _ = w.Write([]byte(fmt.Sprintf("response compose fail: %+v", err)))
			return
		}
		_, _ = w.Write([]byte(s))
	}
}
func makeRoute(src *background, cfg *config.Route) (http.Handler, error) {
	r := &route{
		headers:   make(map[string]*header),
		src:       src,
		responses: make(map[string]composable),
	}

	var err error

	for k, v := range cfg.Headers {
		s, err := makeSegments(v)
		if err != nil {
			return nil, errors.Wrapf(err, "make header %s", k)
		}
		h := &header{
			name:   k,
			value:  s,
			static: false,
		}
		if s.isStatic() {
			h.static = true
		}
	}

	r.request, err = makeDynamicConsumer(cfg.Request.Check, cfg.Request.Success, cfg.Request.Failure, cfg.Request.Template, ignoreOnFail)
	if err != nil {
		return nil, errors.Wrapf(err, "make request consumer")
	}

	for k, v := range cfg.Response {
		s, err := makeSegments(string(v))
		if err != nil {
			return nil, errors.Wrapf(err, "make response %s", k)
		}
		r.responses[k] = s
	}

	return r, nil
}
