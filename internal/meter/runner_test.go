package meter

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net"
	"net/http"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

type TestRunnerBody struct {
	Seq int `json:"seq"`
}
type testRunnerProvider struct {
	req *TestRunnerBody
	c   content
	h   map[string]string
	r   *mux.Router
	s   *http.Server

	fail       next
	expectFail bool
}

func (t *testRunnerProvider) processResponse(bg *background) next {
	if t.checkRequest(bg) != nil {
		return nextAbortAll
	}
	err := t.matchResponse(bg)
	if err != nil {
		bg.setError(err)
		return nextAbortPlan
	}
	return nextContinue
}

func (t *testRunnerProvider) processFailure(bg *background, err error) next {
	if !t.expectFail {
		return nextAbortAll
	}
	return t.fail
}
func (t *testRunnerProvider) checkRequest(bg *background) error {
	url := bg.getLocalEnv(KeyURL)
	body := bg.getLocalEnv(KeyRequest)

	if url != t.c[catURL] {
		return errors.New("url not match : " + url)
	}
	if len(body) == 0 {
		return errors.New("no body")
	}

	var v TestRunnerBody
	err := json.Unmarshal([]byte(body), v)
	if err != nil {
		return err
	}

	if v.Seq != t.req.Seq {
		return errors.New("not expect seq")
	}
	return nil
}

func (t *testRunnerProvider) getMethod(bg *background) string {
	return t.c[catMethod]
}

func (t *testRunnerProvider) getUrl(bg *background) (string, next) {
	return t.c[catURL], nextContinue
}

func (t *testRunnerProvider) getHeaders(bg *background) (map[string]string, next) {
	return t.h, nextContinue
}

func (t *testRunnerProvider) getRequestBody(bg *background) (string, next) {
	return t.c[catBody], nextContinue
}

func (t *testRunnerProvider) getProvider(bg *background) (provider, next) {
	return t, nextContinue
}
func (t *testRunnerProvider) matchResponse(bg *background) error {
	b := []byte(bg.getLocalEnv(KeyResponse))

	var v TestRunnerBody
	err := json.Unmarshal(b, &v)
	if err != nil {
		return err
	}
	if v.Seq != t.req.Seq {
		return errors.New("no match")
	}

	return nil
}
func (t *testRunnerProvider) startServer(seq int) error {
	t.r = mux.NewRouter()
	t.r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("content-type") != "application/json" ||
			r.Header.Get("custom") != "header" {
			w.WriteHeader(400)
			return
		}

		if r.Method != t.c[catMethod] {
			if r.Method != "GET" || t.c[catMethod] != "" {
				w.WriteHeader(400)
				return
			}
		}

		b, _ := ioutil.ReadAll(r.Body)
		if len(b) > 0 {
			_, _ = w.Write(b)
		}
		_ = r.Body.Close()
	})
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return err
	}
	t.s = &http.Server{
		Handler: t.r,
	}
	go t.s.Serve(l)

	list := strings.Split(l.Addr().String(), ":")
	port := list[len(list)-1]
	t.c[catMethod] = "POST"
	t.c[catURL] = "http://127.0.0.1:" + port
	t.req = &TestRunnerBody{Seq: seq}
	body, err := json.Marshal(t.req)
	if err != nil {
		return err
	}
	t.c[catBody] = string(body)
	t.h["custom"] = "header"
	t.h["content-type"] = "application/json"
	return nil
}
func (t *testRunnerProvider) stopServer() {
	_ = t.s.Close()
}

func TestRunner(t *testing.T) {
	p := &testRunnerProvider{c: map[category]string{}, h: map[string]string{}}
	if err := p.startServer(1); err != nil {
		t.Fatalf(err.Error())
	}
	defer p.stopServer()

	r, err := makeRunner("test", p, nil, nil)
	if err != nil {
		t.Fatalf(err.Error())
	}

	bg, err := createDefaultBackground()
	if err != nil {
		t.Fatalf(err.Error())
	}
	bg.setGlobalEnv(KeyDebug, "true")
	n := r.run(bg)
	if n != nextContinue {
		t.Fatal("expect continue")
	}
	err = p.matchResponse(bg)
	if err != nil {
		t.Fatalf(err.Error())
	}

}
func TestRunnerFail(t *testing.T) {
	p := &testRunnerProvider{c: map[category]string{}, h: map[string]string{}}
	if err := p.startServer(2); err != nil {
		t.Fatalf(err.Error())
	}
	defer p.stopServer()

	r, err := makeRunner("test", p, nil, p)
	if err != nil {
		t.Fatalf(err.Error())
	}

	bg, err := createDefaultBackground()
	if err != nil {
		t.Fatalf(err.Error())
	}

	bg.setGlobalEnv(KeyDebug, "true")
	p.c[catURL] = "http://127.0.0.1:65333"
	p.expectFail = true
	p.fail = nextAbortPlan
	n := r.run(bg)
	if n != nextAbortPlan {
		t.Fatal("expect abort, got ", n)
	}
	if bg.getLocalEnv(KeyStatus) != "" {
		t.Fatal("expect status 400, got ", bg.getLocalEnv(KeyStatus))
	}
}
