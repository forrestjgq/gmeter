
# Variable
Variable refers to an environment value indexed by string.

There are two types of variable: local variable and global variable. Local variable is defined and can only be accessed in the domain of a test plan, and global variable is defined globally and can be accessed anywhere.

Two forms of variable representation are defined:
- `$(name)`: local variable
- `${name}`: global variable

Here `name` is variable name which is started with alpha or `_` and composed of `[a-zA-Z0-9_-.]`.

To read from variable, just use `$(name)` or `${name}`.

To write to variable, use express of `$(name) = "something with whitespace"` or `${name} = something`. Quotes are not required here except that white spaces is included.

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

| Variable | Type   | Read-Only | Description                               |
| --       | --     | --        | --                                        |
| TEST     | string | true      | name of current test                      |
| SEQUENCE | int64  | true      | Test running sequence number start from 0 |
| URL      | string | false     | HTTP request URL                          |
| REQUEST  | string | false     | HTTP request body                         |
| STATUS   | int    | false     | HTTP response status code                 |
| RESPONSE | string | false     | HTTP response body                        |
| INPUT    | string | false     | Input of command segments                 |
| OUTPUT   | string | false     | Output of command segments                |
| ERROR    | string | false     | Error output of command segments          |

# Command
Gmeter command is just like shell, with a few different.
1. instead of stdin/stdout/stderr, `$(INPUT)`/`$(OUTPUT)`/`$(ERROR)` are defined.
2. command alternatively reads parameter from variable if not explictely present. For example, `cmd <arg>/$(INPUT)` indicates if `<arg>` is not present, use `$(INPUT)` as parameter instead.
3. output will be writtern to `$(OUTPUT)`
4. error will be writtern to `$(ERROR)`
5. GMeter is a case sensitive system.

## cvt
`cvt [-b] [-d] <content>/$(INPUT)`

convert `<content>` or `$(INPUT)` to a bool(`-b`) or number(`-d`).

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

To support value of boolean and number, you need append a command `cvt [-b]/[-d]`:
```json
{
    "number": "`echo 1.0 | cvt -d`"
}
```

## echo
`echo <content>/$(INPUT)`

write `<content>` or `$(INPUT)` to `$(OUTPUT)`
## cat
`cat <path>/$(INPUT)`

Read all file content from given `<path>` and write to $(OUTPUT).

## write
`write [-c <content>] <path> `

Write `<content>` to given file represented by `<path>`, if `-c <content>` is not specified, write `$(INPUT)` instead.

## env
```
# write local environment variable
envw [-c <content>] <variable>
envd <variable>
```
`env` commands provide:
- `envw`: Write `<content>` to local variable named by `<variable>`
- `envd` deletes local variable named by `<variable>`

## assert
```
assert <expr>
```

assert will report an error if `<expr>` is evaluated as `false`.

`<expr>` accepts two forms: compare and logical judgment:

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

## list
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

## b64

```sh
# Encode string:
b64 <string>/$(INPUT)

# Encode file
b64 -f <file>/$(INPUT)
```

Base64 encode a string or file

## json
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


## Pipeline
A pipeline is a command queue executed one by one. Each command could write its output content to `$(OUTPUT)` and gmeter will copy that into `$(INPUT)`, and then call next command so that it may use `$(INPUT)` as its parameter.

So it works just like shell pipe.

This is how a pipeline is defined:
`cmd1 | cmd2 | cmd3 | ...`

If any command in a pipeline write "EOF" to `$(ERROR)`, pipeline finishes.

If any command in a pipeline write an error other than "EOF" to `$(ERROR)`, pipeline aborts and test will fail.

## lua support
You may embed lua script inside and by read and write variable to communicates with GMeter.

lua script can be embedded inside `<<` and `>>`, multiple lines are supported, or read from file

```sh
# embedded
lua<<  <script>  >>

# read from file
lua <file>/$(INPUT)
```


