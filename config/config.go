package gmeter

import (
	"encoding/json"
	"fmt"
	"regexp"
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

type Test struct {
	Host    string          // `key` to Config.Hosts, or : [<proxy>|]<host>
	Request json.RawMessage // `key` to Config.Messages or `fread <file>` or Request it self
	Check   []string        // `cmd1` | `cmd2` | ..., like: `fwrite <file>`
	Timeout string          // like "5s", "1m10s", "30ms"...
}

type Option string

const (
	OptionAbortIfFail Option = "AbortIfFail" // true or false
	OptionSaveFailure Option = "SaveFailure" // to a path
	OptionSaveReport  Option = "SaveReport"  // to a path
)

type Schedule struct {
	Tests       string // "test1[|test2[|test3...]]", test pipeline composed of one or more tests
	Count       uint64 // 0 for infinite, or specified count, default 0
	Concurrency int    // 0 or 1 for one routine, or specified routines, must less than 100000, default: 1 routine
}

type RunMode string

const (
	RunPipe       RunMode = "Pipe"       // Run Schedule one by one, previous failure will not impact next schedule
	RunConcurrent RunMode = "Concurrent" // Run All Config.Schedules concurrently until all exit
)

type Config struct {
	Name     string              // Everyone has a name
	Hosts    map[string]*Host    // predefined hosts
	Messages map[string]*Request // predefined request messages
	Tests    map[string]*Test    // predefined tests

	Mode      RunMode     // how to run schedules, default RunPipe
	Schedules []*Schedule // all test schedules, each one runs a series of tests

	Options map[Option]string // options globally
}

func (c *Config) AddHost(name string, host *Host) {
	if c.Hosts == nil {
		c.Hosts = make(map[string]*Host)
	}
	c.Hosts[name] = host
}
func (c *Config) AddMessage(name string, msg *Request) {
	if c.Messages == nil {
		c.Messages = make(map[string]*Request)
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
