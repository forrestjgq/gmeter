// Package config defines a configuration for gmeter to use to start a series HTTP restful test.
//
//     Note: Fields defined in this package with `[dynamic]` comments allows environment
//     variables and commands embedding. Refer to of gmeter command document
//     https://github.com/forrestjgq/gmeter/blob/main/command.md for detail description.
//
// gmeter runs HTTP by definition of Schedule(s). Each schedule, defining one HTTP test, or a pipeline
// of HTTP tests, is ran by gmeter independently, and contains a series of HTTP requests execution.
// These requests can be linearly executed one by one, or concurrently executed through multiple routines.
//
// Request can be executed repeatedly for specified rounds, or be dynamically generated until it reaches
// EOF. See iterable command section in command document for more information:
//          https://github.com/forrestjgq/gmeter/blob/main/command.md#iterable-command
//
// Each request execution contains these steps:
//  - PreProcess: prepare for request generation, like setting up environment
//  - Request generation: parsing request definition and generates an HTTP request.
//  - HTTP execution: send HTTP request, and write status code and response
//    into environment.
//  - Response processing: including response check, success and failure processing,
//    or report writing.
//
// Before reading the following samples, it's strongly recommended for you to read Config and all
// related definitions.
//
// The first sample is a static definition. It assumes your target server allows POST/GET/DELETE access
// to '/repo' to operate storage quantity of a fruit, and you need write a new fruit, then read to check
// if it's written, and delete it for next tests. Here is it:
//		{
//		    "Name": "fruit-repo",
//		    "Hosts": {
//		        "localhost": {
//		            "Host": "http://127.0.0.1:8000"
//		        }
//		    },
//		    "Messages": {
//		        "quantity-write": {
//		            "Method": "POST",
//		            "Path": "/repo",
//		            "Headers": {
//		                "content-type": "application/json"
//		            },
//		            "Body": {
//		                "repo": "fruit",
//		                "type": "apple",
//		                "quantity": 300
//		            }
//		        },
//		        "quantity-read": {
//		            "Method": "GET",
//		            "Path": "/repo?id=fruit&type=apple"
//		        },
//		        "quantity-delete": {
//		            "Method": "DELETE",
//		            "Path": "/repo?id=fruit&type=apple"
//		        }
//		    },
//		    "Tests": {
//		        "test-write": {
//		            "Host": "localhost",
//		            "Request": "write",
//		            "Timeout": "3s"
//		        },
//		        "test-read": {
//		            "Host": "localhost",
//		            "Request": "write",
//		            "Timeout": "1s"
//		        },
//		        "test-delete": {
//		            "Host": "localhost",
//		            "Request": "write",
//		            "Timeout": "1s"
//		        }
//		    },
//		    "Schedules": [
//		        {
//		            "Name": "quantity",
//		            "Tests": "test-write|test-read|test-delete",
//		            "Count": 100000,
//		            "Concurrency": 1
//		        }
//		    ],
//		    "Options": {
//		        "AbortIfFail": "true"
//		    }
//		}
//
// You can see that `quantity` Schedule defines a pipeline of tests composed by `test-write`, `test-read`,
// `test-delete`, and runs in a single routine for 100K times. Each time gmeter sends there 3 request:
//   - request 1: POST http://127.0.0.1:8000/repo, body:
//		          {
//		              "repo": "fruit",
//		              "type": "apple",
//		              "quantity": 300
//		          }
//   - request 2: GET http://127.0.0.1:8000/repoid=fruit&type=apple
//   - request 3: DELETE http://127.0.0.1:8000/repoid=fruit&type=apple
//
// You may not be satisfied with these tests for these reasons:
//  - no concurrent
//  - only apple is tested
//  - test-read does not check response
//  - or more for greed you
//
// Now dynamic request can be a great tool, let's improve it.
//
// First, create a list, each line of which is a json of fruit name and quantity like:
// 		{"fruit": "apple", "quantity": 100}
// 		{"fruit": "orange", "quantity": 310}
// 		{"fruit": "pear", "quantity": 0}
// 		{...}
//
// The name is irrelevant, so you may create random names to make these list bigger enough like
// 100k lines.
//
// then we define configuration:
//		 {
//		    "Name": "fruit-repo",
//		    "Hosts": {
//		        "localhost": {
//		            "Host": "http://127.0.0.1:8000"
//		        }
//		    },
//		    "Messages": {
//		        "quantity-write": {
//		            "Method": "POST",
//		            "Path": "/repo",
//		            "Headers": {
//		                "content-type": "application/json"
//		            },
//		            "Body": {
//		                "repo": "fruit",
//		                "type": "$(FRUIT)",
//		                "quantity": "`cvt -i $(QTY)`"
//		            },
//		            "Response": {
//		                "Check": [
//		                    "`assert $(STATUS) == 200`"
//		                ]
//		            }
//		        },
//		        "quantity-read": {
//		            "Method": "GET",
//		            "Path": "/repo?type=$(FRUIT)",
//		            "Response": {
//		                "Check": [
//		                    "`assert $(STATUS) == 200`",
//		                    "`json .type $(RESPONSE) | assert $(INPUT) == $(FRUIT)`",
//		                    "`json .quantity $(RESPONSE) | assert $(INPUT) == $(QTY)`"
//		                ]
//		            }
//		        },
//		        "quantity-delete": {
//		            "Method": "DELETE",
//		            "Path": "/repo?type=$(FRUIT)",
//		            "Response": {
//		                "Check": [
//		                    "`assert $(STATUS) == 200`"
//		                ]
//		            }
//		        }
//		    },
//		    "Tests": {
//		        "test-write": {
//		            "PreProcess": [
//		                "`list /path/to/fruit/list | envw JSON`",
//		                "`json .fruit | envw FRUIT`",
//		                "`json .quantity | envw QTY`"
//		            ],
//		            "Host": "localhost",
//		            "Request": "write",
//		            "Timeout": "3s"
//		        },
//		        "test-read": {
//		            "Host": "localhost",
//		            "Request": "write",
//		            "Timeout": "1s"
//		        },
//		        "test-delete": {
//		            "Host": "localhost",
//		            "Request": "write",
//		            "Timeout": "1s"
//		        }
//		    },
//		    "Schedules": [
//		        {
//		            "Name": "quantity",
//		            "Tests": "test-write|test-read|test-delete",
//		            "Concurrency": 100
//		        }
//		    ],
//		    "Options": {
//		        "AbortIfFail": "true"
//		    }
//		}
//
// In this configuration, there are still 3 tests composing a pipeline, but as first request, test-write
// applies a preprocess, which read a line from list and write fruit and quantity into environment variable
// `$(FRUIT)` and `$(QTY)`. These two variables will be quoted in request generation and response checking.
//
// You may note that each case adds a `Response|Check`, containing one or several commands to check response
// status and content.
//
// In `Schedules` a concurrency of 100 is applied.
//
// These covers all your needs.
//
// Read commands document for more tools to generate requests and process response.
package config

import (
	"encoding/json"
	"fmt"
	"regexp"
)

// Host defines a server and proxy to visit this server
type Host struct {
	// format: http://domain[:port][/more[/more...]], https is not supported yet.
	Host string
	// Proxy defines a proxy used to access Host.
	// format: <protocol>://[user:password@]domain[:port], protocol could be http or socks5
	Proxy string
}

// Check validates Host setting.
func (h *Host) Check() error {
	matched, matchErr := regexp.Match("^http://([^@:]+:[^@:]+@)?.*(:[0-9]+)?(/[^?&]+)*$", []byte(h.Host))
	if matchErr != nil {
		panic(fmt.Sprintf("http match regexp fail, error: %v", matchErr))
	}
	if !matched {
		return fmt.Errorf("host invalid: %s", h.Host)
	}

	return nil
}

// Test defines parameters required to execute an HTTP request.
//
// gmeter will first call PreProcess if defined any, then use Host and RequestMessage
// or Request to search HTTP server and request message definitions and combining them
// to generate a real HTTP request. Request URL will be written to $(URL). Request body,
// if any, will be written to $(REQUEST).
//
// if both Request and RequestMessage are defined, RequestMessage is preferred.
//
// While server responds, status code will be written to $(STATUS), and response body,
// if any, will be written to $(RESPONSE). Then Response.Success will be called.
//
// If any failure occurs duration above procedures, Response.Failure will be called.
type Test struct {
	PreProcess     []string  // [dynamic] processing before each HTTP request
	Host           string    // `key` to Config.Hosts, or : [<proxy>|]<host>
	Request        string    // `key` to Config.Messages
	RequestMessage *Request  // request message definition, preferred over Request
	Response       *Response // Optional entity used to process response or failure
	Timeout        string    // HTTP request timeout, like "5s", "1m10s", "30ms"..., default "1m"
}

// Option defines options gmeter accepts. These options can be used as key in Config.Options.
type Option string

const (
	// "true" or "false", default "false"
	// If set to true, test will be aborted if any error in any concurrent routine occurs.
	OptionAbortIfFail Option = "AbortIfFail"

	// internal usage.
	// path to config file, set by gmeter.
	OptionCfgPath Option = "ConfigPath"

	// internal usage. "true" or "false", default "false".
	// set to true to enable gmeter dumping.
	OptionDebug Option = "Debug" // true or false
)

// Report allows test write customized content into given file.
//
// Format behaves as an template and guide gmeter to parse it's definition,
// and compose eventually string and write to file indicated by Path. It's
// used only while command `report` is called without `-f` and `-t` options.
//
// Templates defines some json templates, referred with key by `report -t` to
// provides a convenient template definition for complex json.
//
// If Path is empty, but format is not, content will be written to stdout.
//
// Note that no line carrier return will be appended by gmeter.
type Report struct {
	// Path defines file path where report will write to.
	//
	// If Path is a relative path like "a/b/c", it will be treated to be relative
	// to config file path. For example, config file path is: "/home/user/test/gmeter.json",
	// Path will be converted to "/home/user/test/a/b/c".
	//
	// If Path already exists, it will be truncated.
	//
	// Any necessary parents in path will be created.
	//
	// [dynamic]
	Path string

	// Format defines a default format of report content. it's implicitly quoted as argument
	// if command `report` is used without given an argument `-f <format>`.
	//
	// For example, this will write response of every successful response body:
	// 		"$(RESPONSE)\n"
	// or this will create a json to save request body, response status, and response body.
	//		"{\"Request\": $(REQUEST), \"Status\": $(STATUS), \"Response\": $(RESPONSE)}\n"
	//
	// [dynamic]
	Format string

	// Templates is used to compose a complicate json reporting while Format is not good enough
	// for you.
	//
	// `report -t <key>` could refer the key of Templates to report a json formation content
	// by parsing `Templates[key]`.
	//
	// [dynamic]
	Templates map[string]json.RawMessage
}

// Schedule defines how to run a pipeline of test.
// A schedule runs on its own and has no side effect with other schedules, if any.
//
// PreProcess will be called before test runs. and then test(s) will be scheduled.
// The decision for gmeter of how to schedule tests depends on:
//  - iterable test: if any iterable command like `list` is defined in anywhere
//    before test actually sending HTTP request to server, the test will be treated
//    as an iterable one, and test will end if any command issues an EOF, disregards
//    of  Count setting.
//
//  - Count: for non-iterable test, defines how many HTTP executions test should run
//
//  - Concurrency: how many routines should be created to run test concurrently for
//    both iterable and non-iterable cases.
type Schedule struct {
	// Name defines name of schedule, and by read ${SCHEDULE} to get.
	Name string

	// PreProcess defines a group of segment which will be composed before tests runs.
	// Note that this preprocessing will be called only once.
	//
	// [dynamic]
	PreProcess []string

	// Tests defined a test pipeline composed of one or more tests.
	// For example: "test1[|test2[|test3...]]", where "test1", "test2", "test3"...
	// are defined in Config.Tests.
	Tests string

	// Reporter defines a template to write test report to a file.
	// Note that Reporter only defines how to write, not when to write. You need call
	// `report` command in Test.Response to actual write something.
	Reporter Report

	// Count defines how many this Tests should run.
	// 0 for infinite, or specified count, default 0.
	// if requests is iterable, this field will be ignored
	Count uint64

	// Concurrency defines how many routines should be created to run Tests.
	// 0 or 1 for one routine, or specified routines, default: 1 routine
	Concurrency int

	// Env defines predefined local environment variables.
	Env map[string]string
}

// RunMode defines gmeter how to run several schedules.
type RunMode string

const (
	RunPipe       RunMode = "Pipe"       // Run Schedule one by one, previous failure will not impact next schedule
	RunConcurrent RunMode = "Concurrent" // Run All Config.Schedules concurrently until all exit
)

// Config defines a gmeter test.
//
// HTTP test can be divided into several parts:
//  - hosts, include host URL and/or proxy
//  - requests, http request method/url/headers/body
//  - request executing parameters, like timeout setting, ...
//  - processing before/after request, these are often used to produce parameters for
//    request and processing response or failure.
//
// So these members are defined:
//  - Hosts: mapped host of http server
//  - Messages: mapped request messages
//  - Tests: combination of host, request message, request parameter, and processing
//
// Tests only gives a series of predefined http execution, and Schedules gives how to
// run these tests. Each schedule in Schedules defines a running test, and they can be
// scheduled in several ways defined by Mode.
//
// In RunPipe mode, schedules will be scheduled one by one, and in RunConcurrent mode
// all schedules are scheduled concurrently. gmeter will be not stopped until all
// schedules stops.
type Config struct {
	Name     string              // Everyone has a name, stored in ${CONFIG}
	Hosts    map[string]*Host    // predefined hosts map that referred by a key string
	Messages map[string]*Request // predefined request map messages that referred by key string
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
