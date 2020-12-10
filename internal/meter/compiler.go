package meter

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/forrestjgq/gmeter/internal/argv"
)

type command interface {
	produce(bg *background)
	close()
}

////////////////////////////////////////////////////////////////////////////////
//////////                          segments                         ///////////
////////////////////////////////////////////////////////////////////////////////

type segment interface {
	getString(bg *background) (string, error)
}
type staticSegment string

func (ss staticSegment) getString(bg *background) (string, error) {
	return string(ss), nil
}

type dynamicSegment struct {
	f func(bg *background) (string, error)
}

func (ds dynamicSegment) getString(bg *background) (string, error) {
	return ds.f(bg)
}

type segments []segment

func (s segments) isStatic() bool {
	for _, seg := range s {
		if _, ok := seg.(staticSegment); ok {
			return true
		}
	}
	return false
}
func (s segments) compose(bg *background) (string, error) {
	arr := make([]string, len(s))
	for i, seg := range s {
		str, err := seg.getString(bg)
		if err != nil {
			return "", err
		}
		arr[i] = str

	}
	if len(arr) == 1 {
		return arr[0], nil
	}
	if len(arr) == 0 {
		return "", nil
	}

	return strings.Join(arr, ""), nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                             echo                          ///////////
////////////////////////////////////////////////////////////////////////////////

type cmdEcho struct {
	content segments
}

func (c *cmdEcho) close() {
	c.content = nil
}

func (c *cmdEcho) produce(bg *background) {
	if content, err := c.content.compose(bg); err != nil {
		bg.setError("echo: " + err.Error())
	} else {
		bg.setOutput(content)
	}
}

func makeEcho(content string) (command, error) {
	if len(content) == 0 {
		content = "$(" + KeyInput + ")"
	}
	seg, err := makeSegments(content)
	if err != nil {
		return nil, err
	}

	return &cmdEcho{content: seg}, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                             cat                           ///////////
////////////////////////////////////////////////////////////////////////////////

type cmdCat struct {
	static  bool
	path    segments
	content []byte
}

func (c *cmdCat) close() {
	c.content = nil
}

func (c *cmdCat) produce(bg *background) {
	if len(c.content) == 0 {
		path, err := c.path.compose(bg)
		if err != nil {
			bg.setError("cat: " + err.Error())
			return
		}

		if f, err := os.Open(filepath.Clean(path)); err != nil {
			bg.setError("cat: " + err.Error())
		} else {
			if b, err1 := ioutil.ReadAll(f); err1 != nil {
				bg.setError("cat: " + err1.Error())
			} else {
				if c.static {
					c.content = b
				}
				bg.setOutput(string(b))
			}
			_ = f.Close()
		}
	} else {
		bg.setOutput(string(c.content))
	}
}

func makeCat(path string) (command, error) {
	if len(path) == 0 {
		path = "$(" + KeyInput + ")"
	}
	seg, err := makeSegments(path)
	if err != nil {
		return nil, err
	}

	return &cmdCat{
		path:   seg,
		static: seg.isStatic(),
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                           write                           ///////////
////////////////////////////////////////////////////////////////////////////////
type cmdWrite struct {
	path    segments
	content segments
}

func (c *cmdWrite) close() {
	c.content = nil
	c.path = nil
}

func (c *cmdWrite) produce(bg *background) {
	content, err := c.content.compose(bg)
	if err != nil {
		bg.setError("write: " + err.Error())
		return
	}
	// do not check content here
	path, err := c.path.compose(bg)
	if err != nil {
		bg.setError("write: " + err.Error())
		return
	}
	if len(path) == 0 {
		bg.setError("write: empty file path")
		return
	}

	if f, err := os.Create(filepath.Clean(path)); err != nil {
		bg.setError("write: " + err.Error())
	} else {
		if _, err1 := f.WriteString(content); err1 != nil {
			bg.setError("write: " + err1.Error())
		}
		_ = f.Close()
	}
}

func makeWrite(v []string) (command, error) {
	content := ""
	fs := flag.NewFlagSet("write", flag.ContinueOnError)
	fs.StringVar(&content, "c", "$(INPUT)", "content to write to file, default using local input")
	err := fs.Parse(v)
	if err != nil {
		return nil, err
	}
	v = fs.Args()
	if len(v) != 1 {
		return nil, fmt.Errorf("write path not specified")
	}
	path := v[0]
	c := &cmdWrite{}
	if c.path, err = makeSegments(path); err != nil {
		return nil, err
	}
	if c.content, err = makeSegments(content); err != nil {
		return nil, err
	}
	return c, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                            env                            ///////////
////////////////////////////////////////////////////////////////////////////////

const (
	envWrite = iota
	envDelete
)

type cmdEnv struct {
	op       int
	variable segments
	value    segments
}

func (c *cmdEnv) close() {
	c.variable = nil
	c.value = nil
}

func (c *cmdEnv) produce(bg *background) {
	variable, err := c.variable.compose(bg)
	if err != nil {
		bg.setError("env: " + err.Error())
		return
	}
	if c.op == envDelete {
		bg.delLocalEnv(variable)
	} else if c.op == envWrite {
		value, err := c.value.compose(bg)
		if err != nil {
			bg.setError("env: " + err.Error())
			return
		}
		bg.setLocalEnv(variable, value)
	} else {
		bg.setError("env: unknown operator " + strconv.Itoa(c.op))
	}
}

func makeEnvw(v []string) (command, error) {
	content := ""
	fs := flag.NewFlagSet("envw", flag.ContinueOnError)
	fs.StringVar(&content, "c", "$(INPUT)", "content to write to local environment, default using local input")
	err := fs.Parse(v)
	if err != nil {
		return nil, err
	}
	v = fs.Args()
	if len(v) != 1 {
		return nil, fmt.Errorf("write path not specified")
	}
	variable := v[0]
	c := &cmdEnv{op: envWrite}
	if c.variable, err = makeSegments(variable); err != nil {
		return nil, err
	}
	if c.value, err = makeSegments(content); err != nil {
		return nil, err
	}
	return c, nil
}
func makeEnvd(v []string) (command, error) {
	variable := v[0]
	c := &cmdEnv{op: envDelete}
	var err error
	if c.variable, err = makeSegments(variable); err != nil {
		return nil, err
	}
	return c, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                            list                           ///////////
////////////////////////////////////////////////////////////////////////////////
type cmdList struct {
	path segments
	file *os.File
	scan *bufio.Scanner
}

func (c *cmdList) close() {
	c.scan = nil
	_ = c.file.Close()
}

func (c *cmdList) produce(bg *background) {
	if c.file == nil {
		var err error
		path, err := c.path.compose(bg)
		if err != nil {
			bg.setError("list: " + err.Error())
			return
		}
		c.file, err = os.Open(filepath.Clean(path))
		if err != nil {
			bg.setError("list: " + err.Error())
			return
		}
		c.scan = bufio.NewScanner(c.file)
	}

	if c.scan.Scan() {
		t := c.scan.Text()
		if len(t) > 0 {
			bg.setOutput(t)
		} else {
			bg.setError(EOF)
		}
	} else {
		bg.setError(EOF)
	}
}

func makeList(path string) (command, error) {
	if len(path) == 0 {
		return nil, errors.New("list file path not provided")
	}
	seg, err := makeSegments(path)
	if err != nil {
		return nil, err
	}
	return &cmdList{
		path: seg,
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                             b64                           ///////////
////////////////////////////////////////////////////////////////////////////////

type cmdB64 struct {
	file    bool
	static  bool
	path    segments
	content segments
	encoded string
}

func (c *cmdB64) close() {
	c.content = nil
	c.path = nil
}

func (c *cmdB64) produce(bg *background) {
	var err error
	if len(c.encoded) == 0 {
		encoded := ""
		if c.file {
			path, err := c.path.compose(bg)
			if err != nil {
				bg.setError("b64: " + err.Error())
				return
			}

			if len(path) == 0 {
				bg.setError("cat: file path is empty")
				return
			}

			if f, err := os.Open(filepath.Clean(path)); err != nil {
				bg.setError(err.Error())
				return
			} else {
				if b, err1 := ioutil.ReadAll(f); err1 != nil {
					bg.setError("cat: " + err1.Error())
					_ = f.Close()
					return
				} else {
					encoded = string(b)
				}
				_ = f.Close()
			}
		} else {
			if encoded, err = c.content.compose(bg); err != nil {
				bg.setError("b64: " + err.Error())
				return
			}
		}

		encoded = base64.StdEncoding.EncodeToString([]byte(encoded))
		if c.static {
			c.encoded = encoded
		}
		bg.setOutput(encoded)
	} else {
		bg.setOutput(c.encoded)
	}
}

func makeBase64(v []string) (command, error) {
	content := ""
	path := ""
	file := false
	fs := flag.NewFlagSet("b64", flag.ContinueOnError)
	fs.BoolVar(&file, "f", false, "encode file content to base64")
	err := fs.Parse(v)
	if err != nil {
		return nil, err
	}
	v = fs.Args()
	if len(v) == 0 {
		if file {
			path = "$(" + KeyInput + ")"
		} else {
			content = "$(" + KeyInput + ")"
		}
	} else if len(v) == 1 {
		if file {
			path = v[0]
		} else {
			content = v[0]
		}
	} else {
		return nil, fmt.Errorf("b64 parse error, unknown: %v", v)
	}

	c := &cmdB64{file: file}
	c.path, err = makeSegments(path)
	if err != nil {
		return nil, err
	}
	c.content, err = makeSegments(content)
	if err != nil {
		return nil, err
	}
	c.static = c.path.isStatic() && c.content.isStatic()
	return c, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                          assert                           ///////////
////////////////////////////////////////////////////////////////////////////////
const (
	opIs = iota
	opNot
	opEqual
	opNotEqual
	opGreater
	opGreaterEqual
	opLess
	opLessEqual
)

const (
	isFloat = iota
	isNum
	isStr
)

type cmdAssert struct {
	a, b  segments
	op    int
	float *regexp.Regexp
	num   *regexp.Regexp
}

func (c *cmdAssert) close() {
	c.a = nil
	c.b = nil
}

func (c *cmdAssert) kindOf(s string) int {
	if c.float.MatchString(s) {
		return isFloat
	}
	if c.num.MatchString(s) {
		return isNum
	}
	return isStr
}

const (
	eps = 0.00000001
)

func (c *cmdAssert) doFloat(lhs, rhs string, bg *background) {
	var (
		a, b float64
		err  error
	)
	if a, err = strconv.ParseFloat(lhs, 64); err != nil {
		bg.setError("convert to float fail: " + lhs)
	}
	if b, err = strconv.ParseFloat(rhs, 64); err != nil {
		bg.setError("convert to float fail: " + rhs)
	}

	delta := a - b
	switch c.op {
	case opEqual:
		if delta < -eps || delta > eps {
			bg.setError(fmt.Sprintf("assert fail: %s == %s", lhs, rhs))
		}
	case opNotEqual:
		if delta >= -eps && delta <= eps {
			bg.setError(fmt.Sprintf("assert fail: %s != %s", lhs, rhs))
		}
	case opGreater:
		if delta <= 0 {
			bg.setError(fmt.Sprintf("assert fail: %s > %s", lhs, rhs))
		}
	case opGreaterEqual:
		if delta < 0 {
			bg.setError(fmt.Sprintf("assert fail: %s >= %s", lhs, rhs))
		}
	case opLess:
		if delta >= 0 {
			bg.setError(fmt.Sprintf("assert fail: %s < %s", lhs, rhs))
		}
	case opLessEqual:
		if delta > 0 {
			bg.setError(fmt.Sprintf("assert fail: %s <= %s", lhs, rhs))
		}
	}
}
func (c *cmdAssert) doNum(lhs, rhs string, bg *background) {
	var (
		a, b int
		err  error
	)
	if a, err = strconv.Atoi(lhs); err != nil {
		bg.setError("convert to int fail: " + lhs)
	}
	if b, err = strconv.Atoi(rhs); err != nil {
		bg.setError("convert to int fail: " + rhs)
	}

	delta := a - b
	switch c.op {
	case opEqual:
		if delta != 0 {
			bg.setError(fmt.Sprintf("assert fail: %s == %s", lhs, rhs))
		}
	case opNotEqual:
		if delta == 0 {
			bg.setError(fmt.Sprintf("assert fail: %s != %s", lhs, rhs))
		}
	case opGreater:
		if delta <= 0 {
			bg.setError(fmt.Sprintf("assert fail: %s > %s", lhs, rhs))
		}
	case opGreaterEqual:
		if delta < 0 {
			bg.setError(fmt.Sprintf("assert fail: %s >= %s", lhs, rhs))
		}
	case opLess:
		if delta >= 0 {
			bg.setError(fmt.Sprintf("assert fail: %s < %s", lhs, rhs))
		}
	case opLessEqual:
		if delta > 0 {
			bg.setError(fmt.Sprintf("assert fail: %s <= %s", lhs, rhs))
		}
	}
}
func (c *cmdAssert) doStr(lhs, rhs string, bg *background) {
	if c.op == opEqual {
		if lhs != rhs {
			bg.setError(fmt.Sprintf("assert fail: %s == %s", lhs, rhs))
		}
	} else if c.op == opNotEqual {
		if lhs == rhs {
			bg.setError(fmt.Sprintf("assert fail: %s != %s", lhs, rhs))
		}
	} else {
		bg.setError(fmt.Sprintf("assert not support, op: %d, lhs %s rhs %s", c.op, lhs, rhs))
	}
}
func (c *cmdAssert) produce(bg *background) {
	var (
		a, b string
		err  error
	)
	if a, err = c.a.compose(bg); err != nil {
		bg.setError("assert: " + err.Error())
		return
	}
	if c.op == opIs {
		if a == "1" || a == "true" {
			return
		}
		bg.setError("assert failure: " + a)
		return
	}
	if c.op == opNot {
		if a == "0" || a == "false" {
			return
		}
		bg.setError("assert failure: !" + a)
		return
	}
	if b, err = c.b.compose(bg); err != nil {
		bg.setError("assert: " + err.Error())
		return
	}

	ta, tb := c.kindOf(a), c.kindOf(b)
	if ta == isStr || tb == isStr {
		c.doStr(a, b, bg)
	} else if ta == isFloat || tb == isFloat {
		c.doFloat(a, b, bg)
	} else {
		c.doNum(a, b, bg)
	}

}

func makeAssert(v []string) (command, error) {
	var a string
	var b string
	c := &cmdAssert{}
	if len(v) == 0 {
		return nil, errors.New("assert nothing")
	}
	if v[0] == "!" {
		c.op = opNot
		if len(v) > 2 {
			return nil, errors.New("assert ! variable, but more comes")
		}
		a = v[1]
	} else if len(v) == 1 {
		a = v[0]
		if a[0] == '!' {
			c.op = opNot
			a = a[1:]
		} else {
			c.op = opIs
		}
	} else if len(v) == 3 {
		a, b = v[0], v[2]
		switch v[1] {
		case "==":
			c.op = opEqual
		case "!=":
			c.op = opNotEqual
		case ">":
			c.op = opGreater
		case ">=":
			c.op = opGreaterEqual
		case "<":
			c.op = opLess
		case "<=":
			c.op = opLessEqual
		default:
			return nil, errors.New("invalid operator " + v[1])
		}
	} else {
		return nil, errors.New("assert expect expr as <a op b>")
	}

	var err error
	if c.a, err = makeSegments(a); err != nil {
		return nil, err
	}
	if c.b, err = makeSegments(b); err != nil {
		return nil, err
	}

	if c.float, err = regexp.Compile("^-?[0-9]+\\.[0-9]*$"); err != nil {
		return nil, err
	}
	if c.num, err = regexp.Compile("^-?[0-9]+$"); err != nil {
		return nil, err
	}
	return c, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                             json                          ///////////
////////////////////////////////////////////////////////////////////////////////

type cmdJson struct {
	path    segments
	content segments
	exist   bool
	numer   bool
}

func (c *cmdJson) close() {
	c.content = nil
	c.path = nil
}

func (c *cmdJson) find(content string, path string) (interface{}, error) {
	if len(path) == 0 {
		return nil, errors.New("invalid json path: " + path)
	}
	segs := strings.Split(path, ".")

	var value interface{}
	if err := json.Unmarshal([]byte(content), &value); err != nil {
		return nil, err
	}

	rete := errors.New(path + " not found")
	for i, key := range segs {
		if len(key) == 0 {
			if i > 0 && i < len(segs)-1 {
				continue
			}
			if i > 0 {
				return value, nil
			}
		}

		r := []rune(key)
		switch c := value.(type) {
		case []interface{}:
			if r[0] != '[' || r[len(r)-1] != ']' {
				return nil, errors.New("expect json list path")
			}
			x := string(r[1 : len(r)-1])
			if x == "" {
				// key value not change, to support "[]".
			} else {
				n, err := strconv.ParseInt(x, 10, 32)
				if err != nil {
					return nil, err
				}
				if int(n) >= len(c) {
					return nil, errors.New("json index " + x + " overflow")
				}

				value = c[n]
			}
		case map[string]interface{}:
			if len(key) == 0 {
				continue
			}
			if v, ok := c[key]; ok {
				value = v
			} else {
				return nil, rete
			}
		default:
			return nil, rete

		}

	}

	return value, nil
}
func (c *cmdJson) produce(bg *background) {
	content, err := c.content.compose(bg)
	if err != nil {
		bg.setError("json: " + err.Error())
		return
	}
	// do not check content here
	path, err := c.path.compose(bg)
	if err != nil {
		bg.setError("json: " + err.Error())
		return
	}
	if len(path) == 0 {
		bg.setError("json: empty path")
		return
	}

	v, err := c.find(content, path)

	if c.numer {
		if err != nil {
			bg.setOutput("0")
			return
		}
		if c, ok := v.([]interface{}); !ok {
			bg.setError("json path is not a list")
		} else {
			bg.setOutput(strconv.Itoa(len(c)))
		}
		return
	}

	if c.exist {
		if err != nil {
			bg.setError("json: " + err.Error())
			return
		}
	} else {
		if err != nil {
			bg.setOutput("")
			return
		}
	}

	switch c := v.(type) {
	case bool:
		if c {
			bg.setOutput("1")
		} else {
			bg.setOutput("0")
		}
	case float64:
		bg.setOutput(strconv.FormatFloat(c, 'f', 8, 64))
	case string:
		bg.setOutput(c)
	case json.Number:
		bg.setOutput(string(c))
	case []interface{}:
		if b, err := json.Marshal(c); err != nil {
			bg.setError("json marshal: " + err.Error())
		} else {
			bg.setOutput(string(b))
		}
	case map[string]interface{}:
		if b, err := json.Marshal(c); err != nil {
			bg.setError("json marshal: " + err.Error())
		} else {
			bg.setOutput(string(b))
		}
	}
}

func makeJson(v []string) (command, error) {
	content := "$(INPUT)"
	c := &cmdJson{}

	fs := flag.NewFlagSet("json", flag.ContinueOnError)
	fs.BoolVar(&c.exist, "e", false, "check if path exists")
	fs.BoolVar(&c.numer, "n", false, "get number of list item")
	err := fs.Parse(v)
	if err != nil {
		return nil, err
	}
	if c.exist && c.numer {
		return nil, fmt.Errorf("json can not take both -n and -e option")
	}
	v = fs.Args()
	if len(v) > 2 {
		return nil, fmt.Errorf("json [-n] [-e] <path> [<content>]")
	}
	if len(v) < 1 {
		return nil, fmt.Errorf("json path not specified")
	}
	path := v[0]
	if c.path, err = makeSegments(path); err != nil {
		return nil, err
	}

	if len(v) == 2 {
		content = v[1]
	}

	if c.content, err = makeSegments(content); err != nil {
		return nil, err
	}
	return c, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                             lua                           ///////////
////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////
//////////                          pipeline                         ///////////
////////////////////////////////////////////////////////////////////////////////
type pipeline []command

func (p pipeline) produce(bg *background) {
	for _, c := range p {
		bg.setInput(bg.getOutput())
		c.produce(bg)
		if bg.getError() != "" {
			return
		}
	}
}
func (p pipeline) close() {
	for _, c := range p {
		c.close()
	}
}

func parse(str string) (command, error) {
	args, err := argv.Argv(str, nil, func(s string) (string, error) {
		return s, nil
	})
	if err != nil {
		return nil, err
	}

	var pp pipeline
	for _, v := range args {
		if len(v) == 0 {
			continue
		}

		switch v[0] {
		case "echo":
			content := ""
			if len(v) == 2 {
				content = v[1]
			} else if len(v) > 2 {
				return nil, fmt.Errorf("echo invalid: %v", v)
			}
			if cmd, err := makeEcho(content); err != nil {
				return nil, err
			} else {
				pp = append(pp, cmd)
			}
		case "cat":
			path := ""
			if len(v) == 2 {
				path = v[1]
			} else if len(v) > 2 {
				return nil, fmt.Errorf("cat invalid: %v", v)
			}
			if cmd, err := makeCat(path); err != nil {
				return nil, err
			} else {
				pp = append(pp, cmd)
			}

		case "envw":
			cmd, err := makeEnvw(v[1:])
			if err != nil {
				return nil, err
			} else {
				pp = append(pp, cmd)
			}
		case "envd":
			cmd, err := makeEnvd(v[1:])
			if err != nil {
				return nil, err
			} else {
				pp = append(pp, cmd)
			}
		case "write":
			cmd, err := makeWrite(v[1:])
			if err != nil {
				return nil, err
			} else {
				pp = append(pp, cmd)
			}
		case "assert":
			cmd, err := makeAssert(v[1:])
			if err != nil {
				return nil, err
			} else {
				pp = append(pp, cmd)
			}
		case "json":
			cmd, err := makeJson(v[1:])
			if err != nil {
				return nil, err
			} else {
				pp = append(pp, cmd)
			}
		case "list":
			if len(v) != 2 {
				return nil, fmt.Errorf("list invalid: %v", v)
			}
			if cmd, err := makeList(v[1]); err != nil {
				return nil, err
			} else {
				pp = append(pp, cmd)
			}
		case "b64":
			cmd, err := makeBase64(v[1:])
			if err != nil {
				return nil, err
			} else {
				pp = append(pp, cmd)
			}
		}
	}

	return pp, nil
}

const (
	phaseCmd = iota
	phaseEnv
	phaseLocal
	phaseGlobal
	phaseString
)

func makeSegments(str string) (segments, error) {
	r := []rune(str)
	start := 0
	phase := phaseString
	var segs segments

	for i, c := range r {
		old := phase
		switch phase {
		case phaseString:
			if c == '$' {
				phase = phaseEnv
			} else if c == '`' {
				phase = phaseCmd
			}
		case phaseCmd:
			if c == '`' {
				phase = phaseString
			}
		case phaseEnv:
			if c == '(' {
				phase = phaseLocal
			} else if c == '{' {
				phase = phaseGlobal
			} else {
				return nil, errors.New("expect '(' or '{' after '$'")
			}
		case phaseLocal:
			if c == ')' {
				phase = phaseString
			}
		case phaseGlobal:
			if c == '}' {
				phase = phaseString
			}
		}

		if old != phase {
			switch old {
			case phaseString:
				if i > start {
					segs = append(segs, staticSegment(r[start:i]))
				}
			case phaseCmd:
				if i-start > 1 {
					cmd, err := parse(string(r[start:i]))
					if err != nil {
						return nil, err
					}
					segs = append(segs, &dynamicSegment{f: func(bg *background) (string, error) {
						cmd.produce(bg)
						errStr := bg.getError()
						if len(errStr) > 0 {
							return "", errors.New(errStr)
						}
						return bg.getOutput(), nil
					}})
				}
			case phaseEnv:
			case phaseLocal:
				if i-start > 1 {
					name := string(r[start:i])
					if len(name) == 0 {
						return nil, errors.New("local variable name is missing")
					}
					segs = append(segs, &dynamicSegment{f: func(bg *background) (string, error) {
						return bg.getLocalEnv(name), nil
					}})
				}
			case phaseGlobal:
				if i-start > 1 {
					name := string(r[start:i])
					if len(name) == 0 {
						return nil, errors.New("global variable name is missing")
					}
					segs = append(segs, &dynamicSegment{f: func(bg *background) (string, error) {
						return bg.getGlobalEnv(name), nil
					}})
				}
			}

			start = i + 1
		}
	}

	if phase != phaseString {
		return nil, fmt.Errorf("parse finish with phase %d", phase)
	}
	if len(r) > start {
		segs = append(segs, staticSegment(r[start:]))
	}

	return segs, nil
}
