# Overview
gmeter provides environment variables and a set of commands support for test cases to dynamically generate cases, check responses, report response with customized formation.

All the power of gmeter comes from commands and variables.

# Variable
Variable refers to an environment value indexed by string.

There are two types of variable: **local variable** and **global variable**. Local variable is defined and can only be accessed in the domain of a test plan, and global variable is defined globally and can be accessed anywhere.

Two forms of variable representation are defined:
- `$(name)`: local variable
- `${name}`: global variable

Here `name` is variable name which is started with alpha or `_` and composed of `[a-zA-Z0-9_-.]`.


## Global Variables
Global variables are those defined inside a schedule and can be accessed by all its tests in every stage at any time.
It is persisted across the whole lifetime of schedule.

| Variable | Type   | Read-Only | Description                        |
| --       | --     | --        | --                                 |
| CONFIG   | string | true      | name of current config             |
| SCHEDULE | string | true      | name of current schedule           |
| TPATH    | string | true      | Path of test config file directory |

## Local Variables
Local variables are defined in a single run of test pipeline. When next run starts, all previous variables will be obsolete.

| Variable | Type   | Read-Only | Description                                                                      |
| --       | --     | --        | --                                                                               |
| TEST     | string | true      | name of current test, set on test is scheduled to run                            |
| SEQUENCE | int    | true      | sequence number of test, start from 1                                            |
| ROUTINE  | int    | true      | routine id of test, start from 0                                                 |
| URL      | string | false     | HTTP request URL, set before HTTP request sent                                   |
| REQUEST  | string | false     | HTTP request body, set before HTTP request sent                                  |
| STATUS   | int    | false     | HTTP response status code, set after HTTP request being responded                |
| RESPONSE | string | false     | HTTP response body, set after HTTP request being responded and there is any body |
| INPUT    | string | false     | Input of command segments, set to $(OUTPUT) before a command is called           |
| OUTPUT   | string | false     | Output of command segments, set by command to write output                       |
| ERROR    | string | false     | Error output of command segments, set if command generate an error               |
| FAILURE  | string | true      | HTTP fail reason, set after HTTP process fails and before fail process is called |

When a new test(or tests group) starts a new round, all local will be cleared.

Local variables are concurrent-safe. If test runs in concurrent routines, each routine maintains an independant local variable.

Through `env` command family, test cases are able to manipulate local variables.

## Variable reference
Variables can be referenced in the string of config or command, for example, a `config.Test` is defined as:
```json
        "quantity-write": {
            "Method": "POST",
            "Path": "/repo",
            "Headers": {
                "content-type": "application/json"
            },
            "Body": {
                "repo": "fruit",
                "type": "$(FRUIT)",
                "quantity": "`cvt -i $(QTY)`"
            },
            "Response": {
                "Check": [
                    "`assert $(STATUS) == 200`"
                ]
            }
        }
```

In the request body, we defined a json with 3 members: `repo`, `type`, `quantity`. 
`repo` has a value of static string `"fruit"`, `type` defines an dynamic string which refers to a local variable `$(FRUIT)`, `quantity` defines a command refers to `$(QTY)`. Vairables referred in a string will be replaced with its value, and vairables referred in a command will be treated as an argument even its value contains spaces inside(just like refers variables in shell command line).

# Command
gmeter command acts just like shell, with a few difference:
1. instead of stdin/stdout/stderr, `$(INPUT)`/`$(OUTPUT)`/`$(ERROR)` are defined.
2. command alternatively reads parameter from variable if not explictely present. For example, `cmd <arg>/$(INPUT)` indicates if `<arg>` is not present, use `$(INPUT)` as parameter instead.
3. output, if any, will be writtern to `$(OUTPUT)`
4. if any error occurs, error string will be writtern to `$(ERROR)`
5. gmeter is a case sensitive system.

gmeter command process only strings, although some command will treat strings as numbers or booleans, it basicly takes string inputs and generates string output.

## iterable command

An iterable command is a command that generates valid output by keep calling it and at last reachs an end, at which time it generates an `EOF` error. An example of iterable command is `list`, which reads a line from a file each time it's called until end of file.

Iterable command is usually used in test preprocess to read cases from a list and write parameters used in test case into environment variables.

Iterable command alters gmeter's action on controlling test counts. If there is no iterable command, how many times test should runs in a shcedule depends on the `config.Schedule.Count`. But there is is any iterable commands in the preprocessing and request generating, `config.Schedule.Count` is discarded and test will be running until any iterable command generates an `EOF`.

## Pipeline
A pipeline is a command queue executed one by one. Each command could write its output content to `$(OUTPUT)` and gmeter will copy that into `$(INPUT)`, and then call next command so that it may(NOT must) use `$(INPUT)` as its parameter.

So it works just like shell pipe.

This is how a pipeline is defined:
`cmd1 | cmd2 | cmd3 | ...`

If any command in a pipeline write `EOF` to `$(ERROR)`, pipeline finishes.

If any command in a pipeline write an error other than `EOF` to `$(ERROR)`, pipeline aborts.

## Group
Commands and pipelines can be grouped as a list, for example, we define several `Check`s in `config.Response` to check HTTP response:
```json
	"Check": [
		"`assert $(STATUS) == 200`",
		"`json .type $(RESPONSE) | assert $(INPUT) == $(FRUIT)`",
		"`json .quantity $(RESPONSE) | assert $(INPUT) == $(QTY)`"
	]
```
Each of these commands or pipelines are called in the sequence they are defined.

In most cases, group behaves as if a long pipeline is splitted into several short pipelines. If error occurs, following pipelines won't be called. But there are exceptions:
   Any error happened in `config.Response.Success` and `config.Response.Fail` will abort current pipeline, but following pipelines will still be called.

## cvt - strip quotes and convert string to specified type
`cvt [-b] [-i] [-f] [-r] <content>/$(INPUT)`

convert `<content>` or `$(INPUT)` to :
- `-b`: bool value
- `-i`: integer value
- `-f`: float value
- `-r`: raw string value

This is used in json boolean value or number value representation. While we need produce a boolean or number as value of json, quote is not allowed to wrap it. For example:
```json
{
    "number": 1.0
}
```
here `1.0` can not be wrapped with quotes:

```json
{
    "number": "1.0"
}
```
`"1.0"` will be considered as a string.

Now if we use command to produce number:
```json
{
    "number": "`echo 1.0`"
}
```
we'll get `"1.0"` instead of `1.0`.

To support value of boolean and number, you need append a command `cvt [-b]/[-i]/[-f]`:
```json
{
    "number": "`echo 1.0 | cvt -f`"
}
```

There is another situation, while we define a template:
```json
{
    "body": "$(RESPONSE)"
}
```
here `$(RESPONSE)` is a json string like:
```json
{
    "name": "gmeter",
    "age": 10
}
```
we expect to get output of:
```json
{
    "body": {
        "name": "gmeter",
        "age": 10
    }
}
```
but actually we got:
```json
{
    "body": "{
        "name": "gmeter",
        "age": 10
    }"
}
```
a pair of quote mark ruins json grammar.

so we need remove extra `""` surrounding `$(RESPONSE)`, apply `cvt -r`:
```json
{
    "body": "`cvt -r $(RESPONSE)`"
}
```

## echo - write string to $(OUTPUT)
`echo <content...>/$(INPUT)`

write `<content...>` or `$(INPUT)` to `$(OUTPUT)`
Note that `<content...>` could be a space seperated string without quote like:
```
echo ${SCHEDULE} runs in routine $(ROUTINE) seq $(SEQUENCE)
```

## cat - write content of a file as string to $(OUTPUT)
`cat <path>/$(INPUT)`

Read all file content from given `<path>` and write to $(OUTPUT).

## write - write string to a file
`write [-c <content>] <path> `

Write `<content>` to given file represented by `<path>`, if `-c <content>` is not specified, write `$(INPUT)` instead.

## print - print string to stdout
`print <content...>/$(INPUT)`

print string into stdout, an extra new line `\n` is appended.
Note that `<content...>` could be a space seperated string without quote like:
```
print ${SCHEDULE} runs in routine $(ROUTINE) seq $(SEQUENCE)
```

## escape - escape quotes
`escape <content>/$(INPUT)`

replace `"` with `\"` in `<content>` or `$(INPUT)` to `$(OUTPUT)`

if you want to write a string value might containing `"` into a json, you need escape it to avoid json grammar error.

## strrepl - replace or delete substring
`strrepl <content> <substring> [<newstring>]`

replace substring `<substring>` in string `<content>` with `<newstring>`, if `<newstring>` is absent, all `<substring>` in `<content>` will be deleted.

## env command family
```
# write local environment variable
envw [-c <content>] <variable>
envd <variable>
envmv <src> <dst>
```

`env` command family includes:
- `envw`: Write `<content>` to local variable named by `<variable>`
- `envd` deletes local variable named by `<variable>`
- `envmv` move local variable `$(src)` value to `$(dst)`, `$(src)` will be cleared

## assert - condition checking
```
assert <condition...> [-h <hints...>]
```

assert will report an error if `<condition>` is evaluated as `false`. If `-h <hints...>` is present, error string will evaluate `<hints...>` and attach to error string for debugging.

`<condition>` accepts two forms: compare and logical judgment:

### compare expression
```
a == b
a != b
a > b
a >= b
a < b
a <= b
```

when compare operators are used, it compares both strings or numbers, here is the rule:
1. when `a` and `b` are integer numbers, all operators are supported
2. when `a` and `b` are numbers, but at least one of them are float number(with a '.' inside), `==` and `!=` will be judged by `abs(a-b)`, if this value is less then `0.00000001`, consider `a == b`, other operators are compared directly.
3. when `a` or `b` is not number, use string compare, and only `==` and `!=` can be applied, or it will report an error.

### logical judgment expression
```
a
!a
```

when logical judgment operators are used, `a` can be 
- `1`
- `0`
- `true`
- `false`

specially, `!$(var)` while `$(var)` is empty will be treat as true.

## list - read line by line from a file
`list <file>`

Read lines one by one from `<file>`, ignore empty lines.

When it reach end of file, returns empty string and write `"EOF"` into `$(ERROR)`.

Unlike `cat`, it will only output one non-empty line once be called, so if you define a request body like:
```json
{
    "url": "`list /file/path/to/list.txt`"
}
```
and `list.txt` contains:
```
/path/to/file1.txt
/path/to/file2.txt
/path/to/file3.txt
```
then first HTTP request will get request body:
```json
{
    "url": "/path/to/file1.txt"
}
```
second HTTP request will get request body:
```json
{
    "url": "/path/to/file2.txt"
}
```

**NOTE**:
    obviously list will use parameter at the first call, so any change of parameter will be useless after that.

## b64 - base64 encoding

```sh
# Encode string:
b64 <string>/$(INPUT)

# Encode file
b64 -f <file>/$(INPUT)
```

Base64 encode a string or file

## json - json query
```
json [-e] [-n] <path> <content>/$(INPUT)
```

`json` will parse `<content>` or `$(INPUT)` as json, and find `<path>`, write its content to `$(OUTPUT)`; if not found, write empty string.

if `-n` is present, expect `<path>` is array and  write length of this array to `$(OUTPUT)`:
1. if `<path>` is not found, output 0
2. if `<path>` is found, but not an array, report error
3. do not write content of `<path>` to `$(OUTPUT)`

if `-e` is present, report error if `<path>` not found

`<path>` follows these rules:
1. divided by '.', each part is called a segment
2. use `[n]` to represent an element of a list, `[]` for whole list
3. if target json is exepcted as an object(like `{...}`), the first segment is empty, in other word, it starts by '.'
4. if target json is an array, start by `[]` or `[n]`, here n is a number

Here gives some examples of path:
```json
	{
       "bool": true,
       "int": 3,
       "float": 3.0,
       "string": "string",
       "map": {
           "k1": "this",
           "k2": 2
       },
       "list": [
           "line1", "line2"
       ]
    }
```

- `.bool` is `true`
- `.string` is `string`
- `.map.k1` is `this`
- `.list` is `["line1", "line2"]`
- `.list.[1]` is `line1`

## if-then-else - if condition
```
if <condition> then <command1> [else <command2>]
```
if `<condition>` evaluates as `true`, then `<command1>` is executed, otherwise if `<command2>` is defined, it will be executed.

`<condition>` has same definition of `assert`.
`<command1>` and `<command2>` could be any command except `if-then-else` itself.

## report - write string to report file
```
report [-n] [-f format] [-t template]
```
`report` command is used to write strings into report file, which is defined by [Report](https://pkg.go.dev/github.com/forrestjgq/gmeter/config#Report) in [Schedule](https://pkg.go.dev/github.com/forrestjgq/gmeter/config#Schedule) of [Config](https://pkg.go.dev/github.com/forrestjgq/gmeter/config#Config).

If `Report.Path` is empty, all reports will be written to stdout.

The content to be written depends on a format string, which could be a formation string with embedded commands or vairables. This is an example of format string:
```
"{ \"Error\": \"$(ERROR)\", \"Status\": $(STATUS), \"Response\": $(RESPONSE)}\n"
```
it uses stored error string/http response status code/http response status body as a json, ending with a new line to seperate. You should know that gmeter does not append any new line for you.

Format string is retrieved from two sources: `Report.Format` in `Schedule` config, or optional `[-f format]` parameter. And `[-f format]` is preferred. If `[-t template]` is present, `Report.Templates[template]` will be applied as formation string.

If report without a valid format string, nothing is reported.

if `[-n]` is present, an extra newline `\n` is appended.


## lua support
NOT SUPPORTED YET.

You may embed lua script inside and by read and write variable to communicates with gmeter.

lua script can be embedded inside `<<` and `>>`, multiple lines are supported, or read from file

```sh
# embedded
lua<<  <script>  >>

# read from file
lua <file>/$(INPUT)
```


