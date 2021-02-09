# Overview
gmeter is a tool to make user easily creates a RESTful client and server. It provides programmable, embedded json processing through integrated environment variables and commands, which make gmeter so powerful.

This document will guide you into gmeter, introduce the basic component of configuration, and show you several advanced usage of gmeter by examples. But it only shows you how to make a gmeter works, it does not cover every corner of gmeter.

# Commands and Variables

Variable environment is a map from string to string, of which key is the variable name, and value is variable value. gmeter treat everything as string. `$(VAR)` is used to access local variables, and `${VAR}` is used to access global variables. The difference of these two kinds of variable is that, global variable is defined by gmeter and can not be written, and local variable, which is defined by gmeter and user, can be read and written by command `env`.

```
# write "Forrest" to variable WHO, if variable not exists, it will be created
`env -w WHO Forrest`

# Read variable WHO and print, here $$ is output of command "env -r"
`env -r WHO | print $$`
or
`print $(WHO)`

# Delete variable WHO
`env -d WHO`
```

Variables has life cycle. They only survives in the range they are defined. This makes some components are called just like a function, and variables are effective only inside it.

The power of gmeter comes from programmable commands and variable environment. It behaves as a script language while user writes a configuration of how gmeter works. Json embedded commands and variable accessing makes json composing more easy, flexable, and readable. The best thing is, combining internally defined variables, such as `$(RESPONSE)` for HTTP client response body, gmeter provides lots of useful command, acting like library, to process json such as comparing, extracting, file accessing, printing...

A command is a string. when it's embedded insided json, it is surrounded by back quotes, like: "\`echo hello world\`" will create a string "hello world" and output, and "\`cat a.list\`" will read content of file `a.list` and output.

Just like shell command, a command is consist of a command name, and 0 or more arguments. Each command has its own argument definition and behavior. 

Command can be pipelined, just like shell. Concated commands by `|` are executed one by one by sequence they are defined. Environment variable `$(INPUT)` and `$(OUTPUT)` behaves like `stdin` and `stdout` to make previous command output flows to next command as input automatically. A group of commands or command pipelines, reprsenting in json as an array, enables a block of processing like function, and each command or pipeline is a statement. Command, command pipeline, and command group makes complicated processing possible and readable inside json. It shows in json like:
```json
"something": [
    "`command1`"
    "`command2`"
    "`...`"
]
```

Because gmeter use commands only embedding in json, so we here give some examples using json representation:

```json
{
    "Path": "/state?var=`env -r NAME`"
}
```
Here we defined a json that takes an `Path` for HTTP routes definition. An command `env -r NAME` is embedded to read local variable `$(NAME)` and replace the command with its output. If `$(NAME)` is `orange`, we'll get a path `/state?var=orange`. How result of commands producing are used depends on the scenario it applies. If gmeter is used as an HTTP client, and it will load a json config file containing a `Path` field to compose URL of request. In this scenario, gmeter will **call** "/state?var=\`env -r NAME\`" to produce a string using as route path.

But things is not always works like that. For example, as HTTP client gmeter will store status code to variable `$(STATUS)` and response body to `$(RESPONSE)` and call something to check response. This action is defined in a json string list and it could be:
```json
"Check": [
    "`assert $(STATUS) == 200`",
    "check result: `json .Result | assert $$ > 0`",
    "something is not a command, and do nothing like a comment"
]
```
Here the first checker ensures status code of request is 200, or it will report an error and abort execution. It does not produce anything, and gmeter does not care because its requirement focus on one thing only: is there any error?

So the second checker embeds pipelined commands inside a string, and the command will be called with a prefixed hint `check result: `, but the string produced by this checker is ignored. We usually use this feature to comment those only care about the error state, and ignore the string it produces.

The third check is a pure string without any command. While gmeter calls it, there is no callable part inside, so there is no error produced. And gmeter will just ignores it. This is offen used as comment for the whole block of command group.

gmeter defines how it uses commands for in config file definitions.

**Be familiar with predefined commands is very important**. Refer to [Command](./command.md) for more detailed command usage and definitions.

Another powerful tool is [jsonc](./jsonc.md), it allows user creating a json template which has same json structure with target json data, and process matched fields on target json data one by one, making complex json processing like a vacation in Hawaii. It is used in HTTP client response and server request processing.

# RESTful HTTP client

A RESTful HTTP client normally requires these elements:
1. URL
2. Method
3. Optional headers
4. Optional request body
5. Response process

URL is composed by:
1. server address, like http://127.0.0.1:10086
2. route path, like /lib/query
3. optional request parameters, like `isbn=9787020144532`

In gmeter, we split URL into two parts: Host(server address) and Path(route path + optional request parameters).
HTTP method actually has lots of definitions, but usually they are GET, PUT, POST, DELETE, PATCH.
Headers are a string to string map enabling client to take extra parameters to server like `"content-type": "application/json"`.
Request body is the message request takes, when it is present, usually you need tell server its content type by `content-type` header. In gmeter only json as request body is supported.

While these information are given, HTTP request can be composed based on a configuration json by gmeter and sent to server. When server responds a status code and an optional response body, gmeter will write these informations to local variables and then call commands and/or json template to process.


## Schedule and Test
gmeter HTTP client has the capability to run a single HTTP request or a series requests for one or multiple times sequentially or concurrently, depending on how user define these requests. The basic HTTP request in gmeter is called **Test**. It contains the definition of how request message is composed, and how the response is processed. Multiple tests can be pipelined so that they can be executed one by one. For example, you may create a data table first, then write a data item, and query to check, at last delete it. In this case, you may define 4 tests and pipeline them. A single test can be a special case of a test pipeline that has one test inside. Pipelined tests in gmeter is called **Schedule**. Schedule defines not only the pipeline itself, but also the count pipeline should run, and concurrency setting.

The definition of Test in gmeter configuration is seperated from Schedule. Each Test has a unique string name, and multiple Tests are organized as a map from string to Test. gmeter will refer to them by quoting their name(s). This kind of definition enables multiplex use of Tests. A test defines a **Host**, a **Request**, and a **Response** as basic components. A Request contains everything an HTTP request requires except Host definition. Combining Host and Request, an HTTP request can be composed by gmeter. Response contains several command processing entity to let user define how to process HTTP response.

///Test(s) could be run once, multiple times, or infinitely depends on schedule `Count` setting. But if there is any `iterable` command in request composing in any test, `Count` is ignored, and test will stop while that command ends. An example of iterable command is `list`, which reads text file for one line at a time until reaching end of file.

///Test(s) could be scheduled in a single thread, or in multiple threads concurrently. When more than one threads is defined, each one has an independant variable environment. If tests define `iterable` command, after that reachs ends, all threads stops. Otherwise the total count of test running will not more than defined `Count`.

///If `AbortIfFail` option is defined, test(s) stops if any error happends. In concurrent test shceduling, once any thread reports an error, all other threads will stops after they finish their current running test.

As we know, commands used in HTTP request composing and response processing requires a variable environment. To make sure previous pipeline call won't make impact to next one, gmeter will cleanup all user local variables before a pipeline call. While Schedule calls pipeline in multiple threads, each one maintains its own local variables. This makes pipeline acting like a reentrant function, and local variables are those variables locally defined inside function and will be invalid outside. 
Besides global and local variables, there is another data container called database. Every Schedule maintains its own database and it won't be cleared across pipeline call, which means for a pipeline, if it write some data to database, it won't be cleared at next running. Database call is thread safe and can be accessed by command `db`, see [Database accessing](command.md#db---database-accessing) for details.

## Configuration structure

The gmeter execute HTTP request through input configuration file(s).  Each configuration file contains a single json object, which defines one or more gmeter schedule(s).

This chapter will discuss how to write a configuration for gmeter HTTP RESTful client.

### Go data type and json
Before discussing about gmeter configuration, We need take a few time to show you how we define json structure. json itself does not define a clear schema to define a specific structure. So we use Golang to describe it.

Json defines these data types:
- basic types: number, boolean, string
- object
- array of another data type

In Golang basic types can be defined as:

| json    | Golang                                         |
| --      | --                                             |
| number  | int/uint/intXX/uintXX/byte/float32/float64/... |
| boolean | bool                                           |
| string  | string                                         |

An example of basic type of Golang is:
```go
type BasicType struct {
    BoolVar bool
    IntVar int
    StringVar string
}
```
Note that the variable name is defined before its type.

An instance of `BasicType` can be marshaled to:
```json
{
    "BoolVar": false,
    "IntVar": 1,
    "StringVar": "hello world"
}
```

For json object, Golang has two different kinds of definitions:
1. if all fields of json object share the same structure definition, it could be defined as a map with key of string
2. otherwise it is defined as a struct

The above example shows a struct of `BasicType` which contains 3 different member data types, so it is defined as structure, but for this kind of json:
```json
{
    "v1": {
        "BoolVar": false,
        "IntVar": 1,
        "StringVar": "hello world"
    },
    "v2": {
        "BoolVar": false,
        "IntVar": 1,
        "StringVar": "hello world"
    },
    "v3": {
        "BoolVar": false,
        "IntVar": 1,
        "StringVar": "hello world"
    }
}
```

All members share the same structure of `BasicType`, so it could be defined as map to `BasicType` in Golang:
```go
map[string]*BasicType
```

A json arry is a json contains several elements share same data structure, in Golang it is defined as slice: `[]DataType`, for example, a command group can be defined as a string json array, that is in Golang: `[]string`. And when a json array contains several json object, it is defined as slice of struct in Golang, for example: `[]BasicType`.

A special data type defined inside gmeter configuration is `json.RawMessage`, it indicates this field is an json message without any definition of structure. In other words, any valid json is valid, for example, HTTP request message is defined as:
```go
type Request struct {
    Method string
    Path string
    Headers map[string]string
    Body json.RawMessage
}
```
Here `Body` is request body and different request contains different structure of request body, so it can not be explicitly defined with a structure. So it could be any json. For example, assuming its body is a `BasicType`, the json:
```json
{
    "Method": "GET",
    "Path": "/var",
    "Headers": {},
    "Body": {
        "BoolVar": false,
        "IntVar": 1,
        "StringVar": "hello world"
    }
}
```
is a valid `Request`.

Actually in gmeter `json.RawMessage` is treated as a string, thanks to GO json package. The reason we use `json.RawMessage` is to avoid defining an entire json inside a string. For example, if you need to define this request body as a `string`, you may need write json like:
```json
{
    "Method": "GET",
    "Path": "/var",
    "Headers": {},
    "Body": "{ \"BoolVar\": false, \"IntVar\": 1, \"StringVar\": \"hello world\" }"
}
```
## Test definition
Let's start with definition of a Test:
```go
type Test struct {
	PreProcess []string // [dynamic] processing before each HTTP request

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
	Timeout string
}

```

`Host` can be a server http address like `http://127.0.0.1:8008`, or a name(which will be talked in later chapter). `Request` is a name of predefined common Request and we'll also take about it later. `Timeout` is the time duration specified for HTTP request timeout. 

`PreProcess` is a command group that is specified by user and will be called before gmeter composes HTTP request. Normally it is used to generate variables to be used in Request composing, or print a message, depending on user's requirement.

this is how gmeter execute a Test:
1. call `PreProcess` 
2. Compose HTTP request(by `Host` and `Request`/`RequestMessage`)
3. Send request to HTTP server and wait at most `Timeout`
4. If server responds, write status code to `$(STATUS)` and response to `$(RESPONSE)`
5. If timeout, or any other error occurs, set `$(FAILURE)` with error message
6. Call `Response`

Here let's focus on `RequestMessage`, `Response` first.

### Request
`Request` message is easy one:
```go
type Request struct {
	Method  string            // default to be GET, could be GET/POST/PUT/DELETE
	Path    string            // [dynamic] /path/to/target, parameter is supported like /path?param1=1&param2=hello...
	Headers map[string]string // extra headers like "Content-Type: application/json"
	Body    json.RawMessage   // [dynamic] Json body to send, or "" if no body is required.
}
```
Note that fields with comment `[dynamic]` indicate that this field could be command embedded field. For example:
```json
{
    "Method": "POST",
    "Path": "/book/$(ISBN)",
    "Headers": {
        "content-type": "application/json"
    },
    "Body": {
        "author": "$(AUTHOR)",
        "publisher": "$(PUBLISHER)",
        "price": "$(PRICE)"
    }
}
```
Here we define a POST message, the route path contains an ISBN, request body contains author, publisher and date information, and we know that `Path` and `Body` are `[dynamic]` fields, so when this Request is composed by gmeter, it requires variables `ISBN`, `AUTHOR`, `PUBLISHER`, and `PRICE`. Those variables must be defined before composing, otherwise they are treated as empty strings. In most cases, we prepare variables request message requires before composing, but sometime we need call a command to process. Assuming we need a body like this:
```json
{
    "author": "forrest jiang",
    "publisher": "deepglint",
    "price": 122.4
}
```
and now `$(PRICE)` is `"122.4"`. Please note that **All variable values are string**, and after request composing we'll get:
```json
{
    "author": "forrest jiang",
    "publisher": "deepglint",
    "price": "122.4"
}
```
In this json, `price` is a string, and it's unacceptable. Now we need to convert `"122.4"` to `122.4`, using `cvt` command:
```json
{
    "Method": "POST",
    "Path": "/book/$(ISBN)",
    "Headers": {
        "content-type": "application/json"
    },
    "Body": {
        "author": "$(AUTHOR)",
        "publisher": "$(PUBLISHER)",
        "price": "`cvt -f $(PRICE)`"
    }
}
```
see [cvt command](command.md#cvt---strip-quotes-and-convert-string-to-specified-type) for more.

This is how composing works:
1. split string, like `Body` string, by `$(...)` or "\`\`", into several segments of substrings
2. for variable reading `$(...)`, gmeter reads from variable environment, get a value string and replace it.
3. for command, gmeter will parse and execute it to get an output string and replace it. `cvt` is a trick one, it not only replace current segment, but also 'eat' the quotes surrounding it.

### Response
Response contains several command groups and a template for response processing.

After HTTP responds, gmeter will write status code to `$(STATUS)` and response body to `$(RESPONSE)`, and call `Template` first, then `Check`. Any error occurs in these two will abort the execution, write the fail reason to `$(FAILURE)` and redirect to `Failure`, otherwise `Success` will be called.

If HTTP timeout, `Failure` will be called directly.

None of these fields is necessary.

```go
type Response struct {
	Check    []string        // [dynamic] segments called after server responds.
	Success  []string        // [dynamic] segments called if error is reported during http request and Check
	Failure  []string        // [dynamic] segments called if any error occurs.
	Template json.RawMessage // [dynamic] Template is a json compare template to compare with response.
}

```

For example, we want to make sure server responds with `200`, or we'll print a message:
```json
{
    "Check": [
        "`assert $(STATUS) == 200`"
    ],
    "Failure": [
        "`print error: $(FAILURE)`"
    ]
}
```

tips:
- `assert` will evaluate logical expression and will report an error if its evalution result is false
- `print` behaves like shell echo command, it will print string to stdout.
- none of these two commands generates its own output string, it will pass its $(INPUT) to $(OUTPUT) directly in command pipeline 
### A full definition of Test
We assemble the samples above to get a Test json:
```json
{
    "Host": "http://127.0.0.1:8008",
    "RequestMessage": {
        "Method": "POST",
        "Path": "/book/$(ISBN)",
        "Headers": {
            "content-type": "application/json"
        },
        "Body": {
            "author": "$(AUTHOR)",
            "publisher": "$(PUBLISHER)",
            "price": "`cvt -f $(PRICE)`"
        }
    },
    "Response": {
        "Check": [
            "`assert $(STATUS) == 200`"
        ],
        "Failure": [
            "`print error: $(FAILURE)`"
        ]
    }
}
```

Now we assume that this Test has a name of `new-book` for discussion of Schedule, and later I'll show you where to define Test and how to name it.

For discussion in Schedule, we gives another Test `get-book` to read a book:
```json
{
    "Host": "http://127.0.0.1:8008",
    "RequestMessage": {
        "Method": "GET",
        "Path": "/book/$(ISBN)"
    },
    "Response": {
        "Check": [
            "`assert $(STATUS) == 200`",
            "`assert $(@json .price $(RESPONSE)) == $(PRICE)`"
        ],
        "Success": [
            "`json .author $(RESPONSE) | print ISBN: $(ISBN), author: $$`"
        ]
    }
}
```
tips:
- `Check` has multiple commands
- `assert $(@json .price $(RESPONSE)) == $(PRICE)` will read price from response and `assert` makes sure it's correct. `$(@command args...)` is a command embedded to another, and its output will used as argument of outer command. It has the same effect from "`json .price $(RESPONSE) | assert $$ == $(PRICE)`", but more readable.
- `json .author $(RESPONSE)` will read from response body, which is a json object containing an `author` field, to get author name and output
- `print ISBN: $(ISBN), author: $$` prints ISBN and author name, here `$$` indicates output of command before pipeline `|`.

## Schedule

Schedule defines what and how Test(s) runs. In this chapter, We'll talk about making a simple schedule that runs two HTTP request for one time.

Here is the definition of Schedule, and currently let's focus on `Tests`, `Count` and `Env`:
```go
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

	// Env defines predefined local environment variables.
	Env map[string]string
}

```
Test(s) that will be running is defined by `Tests`. It's a string that contains one or multiple Test names seperated by `|`. Here we want to write a book to server and read it from server, so Tests will be `"new-book|get-book"`.

As we know, `new-book` Test requires these variables:
- ISBN
- AUTHOR
- PUBLISHER
- PRICE

If any of these variables are not defined, request will fail. So we use `Env` to define these variables:
```json
{
    "Env": {
        "ISBN": "123456789",
        "AUTHOR": "Forrest Jiang",
        "PUBLISHER": "DeepGlint",
        "PRICE": "122.4"
    }
}
```

`Count` decides how many time these Tests should be run. Here we run once. So a Schedule is defined as:
```json
{
    "Name": "book",
    "Tests": "new-book|get-book",
    "Count": 1,
    "Env": {
        "ISBN": "123456789",
        "AUTHOR": "Forrest Jiang",
        "PUBLISHER": "DeepGlint",
        "PRICE": "122.4"
    }
}
```

Now we finish the schedule definition. But there are more. gmeter accept `Config` as configuration, so we need wrap our schedule in it.
## Config
Now only two fields need your concern:  `Mode` ,  `Options` , `Tests`, and `Schedules`.

```go
type Config struct {
	Name string // Everyone has a name, stored in ${CONFIG}

	// Imports defines configuration files to be loaded as template.
	// All the Hosts/Messages/Tests/Env/Options defined inside those template configurations
	// will be copied to this config except the same key already defined.
	//
	// If global template is specified by `-template <path>`, template will be imported before
	// this.
	Imports []string

	// predefined hosts map that referred by a key string.
	// if key is "-", this host is applied to those Tests defined without an explicit Test.Host.
	Hosts map[string]*Host

	Messages map[string]*Request // predefined request map messages that referred by key string
	Tests    map[string]*Test    // predefined tests

	Mode      RunMode     // how to run schedules, default RunPipe
	Schedules []*Schedule // all test schedules, each one runs a series of tests

	Env     map[string]string // Env defines predefined global environment variables.
	Options map[Option]string // options globally
}

```

A config could define multiple Schedules, and `Mode` defines how gmeter will run them:
- "Pipe": default value, Schedules will be run one by one with the same sequence they are defined
- "Concurrent": Each Schedule will get a thread to run and gmeter will run them all at same time concurrently.

`Options` defines some options that guide gmeter to make decision, now only `"AbortIfFail"` is configurable, once it is set to `"true"`, any error happened in any test will abort the whole test, otherwise only the error will be ignored.

`Tests` defines several `Test` each has a name as key in map, and Schedule will refer to this name to define its Tests.

Now let's fill previous defined Tests and Schedule in `Config`:
```json
{
    "Name": "Library",
    "Tests": {
        "new-book": {
            "Host": "http://127.0.0.1:8008",
            "RequestMessage": {
                "Method": "POST",
                "Path": "/book/$(ISBN)",
                "Headers": {
                    "content-type": "application/json"
                },
                "Body": {
                    "author": "$(AUTHOR)",
                    "publisher": "$(PUBLISHER)",
                    "price": "`cvt -f $(PRICE)`"
                }
            },
            "Response": {
                "Check": [
                    "`assert $(STATUS) == 200`"
                ],
                "Failure": [
                    "`print error: $(FAILURE)`"
                ]
            }
        },
        "get-book": {
            "Host": "http://127.0.0.1:8008",
            "RequestMessage": {
                "Method": "GET",
                "Path": "/book/$(ISBN)"
            },
            "Response": {
                "Check": [
                    "`assert $(STATUS) == 200`",
                    "`assert $(@json .price $(RESPONSE)) == $(PRICE)`"
                ],
                "Success": [
                    "`json .author $(RESPONSE) | print ISBN: $(ISBN), author: $$`"
                ]
            }
        }
    },
    "Mode": "Pipe",
    "Schedules": [
        {
            "Name": "book",
            "Tests": "new-book|get-book",
            "Count": 1,
            "Env": {
                "ISBN": "123456789",
                "AUTHOR": "Forrest Jiang",
                "PUBLISHER": "DeepGlint",
                "PRICE": "122.4"
            }
        }
    ],
    "Options": {
        "AbortIfFail": "true"
    }
}
```

### Define hosts
You may notice that tests defined in a config file are almost all same, and they are defined repeatly. Actually the IP and port in reality often changes. Once that happens, you may need to modify them one by one.

There is another solution.

`Config` defines a `Hosts` allowing you predefine some hosts:
```go
type Host struct {
	// format: http://domain[:port][/more[/more...]], https is not supported yet.
	Host string
	// Proxy defines a proxy used to access Host.
	// format: <protocol>://[user:password@]domain[:port], protocol could be http or socks5
	Proxy string
}

type Config struct {
	// predefined hosts map that referred by a key string.
	// if key is "-", this host is applied to those Tests defined without an explicit Test.Host.
	Hosts map[string]*Host
}

```

Each `Host` is defined with a name as key of `Hosts` map, and you need only refer to them by this name:

```json
{
    "Name": "Library",
    "Hosts": {
        "library": {
            "Host": "http://127.0.0.1:8008"
        }
    },
    "Tests": {
        "new-book": {
            "Host": "library",
            "RequestMessage": { },
            "Response": { }
        },
        "get-book": {
            "Host": "library",
            "RequestMessage": { },
            "Response": { }
        }
    },
    "Mode": "Pipe",
    "Schedules": [ ],
    "Options": { }
}
```

If you are sure there is only one host is defined and never more, you can even ignore it in Test definition:
```json
{
    "Name": "Library",
    "Hosts": {
        "library": {
            "Host": "http://127.0.0.1:8008"
        }
    },
    "Tests": {
        "new-book": {
            "RequestMessage": { },
            "Response": { }
        },
        "get-book": {
            "RequestMessage": { },
            "Response": { }
        }
    },
    "Mode": "Pipe",
    "Schedules": [ ],
    "Options": { }
}
```

And it's not finished. 

If you use gmeter in automatically testing, the `Host` may be dynamic and is not allowed to be modified manually. Then you may use gmeter command line `-e "key=value key=value"` option to setup global variables and use them in `Host` for IP and port, like:
```json
{
    "Name": "Library",
    "Hosts": {
        "library": {
            "Host": "http://${IP}:${PORT}"
        }
    },
    "Tests": { },
    "Mode": "Pipe",
    "Schedules": [ ],
    "Options": { }
}
```
and gmeter command line:
```sh
gmeter -e="IP=127.0.0.1 PORT=8008" config.json
```
will setup `Host` for you.

### Reuse request messages
Assuming this requirement:
1. send a request to add a book into library, and make sure it successes
2. do that again, and make sure it fails with a status code not 200

You may define your config like this:
```json
{
    "Name": "Library",
    "Hosts": {
        "library": { "Host": "http://${IP}:${PORT}" }
    },
    "Tests": {
        "new-book": {
            "RequestMessage": {
                "Method": "POST",
                "Path": "/book/$(ISBN)",
                "Headers": {
                    "content-type": "application/json"
                },
                "Body": {
                    "author": "$(AUTHOR)",
                    "publisher": "$(PUBLISHER)",
                    "price": "`cvt -f $(PRICE)`"
                }
            },
            "Response": {
                "Check": [
                    "`assert $(STATUS) == 200`"
                ],
                "Failure": [
                    "`print error: $(FAILURE)`"
                ]
            }
        },
        "dup-new-book": {
            "RequestMessage": {
                "Method": "POST",
                "Path": "/book/$(ISBN)",
                "Headers": {
                    "content-type": "application/json"
                },
                "Body": {
                    "author": "$(AUTHOR)",
                    "publisher": "$(PUBLISHER)",
                    "price": "`cvt -f $(PRICE)`"
                }
            },
            "Response": {
                "Check": [
                    "`assert $(STATUS) != 200`"
                ]
            }
        }
    },
    "Mode": "Pipe",
    "Schedules": [ ],
    "Options": {
    }
}
```

The biggest issue here is that `RequestMessage` are written twice with completely same content. gmeter refuse duplication! That is why a `Messages` are defined in `Config`. Like `Hosts`, it defines some `Request` indexed by a string name for `Test` to refer to by its `Request` field:
```json
{
    "Name": "Library",
    "Hosts": {
        "library": { "Host": "http://${IP}:${PORT}" }
    },
    "Messages": {
        "new-book-req": {
            "Method": "POST",
            "Path": "/book/$(ISBN)",
            "Headers": {
                "content-type": "application/json"
            },
            "Body": {
                "author": "$(AUTHOR)",
                "publisher": "$(PUBLISHER)",
                "price": "`cvt -f $(PRICE)`"
            }
        },
    },
    "Tests": {
        "new-book": {
            "Request": "new-book-req",
            "Response": {
                "Check": [
                    "`assert $(STATUS) == 200`"
                ],
                "Failure": [
                    "`print error: $(FAILURE)`"
                ]
            }
        },
        "dup-new-book": {
            "Request": "new-book-req",
            "Response": {
                "Check": [
                    "`assert $(STATUS) != 200`"
                ]
            }
        }
    },
    "Mode": "Pipe",
    "Schedules": [ ],
    "Options": {
    }
}
```
Please note that in Test definition, `RequestMessage` disappears and a `Request` is used to refer to predefined `Messages`.

## Advanced topics
### Concurrent running
All we discussed above are sequential execution of HTTP requests. But we need concurrent call sometimes. Usually concurrent HTTP requests can be applied in these scenarios:
1. Simulate large scale HTTP visiting
2. Enhance the performance of large number of HTTP requests.
3. Server function for concurrent processing.

All these require gmeter starts multiple requests in parallel. And gmeter supports it.

The concurrency support in gmeter is appicable in two levels:
1. Schedules. Multiple schedules can be defined inside one single config, and [Config.Mode](https://pkg.go.dev/github.com/forrestjgq/gmeter/config#RunMode) decides how gmeter run these schedules. If it is defined as `Concurrent`, all schedules will be started concurrently until all them succeeds or any of them fails(while `AbortIfFail` option is `true`). Each of these schedules runs complete seperately just like they are run in `Pipe` mode.
2. Tests inside a single Schedule. While a schedule defines a `Concurrency` larger than 1, multiple threads are started to execute Tests concurrently. Test in each thread runs sequentially. When a running ends, it will request next run to Schedule, and continue if Schedule agrees, or exit if Schedule give an indication of test ending.

Concurrent tests running only make sence while Schedule defines multiple running. 
There are 2 kind of definition to decide how many times tests should be executed.

The first one is simple. Set a number larger than 1 in `Schedule.Count` for a specified number of execution, or set to 0 for infinite running. Schedule will record the number of total requests from all threads. While it reaches `Schedule.Count`, all requests for next run from any thread will get an end of testing and exit.

The second one comes from what we call as `iterable` commands. gmeter defines many commands. Some of them are defined as `iterable`. This kind of command will generate an error of EOF while it reaches an end. Schedule will check the procedure of request composing(test preprocess, request message composing), and if any iterable command is used inside it, Schedule is iterable, and**`Schedule.Count` is ignored**. While Schedule got an EOF error in request composing, Schedule will give indication to all threads at request for next running, and they'll exit.

Next chapter will introduce iterable commands usage.

### Iterable commands
#### list command
`list` command reads one non-empty line from a file on every call , and output that line. While it reaches ends of file, it will generate an EOF error.

`list` command is very useful while Schedule defines a template of request, and requires variables to compose a full request. By reading from list to create those variables different request could be generated. This enables the capability of massive requesting with different parameters. For example, you need to add 1 million new books into library server. In previous chapters we show you how to define a request and use defined variables to generate a message to send. You can not write 1 million that config or modify that config file once for all. All you need is let gmeter reads parameters from list and write to variables for you, then execute them. It is the situation `list` is good at.

First we create a template config that requires several variables to run:

```json
{
    "Name": "Library",
    "Hosts": {
        "library": { "Host": "http://${IP}:${PORT}" }
    },
    "Tests": {
        "new-book": {
            "RequestMessage": {
                "Method": "POST", "Path": "/book/$(ISBN)",
                "Headers": {
                    "content-type": "application/json"
                },
                "Body": {
                    "author": "$(AUTHOR)",
                    "publisher": "$(PUBLISHER)",
                    "price": "`cvt -f $(PRICE)`"
                }
            },
            "Response": {
                "Check": [
                    "`assert $(STATUS) == 200`"
                ],
                "Failure": [
                    "`print error: $(FAILURE)`"
                ]
            }
        }
    },
    "Mode": "Pipe",
    "Schedules": [
        {
            "Name": "book",
            "Tests": "new-book"
        }
    ],
    "Options": {
        "AbortIfFail": "true"
    }
}
```

This template config requires four variables: ISBN, AUTHOR, PUBLISHER, PRICE. We need define them in a list file as json and each line contains a full json:
```
{"isbn": "123456789", "author": "Forrest Jiang", "publisher": "DeepGlint", "price": 122.4}
{"isbn": "123456788", "author": "Sheldon Cooper", "publisher": "Big Bang", "price": 77.99}
...
```
and we need a `list` command to read each line and treat it as json to write to variables. This should be done in the beginning of request message composing. So we put it in `PreProcess` of Test:
```json

{
    "Name": "Library",
    "Hosts": { },
    "Tests": {
        "new-book": {
            "PreProcess": [
                "step 1, read a line and write to variable: `list ${LIST} | env -w $(JSON)`",
                "step 2, extract from $(JSON) to write variables"
                "`json .isbn $(JSON) | env -w ISBN`"
                "`json .author $(JSON) | env -w AUTHOR`"
                "`json .publisher $(JSON) | env -w PUBLISHER`"
                "`json .price $(JSON) | env -w PRICE`"
            ]
        }
    },
    "Schedules": [ ],
    "Options": { }
}
```

tips:
1. again, you may notice that `PreProcess` cares about execution procedure, not the results it produces. So a string could be a comment or a comment could be embedded with a command.
2. `list ${LIST}` expects a list file path from global variables, which could be specified in command by by argument `-e`.

1 million is a great number and we expect it to be concurrent, so give `Schedule.Concurrency` a number of 100. The full config is:
```json
{
    "Name": "Library",
    "Hosts": {
        "library": { "Host": "http://${IP}:${PORT}" }
    },
    "Tests": {
        "new-book": {
            "PreProcess": [
                "step 1, read a line and write to variable: `list ${LIST} | env -w $(JSON)`",
                "step 2, extract from $(JSON) to write variables"
                "`json .isbn $(JSON) | env -w ISBN`"
                "`json .author $(JSON) | env -w AUTHOR`"
                "`json .publisher $(JSON) | env -w PUBLISHER`"
                "`json .price $(JSON) | env -w PRICE`"
            ],
            "RequestMessage": {
                "Method": "POST", "Path": "/book/$(ISBN)",
                "Headers": {
                    "content-type": "application/json"
                },
                "Body": {
                    "author": "$(AUTHOR)",
                    "publisher": "$(PUBLISHER)",
                    "price": "`cvt -f $(PRICE)`"
                }
            },
            "Response": {
                "Check": [ "`assert $(STATUS) == 200`" ],
                "Failure": [ "`print error: $(FAILURE)`" ]
            },
            "Timeout": "10s"
        }
    },
    "Mode": "Pipe",
    "Schedules": [
        {
            "Name": "book",
            "Tests": "new-book",
            "Concurrency": 100
        }
    ],
    "Options": {
        "AbortIfFail": "true"
    }
}
```

##### Automatic map from a list to variable
The previous demo of `list` requires manual extracting from json to write to variables. It's not good enough. A better usage of json is `json -m`, which maps a json to variable directly:
```json
[
    "read a line of json and map to variable: `list ${LIST} | json -m`",
]
```

We just need make sure that the name of json object is exactly the same as the variable we need in config:
```
{"ISBN": "123456789", "AUTHOR": "Forrest Jiang", "PUBLISHER": "DeepGlint", "PRICE": 122.4}
{"ISBN": "123456788", "AUTHOR": "Sheldon Cooper", "PUBLISHER": "Big Bang", "PRICE": 77.99}
```

see [json command](./command.md#json---json-query) for more description for `json -m`

#### until command
Assume you have this requirement:

gmeter is deployed in a CI environment and it starts an HTTP server then start testing. You need to make sure test does not start until server has been started. Fortunately server provide a ping HTTP route to pong back to caller to finish a handshake. So you may need send ping repeatly until it pongs, after which other test can be started.

How this can be done? `until` command is used for this kind of situation.

`until` will generate an EOF while its condition satisfies. see [until command](./command.md#until---do-test-until-condition-satisfied). We may create a test to send ping until it pongs and then, for example, start `new-book`:

```json
{
    "Name": "Library",
    "Hosts": {
        "library": { "Host": "http://${IP}:${PORT}" }
    },
    "Tests": {
        "ping": {
            "PreProcess": [ "`until $(@db -r PONG) == 1 | print Waiting for server ready...`" ],
            "RequestMessage": { "Path": "/vse/ping" },
            "Response": {
                "Check": [ 
                    "`assert $(STATUS) == 200`" 
                ],
                "Success": [ "while server responds, write database: `db -w PONG 1`" ],
                "Failure": [ "wait for a while if server is not ready: `sleep 5s`" ]
            },
            "Timeout": "3s"
        },
        "new-book": { }
    },
    "Mode": "Pipe",
    "Schedules": [
        {
            "Name": "ping",
            "PreProcess": [ "`db -w PONG 0`" ],
            "Tests": "ping"
        },
        {
            "Name": "book",
            "Tests": "new-book"
        }
    ],
    "Options": { "AbortIfFail": "true" }
}
```

tips:
1. Schedule `ping` defines a `PreProcess` to write database item `PONG` with 0 as initial value
2. `until $(@db -r PONG) == 1` will read from database item `PONG` to check if server has been responded. If `PONG` has a value 1, which is written on response `Success` processing, `until` generates an EOF to stop `ping` execution, and next Schedule `book` will be called.

### Report
You may need write to file to save HTTP result. gmeter provide an internal reporter for user to write content with customized output formation to specified file.

Reporter is created and used by a single [Schedule](https://pkg.go.dev/github.com/forrestjgq/gmeter/config#Schedule) as its `Reporter`, and all Tests it defines could visit it by command [report](./command.md#report---write-string-to-report-file).

`report` command could compose a formation string , or to compose a template of json string, to get an output string and send to Schedule to write to file. Here is the `Report` definition:

```go
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

```
A formation string could be embedded with commands or variable accessing. It could be specified by `report -f`, or use default formation `Report.Format`. For example:
```json
{
    "Tests": {
        "rpt": {
            "RequestMessage": {},
            "Response": {
                "Success": [ "here we report: `report -n -f {\"status\": $(STATUS)}`" ],
            }
        }
    },
    "Schedules": [
        {
            "Name": "rpt-demo",
            "Tests": "rpt",
            "Reporter": {
                "Path": "./status.log",
            }
        }
    ]
}
```
will write a json that takes HTTP response status code to whichever file `Report` specifies.

And this will do the same work:
```json
{
    "Tests": {
        "rpt": {
            "RequestMessage": {},
            "Response": {
                "Success": [ "here we report: `report -n`" ],
            }
        }
    },
    "Schedules": [
        {
            "Name": "rpt-demo",
            "Tests": "rpt",
            "Reporter": {
                "Path": "./status.log",
                "Format": "{\"status\": $(STATUS)}"
            }
        }
    ]
}
```
If test is executed for 3 times with status code 200, 400, 304, it will write:
```
{"status": 200}
{"status": 400}
{"status": 304}
```
to file `status.log`.

Format string is a line of string, which makes complex composing difficult, even impossible. So `Report` defines a `Templates` which allowes you defining several json templates with embedded variable accessing and commands to generate any readable json. For example:
```json
{
    "Tests": {
        "rpt": {
            "RequestMessage": {},
            "Response": {
                "Check": [ "`assert $(STATUS) == 200`" ],
                "Success": [ "here we report: `report -n -t success`" ],
                "Failure": [ "here we report: `report -n -t failure`" ]
            }
        }
    },
    "Schedules": [
        {
            "Name": "rpt-demo",
            "Tests": "rpt",
            "Reporter": {
                "Path": "./status.log",
                    "Templates": {
                        "success": {
                            "error": "none",
                            "status" : "`cvt -i $(STATUS)`",
                            "isbn": "$(ISBN)",
                            "author": "$(AUTHOR)"
                        },
                        "failure": {
                            "error": "$(FAILURE)",
                            "status" : "`cvt -i $(STATUS)`"
                        }
                    }
            }
        }
    ]
}
```
and it will report:
```
{
    "error": "none",
    "status": 200,
    "isbn": "123456789",
    "author": "Forrest Jiang"
}
{
    "error": "xxxxxxx",
    "status": 400
}
{
    "error": "xxxxxxx",
    "status": 304
}
```
You may notice that `Templates` defines multiple template, and referred by `report -f` with json name, here is `success` and `failure`.

The advantage of template over formation is:
1. it could define readable json
2. no more `\"xxx\"`, just `"xxx"`

And you must know that, gmeter process `json.RawMessage` as a string and will compose it to get output. So any empty spaces will be kept and written to file. Of course, it is ok if you write template in one line like:
```json
{ "error": "none", "status" : "`cvt -i $(STATUS)`", "isbn": "$(ISBN)", "author": "$(AUTHOR)" }
```
You'll get one line for each successful response.

### Component multiplex
Sometimes we need compose many Configs for a server, use almost the same group of HTTP routes. A request message or even a Test defined inside one Config has to be defined in another. Same issues happens in almost every Config fields.

We need a mechanism so that nothing should be defined twice. That's why gmeter provides `Config.Imports` list. Each of this list contains a file path related to this config file which defines another Config, and this imported Config will be integrated into current one.

This is how integration happens:
    The most reuseable parts of a Config is: Hosts, Messages, Tests, Env, Options, let's call it component maps. All of them are maps. If a Config A imports Config B, each component maps of Config B will be copied into corresponding map of A, unless A already has a definition with same key of map item.

This makes Config to be imported by another looks like a library, and it could be reused by another Config.

It's exteamly useful if you use gmeter for a large number of Tests spreading to many Configs.

Let's show you how to use it by an simple example:

First let's define some messages and tests in a component config `base.json`:
```json
{
    "Hosts": {
        "library": { "Host": "http://${IP}:${PORT}" }
    },
    "Messages":{
        "new-book-req": {
            "Method": "POST", "Path": "/book/$(ISBN)",
                "Headers": {
                    "content-type": "application/json"
                },
                "Body": {
                    "author": "$(AUTHOR)",
                        "publisher": "$(PUBLISHER)",
                        "price": "`cvt -f $(PRICE)`"
                }
        },
        "query-book-req": {
            "Method": "GET", "Path": "/book/$(ISBN)"
        },
        "del-book-req": {
            "Method": "DELETE", "Path": "/book/$(ISBN)"
        }
    },
    "Tests": {
        "new-book-ok": {
            "Request": "new-book-req",
            "Response": { "Check": [ "`assert $(STATUS) == 200`" ] },
            "Timeout": "10s"
        },
        "new-book-fail": {
            "Request": "new-book-req",
            "Response": { "Check": [ "`assert $(STATUS) != 200`" ] },
            "Timeout": "10s"
        },
        "must-has-book": {
            "Request": "query-book-req",
            "Response": {
                "Check": [ "`assert $(STATUS) == 200`" ],
                "Success": [
                    "`env -w AUTHOR $(@json .author $(RESPONSE))`"
                ]
            },
            "Timeout": "10s"
        },
        "must-not-has-book": {
            "Request": "query-book-req",
            "Response": { "Check": [ "`assert $(STATUS) != 200`" ] },
            "Timeout": "10s"
        },
        "delete-book": {
            "Request": "del-book-req",
            "Response": { "Check": [ "`assert $(STATUS) == 200`" ] },
            "Timeout": "10s"
        }
    },
    "Options": {
        "AbortIfFail": "true"
    }
}
```

And we create a config `book-edit.json` to testify new-query-delete with check procedure:
```json
{
    "Name": "book-edit",
    "Imports": ["base.json"],
    "Schedules": [
        {
            "Name": "new-query-delete-check",
            "Tests": "new-book-ok|new-book-fail|must-has-book|delete-book|must-not-has-book",
            "Count": 1,
            "Env": {
                "ISBN": "123456789",
                "AUTHOR": "Forrest Jiang",
                "PUBLISHER": "DeepGlint",
                "PRICE": "122.4"
            }
        }
    ]
}
```
Then create another config `book-concurrent.json` to run concurrent new-query-delete procedure:

```json
{
    "Name": "book-concurrent",
    "Imports": ["base.json"],
    "Tests": {
        "book-from-list": {
            "PreProcess": "`list a.list | json -m`",
            "Request": "new-book-req",
            "Response": { "Check": [ "`assert $(STATUS) == 200`" ] },
            "Timeout": "10s"
        }
    },
    "Schedules": [
        {
            "Name": "new-query-delete",
            "Tests": "book-from-list|must-has-book|delete-book",
            "Concurrency": 100
        }
    ]
}
```
As you see, `book-concurrent.json` not only imports `must-has-book` and `delete-book`, it defines another Test `book-from-list` which quotes Request `new-book-req`.

gmeter allows multiple Config executions in a command line, you could execute them:
```
gmeter book-edit.json book-concurrent.json
```

`Imports` is not the only way to import configs. gmeter provide argument `-template=xxxx` to import a Config for all. For example, we need to execute lots of Configs, which imports lots of base Configs for one single HTTP server, we may create a base Config `template.json` for all other to define only `Hosts` and `Messages` because they do not change:
```json
{
    "Hosts": {
        "library": {
            "Host": "http://${IP}:${PORT}" 
        }
    },
    "Messages":{
        "new-book-req": {
            "Method": "POST", "Path": "/book/$(ISBN)",
                "Headers": {
                    "content-type": "application/json"
                },
                "Body": {
                    "author": "$(AUTHOR)",
                        "publisher": "$(PUBLISHER)",
                        "price": "`cvt -f $(PRICE)`"
                }
        },
            "query-book-req": {
                "Method": "GET", "Path": "/book/$(ISBN)"
            },
            "del-book-req": {
                "Method": "DELETE", "Path": "/book/$(ISBN)"
            }
    },
    "Options": {
        "AbortIfFail": "true"
    }
}
```
And component Config could ignore these fields:

```json
{
    "Tests": {
        "new-book-ok": {
            "Request": "new-book-req",
            "Response": { "Check": [ "`assert $(STATUS) == 200`" ] },
            "Timeout": "10s"
        },
        "new-book-fail": {
            "Request": "new-book-req",
            "Response": { "Check": [ "`assert $(STATUS) != 200`" ] },
            "Timeout": "10s"
        },
        "must-has-book": {
            "Request": "query-book-req",
            "Response": {
                "Check": [ "`assert $(STATUS) == 200`" ],
                "Success": [
                    "`env -w AUTHOR $(@json .author $(RESPONSE))`"
                ]
            },
            "Timeout": "10s"
        },
        "must-not-has-book": {
            "Request": "query-book-req",
            "Response": { "Check": [ "`assert $(STATUS) != 200`" ] },
            "Timeout": "10s"
        },
        "delete-book": {
            "Request": "del-book-req",
            "Response": { "Check": [ "`assert $(STATUS) == 200`" ] },
            "Timeout": "10s"
        }
    }
}
```
In the command line, we specify template config:
```
gmeter -template template.json book-edit.json book-concurrent.json
```
#### Test Base
We just discussed the multiplex of Config fields, now we'll discuss more detailed multiplex: Test Base.

Schedule can define a `TestBase` field, which is the name of a Test. But this Test is special: all its fields will be copied into Tests Schedule uses:
1. For string fields like `Host`, `Request`, `Timeout`, if target Test does not define them, use TestBase's definition
2. For list fields like `PreProcess`, all TestBase's list will be inserted to the head of target Test.
3. For `Request`, if target Test does not define it, use TestBase's definition
4. For `Response` fields, list fields will be inserted to target at the head and `Template` field is preferred in the target.

For example:
```json
{
    "Name": "book-edit-base",
    "Imports": ["base.json"],
    "Tests": {
        "base": {
            "PreProcess": "`list a.json | json -m`"
        }
    },
    "Schedules": [
        {
            "Name": "new-book",
            "Tests": "new-book-ok",
            "TestBase": "base"
        }
    ]
}
```
This Config will imports `base.json` for Test `new-book-ok`, which requires several variables. By define a `base` Test with a `PreProcess` to read from list for those variables, we may create a working Test.

`TestBase` is not used as often as Imports configs, but it will be useful if you want to do some common thing like checking status code for all tests in a single schedule.

### Json compare

Json compare is a mechanism provided by gmeter to compare a json data from a json template.

Json template defines a json embedded with variables and commands for concerned fields. It has the same structure with target json data. gmeter walks each fields of json template and corresponding json data, writes json data of this field to a special variable `$<value>`, then call json template of this field. If json template defines a string without any command or is basic types like boolean or number, just compare `$<value>` with data defined inside json template.

Give an simple example for demo. A json template is defined as:
```json
{
    "author": "Forrest Jiang",
    "price": "`assert $(PRICE) > 0`"
}
```
and a json data is:
```json
{
    "author": "Forrest Jiang",
    "publisher": "DeepGlint",
    "price": 122.4
}
```

gmeter will walk json template as:
1. visit `author`, get a string `"Forrest Jiang"`, and corresponding json data is `Forrest Jiang`. Write `Forrest Jiang` to `$<value>`, then compare `$<value>` with string `"Forrest Jiang"`. These are same string, continue walking.
2. visit `price`, get a command `assert $(PRICE) > 0`, and corresponding json data is `122.4`. Write `"122.4"` to `$<value>` and call `assert $(PRICE) > 0`. Command passes, continue walking.
3. template walking ends, and `publisher` in json data is ignored.

Json compare can process any fields, including compound field. For example, a template is defined as:
```json
{
    "book": "`json .author $<value> | assert $$ == $(AUTHOR)`",
}
```
and a json data is:
```json
{
    "book": {
        "author": "Forrest Jiang",
        "publisher": "DeepGlint",
        "price": 122.4
    }
}
```

When this template process `book`, its value a compound json object, gmeter will save the json data to `$<value>`. Template will read `author` field of this json value and make sure it's same as `$(AUTHOR)`.

Json compare in HTTP client could be deployed in `Response.Template`. It helps user to process json field in a nature way without extracting value manually by `json` command. With json compare, user could process HTTP response body inside this template, and check other parameters like status code inside `Response.Check`.

Json compare actually is far more powerful than we discussed here. For more information, refer to [jsonc](./jsonc.md).

### Flow control
First let's define QPS and Parallel.

QPS indicates in recent 1 second, how many request has been sent but not responded. Parallel indicates how many request has been sent but not responded. It is sure that `QPS <= Parallel`. Due to concurrent requests number is defined by `Schedule.Concurrency`, we know that `Parallel <= Schedule.Concurrency`.

To make a flow control, you may define `Schedule.QPS` and `Schedule.Parallel` to make sure gmeter send request under control and as precise as possible.

User should know that `Schedule.Concurrency` can not be used as parallel control, because it decides how many gmeter threads should be started for HTTP request, which includes request composing, client request execution, and response processing. With a given concurrency number, the parallel requests number is always less because some of them are composing requests and some of them are processing response. The parallel number is decided by concurrency number and the proportion one client request takes in one full execution. Less the proportion, less the parallel number.

# HTTP RESTful server
gmeter allows user create several HTTP RESTful servers from a config file.
```go
type HttpServer struct {
	Address string            // ":0" or ":port" or "ip:port"
	Routes  []*Route          // HTTP server routers
	Report  Report            // Optional reporter, may used in router processing
	Env     map[string]string // predefined global variables
}


type HttpServers struct {
	// Servers represented by a name
	Servers map[string]*HttpServer
}

```
`HttpServer` defines a single server which listen to `Address`, and dispatch requests to `Routes` by route matching.
```go
type Route struct {
	// HTTP request method this route will process, default for "GET"
	Method string

	// [dynamic] router path definition, it could take path variables like:
	//     "/var/js/{script}
	// and by accessing `$(script)` you'll get the path segment value, for example,
	// if request path is:
	//     "/var/js/query"
	// now `$(script)` will get value `query`
	//
	// And you can also specify request parameters taking by request path, for example:
	//     "/var/js/{script}?name=hello
	// and by accessing `$(name)` you'll get `hello`. Multiple parameter with same name will
	// be joined together separated by a space:
	//     "/var/js/{script}?name=hello&name=world
	// and by accessing `$(name)` you'll get `hello world`.
	Path string

	// [dynamic] required headers definition, if its value is a raw string like "application/json", it
	// requires request takes this header with value of "application/json"; or if it is a dynamic
	// segment(with embedded commands) like "`assert $$ == hello`", instead of string comparing, it
	// will be called.
	Headers  map[string]string

	Request  *RequestProcess            // HTTP request processing
	Response map[string]json.RawMessage // [dynamic] multiple responses template identified by key of map
	Env      map[string]string          // predefined local variables
}
```

The port server is listening will be written to a global variable `${HTTP.PORT}`. HTTP client could use this value to make up a host.

A route matches HTTP request by `Method` and `Path`. If `Headers` is defined, a header match is also required.
Route path matching follows [gorilla mux](https://github.com/gorilla/mux), and path variables and request parameters in URL are written into local variables automatically by gmeter.

`Request` is the HTTP request processing entity, defined and processed exactly like `Response` in HTTP client. We discuss no more here.

`Response` is a map of json template to be composed as response body. Each one has a name as key of map. `Request` should write name of response body template into `$(RESPONSE)` if response body is required.

If you're familiar with HTTP RESTful client, it's really easy for you to understand HTTP RESTful server. So we just gives an example to show you how to start a server:
```json
{
	"Servers": {
		"fruit-server": {
			"Address":  "127.0.0.1:8009",
			"Routes":  [
				{
					"Method":  "POST",
                    "Path":  "/add/{supplier}",
                    "Headers":  { "content-type":  "application/json" },
                    "Request":  {
                        "Template":  {
                            "Fruit":  "`env -w FRUIT $`",
                            "Qty":  "`assert $ > 10 | env -w QTY $`"
                        },
                        "Success":  [
                            "`env -w RESPONSE default`",
                            "`env -w STATUS 200`",
                            "`report -n -t add`"
                        ]
                    },
                    "Response": {
                        "default": {
                            "Fruit": "$(FRUIT)"
                        }
                    }
				}
			],
			"Report":  {
				"Path":  "./server.log",
				"Templates":  {
					"add" :{ "supplier": "$(supplier)", "fruit": "$(FRUIT)", "qty": "$(QTY)" }
				}
			}
		}
	}
}
```
This config defines an HTTP server `fruit-server` listening on `127.0.0.1:8009`. It defines only one route accepting a POST on `/add/{supplier}` that takes json body. URL matching will get `{supplier}` content and write to `$(supplier)` and quoted by `Report` template. See field comment on `Path` for detailed path parsing.

When a request is received, "Fruit" value will be written to `$(FRUIT)`, "Qty" will be checked and written to `$(QTY)` by `Template`. After that `Success` will set response to `default` defined in `Response`, and set HTTP response status code to 200. Then record request data to `server.log` by `report` command with a template `add`.


