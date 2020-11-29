package gmeter

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/forrestjgq/gomark"
	"github.com/golang/glog"
)

type result struct {
	index int
	err   error
}

func send(t *Test, req *request) (*response, error) {
	msg, err := http.NewRequest(req.method, req.url, bytes.NewReader([]byte(req.body)))
	if err != nil {
		return nil, err
	}
	if len(req.headers) > 0 {
		for k, v := range req.headers {
			msg.Header.Add(k, v)
		}
	}
	rsp, err1 := t.h.Do(msg)
	if err1 != nil {
		return nil, err1
	}

	ret := &response{
		status: rsp.StatusCode,
		body:   nil,
	}
	if rsp.Body != nil {
		b, err2 := ioutil.ReadAll(rsp.Body)
		if err2 != nil {
			return nil, err2
		}
		ret.body = b
		rsp.Body.Close()
	}

	return ret, nil
}
func checkRsp(t *Test, rsp *response, err error) error {
	for _, ck := range t.ResponseCheck {
		if err == nil {
			if ck.ExpectStatus != 0 && ck.ExpectStatus != rsp.status {
				err = fmt.Errorf("expect state %d, get %d", ck.ExpectStatus, rsp.status)
			}
			if ck.NotExpectStatus != 0 && ck.NotExpectStatus == rsp.status {
				err = fmt.Errorf("not expected state %d", ck.ExpectStatus)
			}
		}
		if ck.StopIfFail && err != nil {
			return err
		}
	}

	return nil
}

type counter interface {
	next() (uint64, bool) // returns id, flag_to_continue
}
type privateCounter struct {
	cnt uint64
	now uint64
}

func (p *privateCounter) next() (uint64, bool) {
	n := atomic.AddUint64(&p.now, 1)
	if n > p.cnt {
		return 0, false
	}
	return n, true
}

type infiniteCounter struct {
	now uint64
}

func (i *infiniteCounter) next() (uint64, bool) {
	return atomic.AddUint64(&i.now, 1), true
}

func one(config *Config, s *Schedule, cnt counter) error {

	prevReqs := make([]*request, len(config.Schedules))
	prevRsps := make([]*response, len(config.Schedules))

	for {
		_, keepTest := cnt.next()
		if !keepTest {
			return nil
		}
		if config.aborted() {
			return nil
		}

		for i, name := range s.Series {
			t := config.Tests[name]
			req := compose(config, prevReqs[i], prevRsps[i], t)
			rsp, err := send(t, req)

			prevReqs[i], prevRsps[i] = req, rsp

			err = checkRsp(t, rsp, err)
			if err != nil && config.abort(err) {
				return err
			}
		}
	}
}
func runSchedule(config *Config, s *Schedule) error {
	if s.Concurrency < 2 {
		var cnt counter
		// one by one
		if s.Count != 0 {
			cnt = &privateCounter{cnt: s.Count}
		} else {
			cnt = &infiniteCounter{}
		}
		return one(config, s, cnt)
	} else {
		var cnt counter
		// concurrent
		if !s.CountForEach {
			cnt = &privateCounter{cnt: s.Count}
		}

		wait := 0
		var err error
		c := make(chan error)
		for i := 0; i < s.Concurrency; i++ {
			go func() {
				inCnt := cnt
				if inCnt == nil {
					if s.Count != 0 {
						inCnt = &privateCounter{cnt: s.Count}
					} else {
						inCnt = &infiniteCounter{}
					}
				}
				c <- one(config, s, inCnt)
			}()
		}
		for e := range c {
			wait++
			if err == nil && e != nil {
				err = e
			}
			if wait >= s.Concurrency {
				break
			}
		}
		return err
	}
}
func check(config *Config) error {
	config.ctx = &context{}
	var schedules []*Schedule
	for _, s := range config.Schedules {
		if s == nil {
			continue
		}
		schedules = append(schedules, s)

		for _, name := range s.Series {
			if name == "" {
				continue
			}
			t := config.Tests[name]
			if t == nil {
				return fmt.Errorf("test %s not found", name)
			}

			// prepare http client
			if t.h == nil {
				t.h = &http.Client{}
				if t.Timeout != "" {
					if du, err := time.ParseDuration(t.Timeout); err != nil {
						return err
					} else {
						t.h.Timeout = du
					}
				}

				// setup host and proxy
				if host, ok := config.Hosts[t.Host]; !ok {
					return fmt.Errorf("host %s not exist for test %s", t.Host, name)
				} else {
					if err := host.check(); err != nil {
						return err
					}

					t.host = host.Host
					if host.Proxy != "" {
						proxy := func(_ *http.Request) (*url.URL, error) {
							return url.Parse(host.Proxy)
						}
						t.h.Transport = &http.Transport{
							Proxy: proxy,
						}
					}
				}

				if msg, exist := config.Messages[t.Request]; !exist {
					return fmt.Errorf("message %s for test %s not exist", t.Request, name)
				} else {
					if err := msg.check(); err != nil {
						return err
					}
					t.reqMsg = msg
				}
			}
			s.tests = append(s.tests, t)
		}
	}

	config.Schedules = schedules
	return nil
}

func runOneByOne(config *Config) error {
	for idx, s := range config.Schedules {
		err := runSchedule(config, s)
		if err != nil && config.hasOption(OptionAbortIfFail) {
			glog.Errorf("test stops on %d, error: %v", idx, err)
			return err
		}
	}

	return nil
}
func runConcurrent(config *Config) error {
	c := make(chan *result, 1000)
	wait := 0
	for idx := range config.Schedules {
		go func(index int) {
			c <- &result{
				index: index,
				err:   runSchedule(config, config.Schedules[index]),
			}
		}(idx)
	}

	var ret error
	for r := range c {
		wait++
		if ret == nil && r.err != nil && config.hasOption(OptionAbortIfFail) {
			glog.Errorf("test stops on %d, error: %v", r.index, r.err)
			ret = r.err
		}

		if wait >= len(config.Schedules) {
			break
		}
	}

	return ret
}
func Run(monitorPort int, config *Config) error {

	if err := check(config); err != nil {
		return err
	}

	if monitorPort != 0 {
		gomark.StartHTTPServer(monitorPort)
		config.ctx.monitor = true
	}

	glog.Info("start test")

	switch config.Mode {
	case RunOneByOne:
		return runOneByOne(config)
	case RunConcurrent:
		return runConcurrent(config)
	default:
		return errors.New("unknown mode " + string(config.Mode))
	}

}
