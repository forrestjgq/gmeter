# What is gmeter
gmeter is a dynamic HTTP request tool with benchmark and monitor support. It's just like jmeter but more configurable and more faster.

# Features
1. test case configure with json;
2. gmeter environment variables access and fantastic embedded command system with pipeline support;
3. test control over specified count or iterable command
4. concurrency
5. test pipeline
6. customized response checking and reporting
7. proxy support
8. performance monitoring, QPS limiting(under development)
9. json compare based on template(under development)

# Install

```sh
go get github.com/forrestjgq/gmeter
```
It will be installed into $GOBIN(if it's empty, get from `go env $GOBIN`). It requires you've got a GO environment.

Or you may directly install to /usr/local/bin:
```sh
curl -sf https://gobinaries.com/forrestjgq/gmeter | sh
```
root permission may be required to install.

# Usage
```
gmeter [-template <template-config>][-httpsrv <http-server-config>] [-arcee <arcee-server-config>] [<config>, <config>, ...]
```
- `<config>` is a file path, it could be
    - A json file path(end with .json), a sample can be get [here](example/sample.json), see [Configuration](https://godoc.org/github.com/forrestjgq/gmeter/config#Config), or
    - A list file, each line contains a file path ends with .json will be treated as a gmeter configuration and will be called. If it is an relative path, it's related to `<config>`'s directory. In a line, `#` is considered to be start of comment, any thing after (and include) `#` will be ignored. Empty line is allowed.
    - A directory, any .json file in this directory and sub-directories of this directory will be treated as a test configuration and will be called.
- `<template-config>` is a configure json file used as a base configuration. If this argument is present, the Hosts/Messages/Tests/Env/Options will be copied to all `<config>` if target configuration does not define those items identified by the key of map. An example could be find in [template](example/base.json) and [configuration](example/sep.json), and the command line would be `gmeter -template example/base.json example/sep.json`.
- `<http-server-config>` is configure json file path for creating http server, a sample can be get [here](example/server.json), see [HTTP Server Configuration](https://godoc.org/github.com/forrestjgq/gmeter/config#HttpServers) for more information.
- `<arcee-server-config>` is configure json file path for creating arcee server, a sample can be get [here](example/arcee.json), see [Arcee Server Configuration](https://godoc.org/github.com/forrestjgq/gmeter/config#Arcee) for more information.

# configuration
All gmeter need is a configuration file. 

To create a dynamic and powerful test configuration, you need be familiar with gmeter variables and commands.

[Command](./command.md)

gmeter attempts to provide json configuration guide through go doc system:

[Configurations](https://godoc.org/github.com/forrestjgq/gmeter/config)

It's recommended that you read this package overview first, and then jump to Config and its members.

# sample
Assume you need execute these test:
1. Post a request to write the quntity of a fruit to server
2. Query 10000 times to get quantity of this fruit
3. Delete the quantity of this fruit from server
4. Query again to make sure delete works.

Here is a configure to do so:
```json
{
    "Name": "fruit",
    "Hosts": {
        "-": {
            "Host": "http://127.0.0.1:8009"
        }
    },
    "Messages": {
        "query-request": { "Path": "/query" }
    },
    "Tests": {
        "add": {
            "RequestMessage": {
                "Method": "POST",
                "Path": "/add",
                "Headers": { "content-type": "application/json" },
                "Body": {
                    "Fruit": "${FRUIT}",
                    "Qty": 100
                }
            },
            "Response": {
                "Check": [ "`assert $(STATUS) == 200`" ]
            }
        },
        "query": {
            "Request": "query-request",
            "Response": {
                "Check": [
                    "`assert $(STATUS) == 200`",
                    "`json .Fruit $(RESPONSE) | assert $$ == ${FRUIT}`",
                    "`json .Qty $(RESPONSE) | assert $$ == 100`"
                ]
            },
            "Timeout": "4s"
        },
        "delete": {
            "RequestMessage": { "Method": "DELETE", "Path": "/del" },
            "Response": { "Check": [ "`assert $(STATUS) == 200`" ] }
        },
        "fail": {
            "Request": "query-request",
            "Response": {
                "Check": [ "`assert $(STATUS) != 200`" ]
            },
            "Timeout": "1s"
        }
    },
    "Schedules": [
        {
            "Name": "add-fruit",
            "Tests": "add",
            "Count": 1
        },
        {
            "Name": "concurrent-query-fruit",
            "Tests": "query",
            "Count": 100000,
            "Concurrent": 100
        },
        {
            "Name": "del-fruit",
            "Tests": "delete|fail",
            "Count": 1
        }
    ],
    "Env": { "FRUIT": "apple" },
    "Options": {
        "AbortIfFail": "true"
    }
}
```

You may find many samples in [example](example), which are used in [start_test](./internal/meter/start_test.go).
These samples cover most scenarios gmeter supports.

# TODOs
1. gomark support
2. QPS restriction
3. better logging
4. better guides
