package gmeter

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"time"

	"github.com/forrestjgq/gomark/gmi"
)

var (
	markBegin = "`"
	markEnd   = "`"
)

type Host struct {
	Host  string // http://domain[:port][/more[/more...]]
	Proxy string // http://[user:password@]domain[:port]
}

func (h Host) check() error {
	matched, matchErr := regexp.Match("^http://([^@:]+:[^@:]+@)?.*(:[0-9]+)?(/[^?&]+)*$", []byte(h.Host))
	if matchErr != nil {
		panic(fmt.Sprintf("http match regexp fail, error: %v", matchErr))
	}
	if !matched {
		return fmt.Errorf("host invalid: %s", h.Host)
	}

	return nil
}

type Check struct {
	StopIfFail      bool
	ExpectStatus    int // set to 0 to ignore this setting, or other code like 200, 400, 4xx, 500, 5xx... that you expect
	NotExpectStatus int // set to 0 to ignore this setting, or other code like 200, 400, 4xx, 500, 5xx... that you not expect
}

type Test struct {
	Host          string // Key to Config.Hosts
	Request       string // key to Config.Messages
	ResponseCheck []*Check
	Timeout       string // like "5s", "1m10s", "30ms"...

	// internal data
	h      *http.Client
	host   string
	reqMsg *Message
}

type Schedule struct {
	Series       []string // Key to Config.Tests, count 1 for each test
	Count        uint64   // 0 for infinite, or specified count
	CountForEach bool     // false if Count is globally set for all concurrent goroutines, true if each goroutine should run Count times
	Concurrency  int      // 0 or 1 for one routine, or specified routines, must less than 100000

	// internal
	tests []*Test
}

type RunMode string

const (
	RunOneByOne   RunMode = "OneByOne"   // Run Schedule one by one, previous failure will not impact next schedule
	RunConcurrent RunMode = "Concurrent" // Run All Config.Schedules concurrently until all exit
)

type Option string

const (
	OptionAbortIfFail Option = "AbortIfFail" // true or false
	OptionSaveFailure Option = "SaveFailure" // to a path
	OptionSaveReport  Option = "SaveReport"  // to a path
)

type context struct {
	monitor bool
	lr      gmi.Marker
	stopped bool
	err     error
}

type Config struct {
	Name     string              // Everyone has a name
	Hosts    map[string]*Host    // predefined hosts
	Messages map[string]*Message // predefined request messages
	Tests    map[string]*Test    // predefined tests

	Mode      RunMode     // how to run schedules
	Schedules []*Schedule // all test schedules, each one runs a series of tests

	Options map[Option]string // options globally

	// internal data
	ctx *context
}

func (c *Config) AddHost(name string, host *Host) {
	if c.Hosts == nil {
		c.Hosts = make(map[string]*Host)
	}
	c.Hosts[name] = host
}
func (c *Config) AddMessage(name string, msg *Message) {
	if c.Messages == nil {
		c.Messages = make(map[string]*Message)
	}
	c.Messages[name] = msg
}
func (c *Config) AddTest(name string, test *Test) {
	if c.Tests == nil {
		c.Tests = make(map[string]*Test)
	}
	c.Tests[name] = test
}
func (c *Config) AddSchedule(name string, schedule *Schedule) {
	c.Schedules = append(c.Schedules, schedule)
}
func (c *Config) AddOption(name Option, value string) {
	if c.Options == nil {
		c.Options = make(map[Option]string)
	}
	c.Options[name] = value
}
func (c *Config) abort(err error) bool {
	if c.abortIfFail() {
		c.ctx.stopped = true
		if c.ctx.err != nil {
			c.ctx.err = err
		}
	}
	return c.ctx.stopped
}
func (c *Config) abortIfFail() bool {
	v, ok := c.Options[OptionAbortIfFail]
	return ok && v == "true"
}
func (c *Config) aborted() bool {
	return c.ctx.stopped
}
func (c *Config) hasOption(option Option) bool {
	_, ok := c.Options[option]
	return ok
}
func (c *Config) preprocess() error {
	c.ctx = &context{}
	var schedules []*Schedule

	for _, s := range c.Schedules {
		if s == nil {
			continue
		}
		schedules = append(schedules, s)

		for _, name := range s.Series {
			if name == "" {
				continue
			}
			t := c.Tests[name]
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
				if host, ok := c.Hosts[t.Host]; !ok {
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

				if msg, exist := c.Messages[t.Request]; !exist {
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

	c.Schedules = schedules
	return nil
}
