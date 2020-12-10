
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

## cat
`cat <path>/$(INPUT)`

Read all file content from given `<path>` and print.

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

`<expr>` accepts two forms: compare and logical judgement:

### compare
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

### logical judgement
```
a
!a
```

when logical judgement operators are used, `a` can be 
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

## Pipeline
A pipeline is a command queue executed one by one. Each command could write its output content to `$(OUTPUT)` and gmeter will copy that into `$(INPUT)`, and then call next command so that it may use `$(INPUT)` as its parameter.

So it works just like shell pipe.

This is how a pipeline is defined:
`cmd1 | cmd2 | cmd3 | ...`

If any command in a pipeline write "EOF" to `$(ERROR)`, pipeline finishes.

If any command in a pipeline write an error other than "EOF" to `$(ERROR)`, pipeline aborts and test will fail.

## lua support
You may embed lua script inside and by read and write variable to communicates with GMeter.

lua script can be embeded inside `<<` and `>>`, multiple lines are supported, or read from file

```sh
# embedded
lua<<  <script>  >>

# read from file
lua <file>/$(INPUT)
```


