
- [Overview](#overview)
- [Variable](#variable)
  * [Variable Decoration](#variable-decoration)
  * [Global Variables](#global-variables)
  * [Local Variables](#local-variables)
  * [Json compare environment variables](#json-compare-environment-variables)
  * [Variable reference](#variable-reference)
- [Expression](#expression)
- [Command](#command)
  * [iterable command](#iterable-command)
  * [Pipeline](#pipeline)
  * [Embedded command](#embedded-command)
  * [Group](#group)
  * [cvt - strip quotes and convert string to specified type](#cvt---strip-quotes-and-convert-string-to-specified-type)
  * [nop - do nothing](#nop---do-nothing)
  * [fail - abort current execution(pipeline or group)](#fail---abort-current-execution-pipeline-or-group-)
  * [echo - echo string](#echo---echo-string)
  * [cat - read content of a file](#cat---read-content-of-a-file)
  * [write - write string to a file](#write---write-string-to-a-file)
  * [sleep - sleep for a while](#sleep---sleep-for-a-while)
  * [print - print string to stdout](#print---print-string-to-stdout)
  * [escape - escape quotes](#escape---escape-quotes)
  * [strrepl - replace or delete substring](#strrepl---replace-or-delete-substring)
  * [strlen - get string length](#strlen---get-string-length)
  * [db - database accessing](#db---database-accessing)
  * [env - local variable operations](#env---local-variable-operations)
  * [eval - expression calculation](#eval---expression-calculation)
  * [assert - condition checking](#assert---condition-checking)
  * [list - read line by line from a file](#list---read-line-by-line-from-a-file)
  * [b64 - base64 encoding](#b64---base64-encoding)
  * [json - json query](#json---json-query)
  * [until - do test until condition satisfied](#until---do-test-until-condition-satisfied)
  * [if-then-else - if condition](#if-then-else---if-condition)
  * [report - write string to report file](#report---write-string-to-report-file)
  * [call - function call](#call---function-call)
  * [lua support](#lua-support)

<small><i><a href='http://ecotrust-canada.github.io/markdown-toc/'>Table of contents generated with markdown-toc</a></i></small>

# Overview
gmeter provides environment variables and a set of commands support for test cases to dynamically generate cases, check responses, report response with customized formation.

All the power of gmeter comes from commands and variables.

# Variable
Variable refers to an environment value indexed by string.

There are two types of variable: **local variable** and **global variable**. Local variable is defined and can only be accessed in the domain of a test plan, and global variable is defined globally and can be accessed anywhere.

Two forms of variable representation are defined:
- `$(name)`: local variable
- `${name}`: global variable
- `$<name>`: json compare environment variable, or json variable

Here `name` is variable name which is started with alpha or `_` and composed of `[a-zA-Z0-9_-.]`.

## Variable Decoration
A variable decoration is to add a prefix sign before variable name. gmeter support:
- `#name`: get length of value of vairable `name`
- `?name`: check existence of variable `name`. It reutrns `true` if it exists, `false` otherwise.

These decorations apply to all variables(local, global, json). Assuming we've got a local variable `LocalVar` with value `hello` and a global `GlobalVar` with value `forrest`:
- `$(#LocalVar)` will get `5`
- `$(#NotExist)` will get `0`
- `$(?NotExist)` will get `false`
- `${#GlobalVar}` will get `7`
- `${?GlobalVar}` will get `true`

This adds a restriction to variable name:
    **While writing a variable, it's name can not be decorated. Attemption to write value to a variable like `#hello` will trigger a panic error**.
## Global Variables
Global variables are those defined inside a schedule and can be accessed by all its tests in every stage at any time.
It is persisted across the whole lifetime of schedule.

| Variable | Type   | Read-Only | Description                        |
| --       | --     | --        | --                                 |
| CONFIG   | string | true      | name of current config             |
| SCHEDULE | string | true      | name of current schedule           |
| TPATH    | string | true      | Path of test config file directory |
| CWD      | string | true      | Path of current working directory  |

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
| FAILURE  | string | true      | HTTP fail reason, set after HTTP process fails and before fail process is called |

When a new test(or tests group) starts a new round, all local will be cleared.

Local variables are concurrent-safe. If test runs in concurrent routines, each routine maintains an independant local variable.

Through `env` command family, test cases are able to manipulate local variables.

## Json compare environment variables
These environment contains two predefined variables:
- `$<key>`: json object member key, only present if current target is an object member.
- `$<value>`: json value, for basic type(bool/number/string) it's their string formation, for object and list it's mashalled json string.


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

# Expression
Expression is used in some commands like `if`/`eval`/`assert` commands. Actually any command requires an expression will declare its argument with `<expr>`.

Expression supports aritchmatic and logical calculations following C language. Here are operators supported in the descent priority order:
- =
- ++, --, !
- \*, /, %
- +, -
- >, >=, <, <=
- ==, !=
- &&
- ||

Expression processes everything as string. In arithmatic calculation, strings will be converted to numbers. For most operators it will be a float, so is the result. `5+2` will produce `7.00000000` instead of `7` and will be saved as a string `"7.00000000"`.
But for `%` operator, numbers will be converted to integers. so `7.5 % 3.1` will produce `1`. 
To make description easy, we'll ignore the `.00000000` part from here.

Strings could be wrapped inside `''`. Although everything is treated as string, space will be used to seperate words. `HELLO` equals to `'HELLO'` but `HELLO WORLD`, which defines 2 strings, does not equal to `'HELLO WORLD'`.

`!`/`&&`/`||` requires one or 2 bool operands. A bool operands should be a `"TRUE"` or `"true"` or `"FALSE"` or `"false"` or experssions that produces these values. `>, >=, <, <=, ==, !=` will produces a bool value. Let's show you how it works by examples:
- `! TRUE` produces `"FALSE"`
- `! 'false'` produces `"TRUE"`
- `3 > 2` produces `"TRUE"`
- `3 > 2 && 2 < 1` produces `"FALSE"`
- `3 > 2 || 2 < 1` produces `"TRUE"`

`()` is used to group expressions: `(3 + 2) * 10` produces `50`, `((3 + 2) * (6 - 4)) / 2` produces `5`

`$(var)`, `${var}`, `$<var>` can be used in expression to read local/global/json environment variables. Assuming a local variable `VAR` has a value `4`, `$(VAR) + 3` produces `7`.

`$(@cmd args...)` is used to call a command and replace with its output. `$(@echo 7) + 3` produces `10`. See command usage in next chapter for more.

The assign operator `=` is used to set local variables. The left part can only be a variable name and right part will be an expression: `a=3+2` will write `5` to local variable `a`. Please note that variable can not be a left value in the form of `$(a)`, which is actually a right value. These expressions are invalid:
- `$(a) = 3`
- `a + 3 == 6`

Multiple expresssions could be defined sperated by `;` and gmeter will calculate them in the sequence they are defined, and use the output of last one as the final result: `a = 3; a > 3; $(a) + 2` will set local variable `a` to 3 first, and calculate `a > 3`, which produces a bool value `"FALSE"`, but it is not the final expression, so it is discarded; then calculate the final one `$(a) + 2` and produces `5`, which is the final result of the whole expression.

# Command
gmeter command acts just like shell but is case sensitive. It will write a string as output, if error occurs, error string will be written to `$(ERROR)`. Command may output empty string.

A command is defined inside back quotes like:
```
`cmd arg1 arg2`
```
If an argument accept a string containing spaces, it may be defined with variable arguments like:
```
echo <content...>
```
it indicates that `<content>` could be zero or more than 1 arguments and those arguments will be joined with spaces to a string as a whole argument:
```
echo hello world
```
will produce `hello world`.

If command is embedded inside a string like:
```
"hello `cmd arg1 arg2` world"
```
the command will be replaced by the output of `cmd arg1 arg2`.

gmeter command process only strings, although some command will treat strings as numbers or booleans, it basicly takes string inputs and generates string output.

Specially, if an argument of command takes an expression, `<expr>` is used to define argument.

## iterable command

An iterable command is a command that generates valid output by keep calling it and at last reachs an end, at which time it generates an `EOF` error. An example of iterable command is `list`, which reads a line from a file each time it's called until end of file.

Iterable command is usually used in test preprocess to read cases from a list and write parameters used in test case into environment variables.

Iterable command alters gmeter's action on controlling test counts. If there is no iterable command, how many times test should runs in a shcedule depends on the `config.Schedule.Count`. But there is is any iterable commands in the preprocessing and request generating, `config.Schedule.Count` is discarded and test will be running until any iterable command generates an `EOF`.

## Pipeline
A pipeline is a command queue executed one by one. Pipeline will save output of a command, and next command may optionally use `$$` to access that output. The first command will get empty string from `$$` and last command will output a string as the output of pipeline. 

So it works just like shell pipe.

This is how a pipeline is defined:
`cmd1 | cmd2 | cmd3 | ...`

Some command will use `$$` instead if one of its argument is not present, and the replaceable argument will be defined as `arg/$$`. If a command is defined as:
```
cmd arg1 arg2 arg3/$$
```
here if `arg3` is not present, `cmd` will read content of previous command output as `arg3`.

An example: `echo hello world | strlen` will output the string length of `hello world` just like `strlen hello world`.

In the execution of pipeline, any error will abort a pipeline.

## Embedded command
Command could be embedded inside another command and its output will be used as an argument. The definition is:
```
$(@cmd arguments...)
$(@cmd1 | cmd2 | cmd3 | ...)
```
`@` must be the first character inside `()` to distinguish with local variable reading.

For example:
```
print $(@echo 3 | env -w HELLO | env -r HELLO)
```
Here `$(@echo 3 | env -w HELLO | env -r HELLO)` is an embedded command, it's output, which is `3` will be used as argument of `print` command, making it equals to;
```
print 3
```

Multiple level embedded is supported, such as:
```
assert $(@echo $(@echo $(@echo 3))) == $(@echo 3)
```

Sometimes an embedded command euqals to a pipeline like:
```
echo 3 | env -w HELLO
env -w HELLO $(@echo 3)
```
These two command does the same thing, but things changes in these two commands:
```
echo 5 | assert $(@eval $$ + 3) > 3
echo 5 | eval $$ + 2 | assert $$ > 3
```
The first command seems does the same work as the second one, but actually it fails:
```
pipeline[0]: strconv.ParseFloat: parsing "": invalid syntax
```
This happens because embedded command can NOT visit outside `$$`. An embedded command is another pipeline which takes no input. so `$(@eval $$ + 3)` will try to parse `$$`, which is empty string, to float number, and it fails.

So you can NOT use `$(INPUT)` or `$$` inside an embedded command.

## Group
Commands and pipelines can be grouped as a list, for example, we define several `Check`s in `config.Response` to check HTTP response:
```json
	"Check": [
		"`assert $(STATUS) == 200`",
		"`json .type $(RESPONSE) | assert $$ == $(FRUIT)`",
		"`json .quantity $(RESPONSE) | assert $$ == $(QTY)`"
	]
```
Each of these commands or pipelines are called in the sequence they are defined. Any item in the list will start with a `$$` of empty string and its output will not be deliver to next command as `$$`. Error from any item will abort the whole group.

## cvt - strip quotes and convert string to specified type
`cvt [-b] [-i] [-f] [-r] <content>/$$

convert `<content>` to :
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

## nop - do nothing
`nop`

nop is a command that does nothing.

## fail - abort current execution(pipeline or group)
`fail <content...>/$$`

Throw error with content `<content...>`.

Note that `<content...>` could be a space seperated string without quote like:
```
fail ${SCHEDULE} fails in routine $(ROUTINE) seq $(SEQUENCE)
```

## echo - echo string
`echo <content...>/$$`

produce a string `<content...>`.

Note that `<content...>` could be a space seperated string without quote like:
```
echo ${SCHEDULE} runs in routine $(ROUTINE) seq $(SEQUENCE)
```

## cat - read content of a file
`cat <path>/$$`

Read all file content from given `<path>`.

## write - write string to a file
`write [-c <content>] <path> `

Write `<content>` to given file represented by `<path>`, if `-c <content>` is not specified, write `$$` instead.

## sleep - sleep for a while
`sleep <duration>`

sleep for a while specified by `<duration>`

`<duration>` is a string represent some time, for example: `1s30ms` for 1 second and 30 milliseconds, `2m30s` for 2 minutes and 30 seconds, `100ms` for 100ms...

## print - print string to stdout
`print <content...>/$$`

print string into stdout, an extra new line `\n` is appended.

Note that `<content...>` could be a space seperated string without quote like:
```
print ${SCHEDULE} runs in routine $(ROUTINE) seq $(SEQUENCE)
```

`print` will output nothing.

## escape - escape quotes
`escape <content>/$$`

replace `"` with `\"` in `<content>`.

if you want to write a string value might containing `"` into a json, you need escape it to avoid json grammar error.

## strrepl - replace or delete substring
`strrepl <content> <substring> [<newstring>]`

replace substring `<substring>` in string `<content>` with `<newstring>`, if `<newstring>` is absent, all `<substring>` in `<content>` will be deleted.

## strlen - get string length
`strlen <content...>/$$`

get the string length by UTF-8 counting.

## db - database accessing
```
db -w <variable> <content...>/$(INPUT)
db -r <variable>
db -d <variable>
```

Database is a persistent container to store key-value pairs, unlike local environment, it remains across sessions and unlike global environment, it can be accessed by `-r`(read), `-d`(delete), `-w` write.

`db` command will apply decoration as variables, so `db -r #name` will read length of data base item `name` and `db -r ?name` will get `true` if database item `name` exists or `false` if not. And variable name with decoration is not allowed.

## env - local variable operations

```shell
// move content of variable <src> to variable <dst>, and <src> variable will be deleted
env -m <src> <dst>

// write local environment variable
env -w <variable> <content...>/$(INPUT)

// read from local <variable>	
env -r <variable>

// delete <variable> from local environment
env -d <variable>
```
## eval - expression calculation
```
eval <expr>
```

eval command will calculate an expression.

## assert - condition checking
```
assert <expr>
```

assert will report an error if `<expr>` is NOT evaluated as `TRUE`. 

## list - read line by line from a file
`list <file>`

Read lines one by one from `<file>`, ignore empty lines or any line starts with`#`, and the perfix and suffix spaces will be removed. If `<file>` is a related file path, it's related to configuration file's directory(`$(TPATH)`);

When it reach end of file, returns empty string and write `"EOF"` into `$(ERROR)`.

Unlike `cat`, it will only output one non-empty line once be called, so if you define a request body like:
```json
{
    "url": "`list /file/path/to/list.txt`"
}
```
and `list.txt` contains:
```
# this is a comment
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
b64 <string>/$$

# Encode file
b64 -f <file>/$$

# Decode string:
b64 -d <string>/$$

# Decode file
b64 -d -f <file>/$$
```

Base64 encode/decode a string or file

## json - json query
```
json [-m] [-e] [-n] <path> <content>/$$
```

`json` will parse `<content>` or `$$` as json, and find `<path>`, output its content; if not found, write empty string.

if `-n` is present, expect `<path>` is array and output length of this array:
1. if `<path>` is not found, output 0
2. if `<path>` is found, but not an array, report error
3. if `<path>` is found and it's an array, output length of this array.

if `-e` is present, report error if `<path>` not found

if `-m` is present, the segment `<path>` indicates must be a map and the json key value will be stored in local environment. For example:
```json
{
	"a": 1,
	"b": {
		"c": "hello",
		"e": true
	}
}
```
`json -m .` will create 3 local variables:
- "a": 1.00000000, here integer will be stored as float
- "b.c": "hello"
- "b.e": "true"

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

## until - do test until condition satisfied
```
until <expr>
```
`until` command is an iterate command.

If `<expr>` evaluates as `TRUE`, an `EOF` is generated.

This is offen used to do prefix HTTP testing until success. For example, send `PING` HTTP request to HTTP server to make sure it pongs to indicate the HTTP server is ready.

## if-then-else - if condition
```
if <expr> then <command1> [else <command2>]
```
if `<expr>` evaluates as `TRUE`, then `<command1>` is executed, otherwise if `<command2>` is defined, it will be executed.

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

## call - function call
```
call <function> <arguments...>
```

Call a function `<function>` defined by `Config.Functions`, and the arguments is listed in `<arguments...>`.

## lua support
NOT SUPPORTED YET.

You may embed lua script inside and by read and write variable to communicates with gmeter.

lua script can be embedded inside `<<` and `>>`, multiple lines are supported, or read from file

```sh
# embedded
lua<<  <script>  >>

# read from file
lua <file>/$$
```


