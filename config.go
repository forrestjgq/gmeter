package gmeter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"

	"github.com/forrestjgq/gomark/gmi"
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

type Message struct {
	Path    string // /path/to/target
	Method  string
	Headers map[string]string
	Params  map[string]string // Path?key=value&key=value..
	Body    json.RawMessage
}

func (m *Message) check() error {
	if matched, matchErr := regexp.Match("^(/[^/]+)*$", []byte(m.Path)); matchErr != nil {
		panic("message match regexp invalid")
	} else if !matched {
		return fmt.Errorf("invalid path: %s", m.Path)
	}

	switch m.Method {
	case http.MethodGet:
		if m.Body != nil {
			return fmt.Errorf("message %s GET with message body", m.Path)
		}
	case http.MethodPut:
	case http.MethodDelete:
		if m.Body != nil {
			return fmt.Errorf("message %s DELETE with message body", m.Path)
		}
	case http.MethodPost:
	case http.MethodPatch:
	default:
		return fmt.Errorf("invalid method: %s for host %s", m.Method, m.Path)
	}

	if m.Headers == nil {
		m.Headers = make(map[string]string)
	}
	if m.Params == nil {
		m.Params = make(map[string]string)
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
	Name      string
	Mode      RunMode
	Hosts     map[string]*Host
	Messages  map[string]*Message
	Tests     map[string]*Test
	Schedules []*Schedule
	Options   map[Option]string

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
