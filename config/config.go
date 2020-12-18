package config

import (
	"fmt"
	"regexp"
)

type Host struct {
	Host  string // http://domain[:port][/more[/more...]]
	Proxy string // http://[user:password@]domain[:port]
}

func (h Host) Check() error {
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
	PreProcess []string // processing before each HTTP request
	Host       string   // `key` to Config.Hosts, or : [<proxy>|]<host>
	Request    string   // `key` to Config.Messages or `<file>`
	Response   *Response
	Timeout    string // like "5s", "1m10s", "30ms"..., default "1m"
}

type Option string

const (
	OptionAbortIfFail Option = "AbortIfFail" // true or false, default false
	OptionCfgPath     Option = "ConfigPath"  // path to config file, set by gmeter
	OptionDebug       Option = "Debug"       // true or false
)

type Report struct {
	Path   string
	Format string // like: $(RESPONSE), or "{\"Request\": $(Request), \"Status\": $(STATUS), \"Response\": $(Response)}\n"
}
type Schedule struct {
	Name string
	// processing before each tests runs
	PreProcess []string
	// "test1[|test2[|test3...]]", test pipeline composed of one or more tests
	Tests    string
	Reporter Report
	// 0 for infinite, or specified count, default 0.
	// if requests is generated from a list, this field will be ignored
	Count uint64
	// 0 or 1 for one routine, or specified routines, must less than 100000, default: 1 routine
	Concurrency int
	Env         map[string]string // predefined local variables
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
