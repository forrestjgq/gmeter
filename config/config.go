// Package config defines a configuration for gmeter to use to start a series HTTP restful test.
//
//     Note: Fields defined in this package with `[dynamic]` comments allows environment
//     variables and commands embedding. Refer to of gmeter command document
//     https://github.com/forrestjgq/gmeter/blob/main/command.md for detail description.
//
//     Note: Fields declared as `interface{}` could be a `string` or `[]string`, which
//     defines a command line or a command group.
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
//
// Read commands document for more tools to generate requests and process response.
//
// A Guideline document is provided to explain how gmeter works:
//		https://github.com/forrestjgq/gmeter/blob/main/guideline.md
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
	PreProcess interface{} // [dynamic] processing before each HTTP request
	// `key` to Config.Hosts, or : [<proxy>|]<host>
	// If Host is empty, gmeter will set it automatically following rules:
	//   - if only 1 hosts exist, set to name of that host
	//   - if more than 1 hosts exist, set to default, anonymous host name "-"
	Host           string
	Request        string    // `key` to Config.Messages
	RequestMessage *Request  // request message definition, preferred over Request
	Response       *Response // Optional entity used to process response or failure
	// HTTP request timeout, like "5s", "1m10s", "30ms"...
	// If Timeout is empty, try use  Schedule.Env["TIMEOUT"] as default value;
	// if it's still empty, it'll be set to "1m" as default value
	Timeout  string
	imported bool
}

func (t *Test) IsImported() bool {
	return t.imported
}

func (t *Test) SetImported() {
	t.imported = true
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
	// If Path already exists, it will be truncated if Append is false.
	//
	// Any necessary parents in path will be created.
	//
	// [dynamic]
	Path string
	// if Append is true, instead of truncating exist file, report content will be appended in
	// that file.
	Append bool

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
	// PreProcess should be a string list or a single string
	//
	// [dynamic]
	PreProcess interface{}
	// PreProcess defines a group of segment which will be composed after tests finishs.
	// Note that this postprocessing will be called only once.
	//
	// PostProcess should be a string list or a single string
	//
	// [dynamic]
	PostProcess interface{}

	// Tests defined a test pipeline composed of one or more tests by quoting name
	// of tests defined in Config.Tests concated by '|'. Specially a '*' indicates
	// any test defined in Config.Tests but not being explicitly defined in pipeline.
	// Please note that '*' could only be defined once at most.
	//
	// If tests are explicitly defined in pipeline like "test1|test2|test3|...", they will
	// be executed in the sequence they are defined. For tests defined by '*', the sequence
	// of executing is not defined.
	//
	// Here are some examples of how to define test pipeline.
	// Assuming we defined t1, t2, t3, t4, t5, t6, t7 in Config.Tests:
	//   1. "t1|t2|t4|t2" will execute t1, t2, t4, t2(again)
	//   2. "t2|t3|*|t5|t2" will execute t2, t3 first, and run t1, t4, t6, t7(defined by *) in random
	//      sequence, at last, t5, t2 will be executed
	//   3. "*" will execute t1 ~ t7 in random sequence once for each
	//   4. "*|t2|t3" will execute t1, t4~t7 in random sequence, and then execute t2, t3
	//   5. "t2|t3|*" will execute t2, t3, and then run t1, t4~t7 in random sequence
	//   6. "" is invalid(no case)
	Tests string

	// TestBase is a special test that behavior like a super class of Tests, this is how
	// it works:
	//
	// for all fields in TestBase and all it's sub fields:
	//    if it is a list, it will be inserted into Test's corresponding field in the head,
	//    otherwise if Test does not define this field, use TestBase's definition.
	//
	// This is used so that while massive cases sharing same test field, and saves developer
	// a lot to edit same content of Test.
	TestBase string

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

	// QPS specifies max request at a single second.
	// Set to a value greater than 1 to enable it.
	QPS int

	// Max executing HTTP request. Effective only if Concurrency greater than 1.
	// Set to a value greater than Concurrency to enable it.
	Parallel int

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
	Name string // Everyone has a name, stored in ${CONFIG}

	// Imports defines configuration file(s) to be loaded as template. It could be a single
	// string, or a string list([]string), each of which is a absolute or relative file path.
	// While relative path is used, it is relate to this config file.
	//
	// All the Hosts/Messages/Tests/Env/Options defined inside those template configurations
	// will be copied to this config except the same key already defined.
	//
	// If global template is specified by `-template <path>`, template will be imported before
	// this.
	Imports interface{}

	// Functions defines several functions, each one is stored inside a map, with a function name
	// as map key.
	// A function is a command line(a string) or a command group(a string list: []string) defined
	// by config and called by command `call`.
	// Command(s) inside function could visit arguments by $n. $0 is always the name of function,
	// and $1 is the first argument, $2 is the second argument, ...
	// Command `call` will pass function name and all required arguments, for example: `call add 3 5` will
	// execute function `add` with argument 1($1) is `3` and argument 2($2) is `5`.
	Functions map[string]interface{}

	// predefined hosts map that referred by a key string.
	// if key is "-", this host is applied to those Tests defined without an explicit Test.Host.
	Hosts map[string]*Host

	Messages map[string]*Request // predefined request map messages that referred by key string
	// predefined tests. key will be test name, and value will be test definition.
	// NOTE that there are some special names:
	//  - "^": executed before any schedule for just once, often used as global initialize
	//  - "$": executed after all schedules for just once, often used as global cleanup
	//  - "<": executed in each schedule as the first test for one time even schedule loops
	//  - ">": executed in each schedule as the last test for one time even schedule loops
	Tests map[string]*Test

	Mode      RunMode     // how to run schedules, default RunPipe
	Schedules []*Schedule // all test schedules, each one runs a series of tests

	Env     map[string]string // Env defines predefined global environment variables.
	Options map[Option]string // options globally
}
