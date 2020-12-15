# what is gmeter
go http benchmark and monitor tool, just like jmeter but more configurable.

# install

`go get github.com/forrestjgq/gmeter/cmd/gmeter/...`

# usage
make sure you've add $GOPATH into your $PATH, then:
```
gmeter -config <config>
```
`<config>` is configure json file path, a sample can be get [here](./sample.json)


# Design Goal
1. configure with json
2. msg/template definition 
3. dynamic response field save and transfer to next
4. automatic id increasing
5. pipeline
6. proxy
7. concurrency of large scale
8. response checking condition definition
9. performance monitoring, like latency recorder(gomark)

# ToDo
    

# Done
    a single test runs for a simple http request
    add gomark
