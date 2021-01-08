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

`go get github.com/forrestjgq/gmeter`

or directly install:

`curl -sf https://gobinaries.com/forrestjgq/gmeter | sh`

# Usage
make sure you've add $GOPATH into your $PATH, then:
```
gmeter -config <config>
```
`<config>` is configure json file path, a sample can be get [here](example/sample.json)

Specially, to use a template in `config.Response.Template`, you should read [jsonc](jsonc.md) to construct a json template.

# configuration
All gmeter need is a configuration file. 

To create a dynamic and powerful test configuration, you need be familiar with gmeter variables and commands.

[Command](./command.md)

gmeter attempts to provide json configuration guide through go doc system:

[Config](https://godoc.org/github.com/forrestjgq/gmeter/config)

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
