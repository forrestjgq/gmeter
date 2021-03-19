<p align="center">
<img 
    src="gmeter_logo.png" 
    width="387" height="100" border="0" alt="gmeter">
<br>
<a href="https://godoc.org/github.com/forrestjgq/gmeter"><img src="https://img.shields.io/badge/api-reference-blue.svg?style=flat-square" alt="GoDoc"></a>
</p>

<p align="center">Make RESTful HTTP More Easy</a></p>
# What is gmeter

gmeter customizes HTTP RESTful clients and HTTP RESTful servers and runs them by configuration. With variable and command system supports, json acts as a script language to process HTTP request and response.

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
10. arithmatic and logical expressions support.

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
gmeter [options] <config>[, <config>, ...]
```
 `<config>` is a file path, it could be
    - A json file path(end with .json), a sample can be get [here](example/sample.json), see [Configuration](https://godoc.org/github.com/forrestjgq/gmeter/config#Config), or
    - A list file, each line contains a file path ends with .json will be treated as a gmeter configuration and will be called. If it is an relative path, it's related to `<config>`'s directory. In a line, `#` is considered to be start of comment, any thing after (and include) `#` will be ignored. Empty line is allowed.
    - A directory, any .json file in this directory and sub-directories of this directory will be treated as a test configuration and will be called.

Optional arguments includes:
- `-t, -template <config>`: load an HTTP client template configuration. `<template-config>` is a configure json file used as a base configuration. If this argument is present, the Hosts/Messages/Tests/Env/Options will be copied to all `<config>` if target configuration does not define those items identified by the key of map. An example could be find in [template](example/base.json) and [configuration](example/sep.json), and the command line would be `gmeter -template example/base.json example/sep.json`.
- `-httpsrv <http-server-config>`: start an HTTP server. `<http-server-config>` is configure json file path for creating http server, a sample can be get [here](example/server.json), see [HTTP Server Configuration](https://godoc.org/github.com/forrestjgq/gmeter/config#HttpServers) for more information.
- `-arcee <arcee-server-config>`: start an Arcee file server. `<arcee-server-config>` is configure json file path for creating arcee server, a sample can be get [here](example/arcee.json), see [Arcee Server Configuration](https://godoc.org/github.com/forrestjgq/gmeter/config#Arcee) for more information.
- `-e="k1=v1 k2=v2 ..."`: predefined global variables. Each variable is defined in `key=value` form, and multiple key value pairs are seperated by spaces.
- `-call <commandline>`: command line called before any config is executed and after any server is started.
- `-f <final>`: final config called even running fails.
- `-gm <port>`: set [GoMark](https://github.com/forrestjgq/gomark) HTTP port, default 7777.

# Documents
- [Guideline](./guideline.md): A guideline explains with examples for you to ease into gmeter:
- [Configurations](https://godoc.org/github.com/forrestjgq/gmeter/config): godoc for configuration description
- [Command](./command.md): gmeter command system and manual
- [Json Compare](./jsonc.md): Json compare manual.

# TODOs
1. gomark support
2. better logging
