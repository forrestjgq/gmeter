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

	"github.com/golang/glog"

	"github.com/forrestjgq/gmeter/internal/argv"
)

type command interface {
	iterable() bool
	produce(bg *background)
	close()
}

////////////////////////////////////////////////////////////////////////////////
//////////                          segments                         ///////////
////////////////////////////////////////////////////////////////////////////////

type segment interface {
	iterable() bool
	getString(bg *background) (string, error)
}
type staticSegment string

func (ss staticSegment) iterable() bool {
	return false
}
func (ss staticSegment) getString(bg *background) (string, error) {
	return string(ss), nil
}

type dynamicSegment struct {
	isIterable bool
	f          func(bg *background) (string, error)
}

func (ds dynamicSegment) iterable() bool {
	return ds.isIterable
}
func (ds dynamicSegment) getString(bg *background) (string, error) {
	return ds.f(bg)
}

type segments []segment

func (s segments) iterable() bool {
	for _, seg := range s {
		if seg.iterable() {
			return true
		}
	}
	return false
}
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

	killFirstQuote := false
	for i, s := range arr {
		if len(s) == 0 {
			continue
		}
		if s[0] == '`' && s[len(s)-1] == '`' {
			arr[i] = s[1 : len(s)-1]
			if i > 0 {
				prev := arr[i-1]
				if len(prev) > 0 && prev[len(prev)-1] == '"' {
					arr[i-1] = prev[:len(prev)-1]
				}
			}
			killFirstQuote = true
			continue
		}
		if killFirstQuote && s[0] == '"' {
			arr[i] = s[1:]
		}
		killFirstQuote = false
	}
	return strings.Join(arr, ""), nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                             cvt                           ///////////
////////////////////////////////////////////////////////////////////////////////

type cmdCvt struct {
	raw     string
	content segments
	toInt   bool
	toFloat bool
	toBool  bool
	toRaw   bool
	exp     *regexp.Regexp
}

func (c *cmdCvt) iterable() bool {
	return false
}

func (c *cmdCvt) close() {
	c.content = nil
}

func (c *cmdCvt) produce(bg *background) {
	if content, err := c.content.compose(bg); err != nil {
		bg.setErrorf("cvt(%s) compose fail: %v", c.raw, err)
	} else {
		if c.toBool {
			if content == "0" || content == "false" {
				bg.setOutput("`false`")
			} else if content == "1" || content == "true" {
				bg.setOutput("`true`")
			} else {
				bg.setErrorf("cvt(%s) convert to bool fail: %s", c.raw, content)
			}
		} else if c.toFloat {
			if !c.exp.MatchString(content) {
				bg.setErrorf("cvt(%s) convert to number fail: %s", c.raw, content)
			} else {
				bg.setOutput("`" + content + "`")
			}
		} else if c.toInt {
			if !c.exp.MatchString(content) {
				bg.setErrorf("cvt(%s) convert to number fail: %s", c.raw, content)
			} else {
				idx := strings.Index(content, ".")
				if idx >= 0 {
					content = content[:idx]
				}
				bg.setOutput("`" + content + "`")
			}
		} else if c.toRaw {
			bg.setOutput("`" + content + "`")
		} else {
			bg.setOutput(content)
		}
	}
}

func makeCvt(v []string) (command, error) {
	raw := strings.Join(v, " ")

	fs := flag.NewFlagSet("cvt", flag.ContinueOnError)
	boolVal := false
	floatVal := false
	intVal := false
	rawVal := false
	fs.BoolVar(&boolVal, "b", false, "convert to bool")
	fs.BoolVar(&floatVal, "f", false, "convert to float number")
	fs.BoolVar(&intVal, "i", false, "convert to integer number")
	fs.BoolVar(&rawVal, "r", false, "convert to raw string(to strip quotes)")

	err := fs.Parse(v)
	if err != nil {
		return nil, err
	}

	content := "$(" + KeyInput + ")"

	v = fs.Args()
	if len(v) == 1 {
		content = v[0]
	} else if len(v) > 1 {
		return nil, fmt.Errorf("cvt(%s) invalid args", raw)
	}

	seg, err := makeSegments(content)
	if err != nil {
		return nil, err
	}
	c := &cmdCvt{
		raw:     raw,
		content: seg,
		toBool:  boolVal,
		toFloat: floatVal,
		toInt:   intVal,
		toRaw:   rawVal,
	}
	if intVal {
		exp, err := regexp.Compile(`^-?[0-9]+(\.0*)?$`)
		if err != nil {
			glog.Fatalf("compile number expr fail, err %v", err)
		}
		c.exp = exp
	}
	if floatVal {
		exp, err := regexp.Compile(`^-?[0-9.]+(e-?[0-9]+)?$`)
		if err != nil {
			glog.Fatalf("compile number expr fail, err %v", err)
		}
		c.exp = exp
	}
	return c, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                            escape                         ///////////
////////////////////////////////////////////////////////////////////////////////

type cmdEscape struct {
	raw     string
	content segments
}

func (c *cmdEscape) iterable() bool {
	return false
}

func (c *cmdEscape) close() {
	c.content = nil
}

func (c *cmdEscape) produce(bg *background) {
	content := ""
	var err error
	if content, err = c.content.compose(bg); err != nil {
		bg.setErrorf("escape %s compose content fail: %v", c.raw, err)
	}
	if len(content) > 0 {
		content = strings.ReplaceAll(content, "\"", "\\\"")
	}
	bg.setOutput(content)
}

func makeEscape(v []string) (command, error) {
	raw := strings.Join(v, " ")
	c := &cmdEscape{
		raw: raw,
	}
	content := "$(" + KeyInput + ")"
	if len(v) == 1 {
		content = v[0]
	} else if len(v) > 1 {
		return nil, fmt.Errorf("escape(%s) invalid args", raw)
	}

	var err error
	if c.content, err = makeSegments(content); err != nil {
		return nil, err
	}
	return c, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                           strrepl                        ///////////
////////////////////////////////////////////////////////////////////////////////

type cmdStrRepl struct {
	raw       string
	content   segments
	substring segments
	newstring segments
}

func (c *cmdStrRepl) iterable() bool {
	return false
}

func (c *cmdStrRepl) close() {
	c.content = nil
	c.newstring = nil
}

func (c *cmdStrRepl) produce(bg *background) {
	content := ""
	newstring := ""
	substring := ""
	var err error
	if content, err = c.content.compose(bg); err != nil {
		bg.setErrorf("strrepl %s compose content fail: %v", c.raw, err)
	}
	if substring, err = c.substring.compose(bg); err != nil {
		bg.setErrorf("strrepl %s compose substring fail: %v", c.raw, err)
	}
	if newstring, err = c.newstring.compose(bg); err != nil {
		bg.setErrorf("strrepl %s compose newstring fail: %v", c.raw, err)
	}
	if len(content) > 0 && len(substring) > 0 {
		content = strings.ReplaceAll(content, substring, newstring)
	}

	bg.setOutput(content)
}

func makeStrRepl(v []string) (command, error) {
	raw := strings.Join(v, " ")
	c := &cmdStrRepl{
		raw: raw,
	}
	if len(v) >= 2 {
		var err error
		if c.content, err = makeSegments(v[0]); err != nil {
			return nil, err
		}
		if c.substring, err = makeSegments(v[1]); err != nil {
			return nil, err
		}
		if len(v) > 2 {
			if c.newstring, err = makeSegments(v[2]); err != nil {
				return nil, err
			}
		} else {
			if c.newstring, err = makeSegments(""); err != nil {
				return nil, err
			}
		}

		return c, nil
	}
	return nil, fmt.Errorf("strrepl(%s) invalid args", raw)
}

////////////////////////////////////////////////////////////////////////////////
//////////                             echo                          ///////////
////////////////////////////////////////////////////////////////////////////////

type cmdEcho struct {
	raw     string
	content segments
}

func (c *cmdEcho) iterable() bool {
	return false
}

func (c *cmdEcho) close() {
	c.content = nil
}

func (c *cmdEcho) produce(bg *background) {
	if content, err := c.content.compose(bg); err != nil {
		bg.setErrorf("echo %s compose fail: %v", c.raw, err)
	} else {
		bg.setOutput(content)
	}
}

func makeEcho(v []string) (command, error) {
	raw := strings.Join(v, " ")
	content := "$(" + KeyInput + ")"

	if len(v) == 1 {
		content = v[0]
	} else if len(v) > 1 {
		return nil, fmt.Errorf("echo(%s) invalid args", raw)
	}

	seg, err := makeSegments(content)
	if err != nil {
		return nil, err
	}

	return &cmdEcho{
		raw:     raw,
		content: seg,
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                             cat                           ///////////
////////////////////////////////////////////////////////////////////////////////

type cmdCat struct {
	static  bool
	path    segments
	content []byte
	raw     string
}

func (c *cmdCat) iterable() bool {
	return false
}

func (c *cmdCat) close() {
	c.content = nil
}

func (c *cmdCat) produce(bg *background) {
	if len(c.content) == 0 {
		path, err := c.path.compose(bg)
		if err != nil {
			bg.setErrorf("cat(%s) compose path fail, error: %v", c.raw, err)
			return
		}

		if f, err := os.Open(filepath.Clean(path)); err != nil {
			bg.setErrorf("cat(%s) %s: %v", c.raw, path, err)
		} else {
			if b, err1 := ioutil.ReadAll(f); err1 != nil {
				bg.setErrorf("cat(%s) read file %s fail: %v", c.raw, path, err1)
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

func makeCat(v []string) (command, error) {
	raw := strings.Join(v, " ")

	path := "$(" + KeyInput + ")"
	if len(v) == 1 {
		path = v[0]
	} else if len(v) > 1 {
		return nil, fmt.Errorf("cat invalid: %v", v)
	}
	seg, err := makeSegments(path)
	if err != nil {
		return nil, err
	}

	return &cmdCat{
		raw:    raw,
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
	raw     string
}

func (c *cmdWrite) iterable() bool {
	return false
}
func (c *cmdWrite) close() {
	c.content = nil
	c.path = nil
}

func (c *cmdWrite) produce(bg *background) {
	content, err := c.content.compose(bg)
	if err != nil {
		bg.setErrorf("write(%s) command compose content fail: %v", c.raw, err)
		return
	}
	// do not check content here
	path, err := c.path.compose(bg)
	if err != nil {
		bg.setErrorf("write(%s) compose path fail: %v", c.raw, err)
		return
	}
	if len(path) == 0 {
		bg.setErrorf("write(%s) compose empty file path", c.raw)
		return
	}

	if f, err := os.Create(filepath.Clean(path)); err != nil {
		bg.setErrorf("write(%s) create file %s fail: %v", c.raw, path, err)
	} else {
		if _, err1 := f.WriteString(content); err1 != nil {
			bg.setErrorf("write(%s) write file %s fail: %v", c.raw, path, err1)
		}
		_ = f.Close()
	}
}

func makeWrite(v []string) (command, error) {
	raw := strings.Join(v, " ")
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
	c := &cmdWrite{raw: raw}
	if c.path, err = makeSegments(path); err != nil {
		return nil, err
	}
	if c.content, err = makeSegments(content); err != nil {
		return nil, err
	}
	return c, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                             if                            ///////////
////////////////////////////////////////////////////////////////////////////////

// if <condition> then <cmd> [else <cmd>]
type cmdIf struct {
	raw       string
	condition command
	cthen     command
	celse     command
}

func (c *cmdIf) iterable() bool {
	return false
}
func (c *cmdIf) close() {
	if c.condition != nil {
		c.cthen.close()
		c.condition.close()
	}
	if c.cthen != nil {
		c.cthen = nil
		c.cthen = nil
	}
	if c.celse != nil {
		c.celse.close()
		c.celse = nil
	}
}

func (c *cmdIf) produce(bg *background) {
	c.condition.produce(bg)
	err := bg.getError()
	bg.setError("")
	if len(err) == 0 {
		c.cthen.produce(bg)
	} else if c.celse != nil {
		c.celse.produce(bg)
	}
}
func makeIf(v []string) (command, error) {
	c := &cmdIf{
		raw: strings.Join(v, " "),
	}
	var cmdCondition []string
	var cmdThen []string
	var cmdElse []string

	thenIdx := len(v)
	elseIdx := len(v)
	for i := 0; i < len(v); i++ {
		if v[i] == "then" {
			thenIdx = i
			break
		}
	}
	if thenIdx == len(v) || thenIdx <= 0 {
		return nil, fmt.Errorf("if(%s): then not found", c.raw)
	}
	cmdCondition = v[:thenIdx]
	for i := thenIdx + 1; i < len(v); i++ {
		if v[i] == "else" {
			elseIdx = i
			break
		}
	}
	if elseIdx == thenIdx+1 {
		return nil, fmt.Errorf("if(%s): then command not found", c.raw)
	}
	cmdThen = v[thenIdx+1 : elseIdx]
	if elseIdx < len(v)-1 {
		cmdElse = v[elseIdx+1:]
	}

	if len(cmdCondition) == 0 || len(cmdThen) == 0 {
		return nil, fmt.Errorf("if(%s): invalid if clause", c.raw)
	}

	var err error
	c.condition, err = makeAssert(cmdCondition)
	if err != nil {
		return nil, fmt.Errorf("if(%s): parse condition fail, err: %v", c.raw, err)
	}
	c.cthen, err = parseCmdArgs(cmdThen)
	if err != nil {
		return nil, fmt.Errorf("if(%s): parse then clause fail, err: %v", c.raw, err)
	}
	if len(cmdElse) > 0 {
		c.celse, err = parseCmdArgs(cmdElse)
		if err != nil {
			return nil, fmt.Errorf("if(%s): parse else clause fail, err: %v", c.raw, err)
		}
	}

	return c, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                           print                           ///////////
////////////////////////////////////////////////////////////////////////////////

type cmdPrint struct {
	raw    string
	format segments
}

func (c *cmdPrint) iterable() bool {
	return false
}
func (c *cmdPrint) close() {
	c.format = nil
}

func (c *cmdPrint) produce(bg *background) {
	if c.format != nil {
		content, err := c.format.compose(bg)
		if err != nil {
			bg.setErrorf("print(%s) compose fail: %v", c.raw, err)
			return
		}
		fmt.Print(content)
	}
}

func makePrint(v []string) (command, error) {
	raw := strings.Join(v, " ")
	if len(v) != 1 {
		return nil, errors.New("print requires a content argument")
	}
	if format, err := makeSegments(v[0]); err != nil {
		return nil, err
	} else {
		return &cmdPrint{
			raw:    raw,
			format: format,
		}, nil
	}

}

////////////////////////////////////////////////////////////////////////////////
//////////                           report                          ///////////
////////////////////////////////////////////////////////////////////////////////

type cmdReport struct {
	raw      string
	format   segments
	template bool
	newline  bool
}

func (c *cmdReport) iterable() bool {
	return false
}
func (c *cmdReport) close() {
	c.format = nil
}

func (c *cmdReport) produce(bg *background) {
	if c.format != nil {
		content, err := c.format.compose(bg)
		if err != nil {
			bg.setErrorf("report(%s) compose fail: %v", c.raw, err)
			return
		}
		if c.template {
			bg.reportTemplate(content, c.newline)
		} else {
			bg.report(content, c.newline)
		}
	} else {
		bg.reportDefault(c.newline)
	}

}

func makeReport(v []string) (command, error) {
	raw := strings.Join(v, " ")
	c := &cmdReport{
		raw: raw,
	}

	format := ""
	template := ""
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	fs.StringVar(&format, "f", "", "format to report, use predefined if not present")
	fs.StringVar(&template, "t", "", "format from template")
	fs.BoolVar(&c.newline, "n", false, "append new line in the end")
	err := fs.Parse(v)
	if err != nil {
		return nil, err
	}

	if len(format) > 0 {
		if c.format, err = makeSegments(format); err != nil {
			return nil, err
		}
	} else if len(template) > 0 {
		if c.format, err = makeSegments(template); err != nil {
			return nil, err
		}
		c.template = true
	}
	return c, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                            env                            ///////////
////////////////////////////////////////////////////////////////////////////////

const (
	envWrite = iota
	envDelete
	envMove
)

type cmdEnv struct {
	op          int
	variable    segments
	dstVariable segments
	value       segments
	raw         string
}

func (c *cmdEnv) iterable() bool {
	return false
}
func (c *cmdEnv) close() {
	c.variable = nil
	c.value = nil
}

func (c *cmdEnv) produce(bg *background) {
	variable, err := c.variable.compose(bg)
	if err != nil {
		bg.setErrorf("env(%s) compose variable fail: %v", c.raw, err)
		return
	}
	if c.op == envDelete {
		bg.delLocalEnv(variable)
	} else if c.op == envWrite {
		value, err := c.value.compose(bg)
		if err != nil {
			bg.setErrorf("env(%s) compose value fail: %v", c.raw, err)
			return
		}
		bg.setLocalEnv(variable, value)
	} else if c.op == envMove {
		dst, err := c.dstVariable.compose(bg)
		if err != nil {
			bg.setErrorf("env(%s) compose value fail: %v", c.raw, err)
			return
		}
		bg.setLocalEnv(dst, bg.getLocalEnv(variable))
		bg.delLocalEnv(variable)
	} else {
		bg.setErrorf("env(%s): unknown operator %d", c.raw, c.op)
	}
}

func makeEnvw(v []string) (command, error) {
	raw := strings.Join(v, " ")
	content := ""
	fs := flag.NewFlagSet("envw", flag.ContinueOnError)
	fs.StringVar(&content, "c", "$(INPUT)", "content to write to local environment, default using local input")
	err := fs.Parse(v)
	if err != nil {
		return nil, err
	}
	v = fs.Args()
	if len(v) != 1 {
		return nil, fmt.Errorf("envw variable not provided")
	}
	variable := v[0]
	c := &cmdEnv{
		raw: raw,
		op:  envWrite,
	}
	if c.variable, err = makeSegments(variable); err != nil {
		return nil, err
	}
	if c.value, err = makeSegments(content); err != nil {
		return nil, err
	}
	return c, nil
}
func makeEnvMv(v []string) (command, error) {
	raw := strings.Join(v, " ")
	if len(v) != 2 {
		return nil, fmt.Errorf("envw variable not provided")
	}
	c := &cmdEnv{
		raw: raw,
		op:  envMove,
	}
	var err error
	if c.variable, err = makeSegments(v[0]); err != nil {
		return nil, err
	}
	if c.dstVariable, err = makeSegments(v[1]); err != nil {
		return nil, err
	}
	return c, nil
}
func makeEnvd(v []string) (command, error) {
	raw := strings.Join(v, " ")
	variable := v[0]
	c := &cmdEnv{op: envDelete, raw: raw}
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
	raw  string
}

func (c *cmdList) iterable() bool {
	return true
}
func (c *cmdList) close() {
	c.scan = nil
	if c.file != nil {
		_ = c.file.Close()
	}
}

func (c *cmdList) produce(bg *background) {
	if c.file == nil {
		var err error
		path, err := c.path.compose(bg)
		if err != nil {
			bg.setErrorf("list(%s) compose path fail: %v", c.raw, err)
			return
		}
		c.file, err = os.Open(filepath.Clean(path))
		if err != nil {
			bg.setErrorf("list(%s) open file %s fail: %v", c.raw, path, err)
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
			c.close()
		}
	} else {
		bg.setError(EOF)
		c.close()
	}
}

func makeList(v []string) (command, error) {
	if len(v) != 1 {
		return nil, fmt.Errorf("list invalid: %v", v)
	}
	path := v[0]
	if len(path) == 0 {
		return nil, errors.New("list file path not provided")
	}
	seg, err := makeSegments(path)
	if err != nil {
		return nil, err
	}
	return &cmdList{
		raw:  strings.Join(v, " "),
		path: seg,
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                             b64                           ///////////
////////////////////////////////////////////////////////////////////////////////

type cmdB64 struct {
	raw     string
	file    bool
	static  bool
	path    segments
	content segments
	encoded string
}

func (c *cmdB64) iterable() bool {
	return false
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
				bg.setErrorf("b64(%s) compose path fail: %v", c.raw, err)
				return
			}

			if len(path) == 0 {
				bg.setErrorf("b64(%s): file path is empty", c.raw)
				return
			}

			if f, err := os.Open(filepath.Clean(path)); err != nil {
				bg.setErrorf("b64(%s) open file %s fail: %v", c.raw, path, err)
				return
			} else {
				if b, err1 := ioutil.ReadAll(f); err1 != nil {
					bg.setErrorf("b64(%s) read file %s fail: %v", c.raw, path, err)
					_ = f.Close()
					return
				} else {
					encoded = string(b)
				}
				_ = f.Close()
			}
		} else {
			if encoded, err = c.content.compose(bg); err != nil {
				bg.setErrorf("b64(%s) compose content fail: %v", c.raw, err)
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
	c := &cmdB64{
		raw: strings.Join(v, " "),
	}
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

	c.file = file
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
	raw   string
	a, b  segments
	op    int
	float *regexp.Regexp
	num   *regexp.Regexp
}

func (c *cmdAssert) iterable() bool {
	return false
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

func (c *cmdAssert) doFloat(lhs, rhs string, bg *background) string {
	var (
		a, b float64
		err  error
	)
	if a, err = strconv.ParseFloat(lhs, 64); err != nil {
		return "convert to float fail: " + lhs
	}
	if b, err = strconv.ParseFloat(rhs, 64); err != nil {
		return "convert to float fail: " + rhs
	}

	delta := a - b
	switch c.op {
	case opEqual:
		if delta < -eps || delta > eps {
			return fmt.Sprintf("assert fail: %s == %s", lhs, rhs)
		}
	case opNotEqual:
		if delta >= -eps && delta <= eps {
			return fmt.Sprintf("assert fail: %s != %s", lhs, rhs)
		}
	case opGreater:
		if delta <= 0 {
			return fmt.Sprintf("assert fail: %s > %s", lhs, rhs)
		}
	case opGreaterEqual:
		if delta < 0 {
			return fmt.Sprintf("assert fail: %s >= %s", lhs, rhs)
		}
	case opLess:
		if delta >= 0 {
			return fmt.Sprintf("assert fail: %s < %s", lhs, rhs)
		}
	case opLessEqual:
		if delta > 0 {
			return fmt.Sprintf("assert fail: %s <= %s", lhs, rhs)
		}
	default:
		return fmt.Sprintf("assert(%s): unknown operator %d", c.raw, c.op)
	}
	return ""
}
func (c *cmdAssert) doNum(lhs, rhs string, bg *background) string {
	var (
		a, b int
		err  error
	)
	if a, err = strconv.Atoi(lhs); err != nil {
		return "convert to int fail: " + lhs
	}
	if b, err = strconv.Atoi(rhs); err != nil {
		return "convert to int fail: " + rhs
	}

	delta := a - b
	switch c.op {
	case opEqual:
		if delta != 0 {
			return fmt.Sprintf("assert fail: %s == %s", lhs, rhs)
		}
	case opNotEqual:
		if delta == 0 {
			return fmt.Sprintf("assert fail: %s != %s", lhs, rhs)
		}
	case opGreater:
		if delta <= 0 {
			return fmt.Sprintf("assert fail: %s > %s", lhs, rhs)
		}
	case opGreaterEqual:
		if delta < 0 {
			return fmt.Sprintf("assert fail: %s >= %s", lhs, rhs)
		}
	case opLess:
		if delta >= 0 {
			return fmt.Sprintf("assert fail: %s < %s", lhs, rhs)
		}
	case opLessEqual:
		if delta > 0 {
			return fmt.Sprintf("assert fail: %s <= %s", lhs, rhs)
		}
	default:
		return fmt.Sprintf("assert(%s): unknown operator %d", c.raw, c.op)
	}
	return ""
}
func (c *cmdAssert) doStr(lhs, rhs string, bg *background) string {
	if c.op == opEqual {
		if lhs != rhs {
			return fmt.Sprintf("assert fail: %s == %s", lhs, rhs)
		}
	} else if c.op == opNotEqual {
		if lhs == rhs {
			return fmt.Sprintf("assert fail: %s != %s", lhs, rhs)
		}
	} else {
		return fmt.Sprintf("assert not support, op: %d, lhs %s rhs %s", c.op, lhs, rhs)
	}
	return ""
}
func (c *cmdAssert) judge(bg *background) string {
	var (
		a, b string
		err  error
	)
	if a, err = c.a.compose(bg); err != nil {
		return fmt.Sprintf("assert(%s) compose lhs fail: %v", c.raw, err)
	}
	if c.op == opIs {
		if a == "1" || a == "true" {
			return ""
		}
		bg.setError("assert failure: " + a)
		return ""
	}
	if c.op == opNot {
		if a == "0" || a == "false" || a == "" {
			return ""
		}
		return "assert failure: !" + a
	}
	if b, err = c.b.compose(bg); err != nil {
		return fmt.Sprintf("assert(%s) compose rhs fail: %v", c.raw, err)
	}

	ta, tb := c.kindOf(a), c.kindOf(b)
	if ta == isStr || tb == isStr {
		return c.doStr(a, b, bg)
	} else if ta == isFloat || tb == isFloat {
		return c.doFloat(a, b, bg)
	} else {
		return c.doNum(a, b, bg)
	}

}
func (c *cmdAssert) produce(bg *background) {
	err := c.judge(bg)
	if len(err) != 0 {
		bg.setError(err)
	}
}

func makeAssert(v []string) (command, error) {
	var a string
	var b string
	c := &cmdAssert{
		raw: strings.Join(v, " "),
	}
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

	if c.float, err = regexp.Compile(`^-?[0-9]+\.[0-9]*$`); err != nil {
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

func (c *cmdJson) iterable() bool {
	return false
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

func (p pipeline) iterable() bool {
	for _, cmd := range p {
		if cmd.iterable() {
			return true
		}
	}
	return false
}
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

func parseCmdArgs(args []string) (command, error) {
	name := args[0]
	args = args[1:]
	switch name {
	case "echo":
		return makeEcho(args)
	case "cat":
		return makeCat(args)
	case "envw":
		return makeEnvw(args)
	case "envmv":
		return makeEnvMv(args)
	case "envd":
		return makeEnvd(args)
	case "write":
		return makeWrite(args)
	case "assert":
		return makeAssert(args)
	case "json":
		return makeJson(args)
	case "list":
		return makeList(args)
	case "b64":
		return makeBase64(args)
	case "cvt":
		return makeCvt(args)
	case "if":
		return makeIf(args)
	case "report":
		return makeReport(args)
	case "print":
		return makePrint(args)
	case "strrepl":
		return makeStrRepl(args)
	case "escape":
		return makeEscape(args)
	default:
		return nil, fmt.Errorf("cmd %s not supported", name)
	}
}
func parse(str string) (command, error) {
	args, err := argv.Argv(str, nil, func(s string) (string, error) {
		return s, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse %s fail, err: %v", str, err)
	}

	var pp pipeline
	for _, v := range args {
		if len(v) == 0 {
			continue
		}
		var cmd command

		cmd, err = parseCmdArgs(v)
		if err != nil {
			return nil, fmt.Errorf("parse %s fail, err: %v", str, err)
		} else {
			pp = append(pp, cmd)
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

// makeSegments creates a segment list which will create a string eventually.
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
				if i > start {
					cmd, err := parse(string(r[start:i]))
					if err != nil {
						return nil, err
					}
					seg := &dynamicSegment{f: func(bg *background) (string, error) {
						cmd.produce(bg)
						errStr := bg.getError()
						if len(errStr) > 0 {
							return "", errors.New(errStr)
						}
						return bg.getOutput(), nil
					}}

					seg.isIterable = cmd.iterable()
					segs = append(segs, seg)
				}
			case phaseEnv:
			case phaseLocal:
				if i > start {
					name := string(r[start:i])
					if len(name) == 0 {
						return nil, errors.New("local variable name is missing")
					}
					segs = append(segs, &dynamicSegment{f: func(bg *background) (string, error) {
						return bg.getLocalEnv(name), nil
					}})
				}
			case phaseGlobal:
				if i > start {
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

type group struct {
	segs        []segments
	ignoreError bool
	isIterable  bool
}

func (g *group) compose(bg *background) (string, error) {
	for _, seg := range g.segs {
		bg.setInput(bg.getOutput())
		s, err := seg.compose(bg)
		if err != nil {
			if g.ignoreError {
				bg.setOutput("")
				bg.setError("")
			} else {
				return "", err
			}
		} else {
			bg.setOutput(s)
		}
	}
	return bg.getOutput(), nil
}

func (g *group) iterable() bool {
	return g.isIterable
}
func makeGroup(src []string, ignoreError bool) (*group, error) {
	g := &group{
		segs:        nil,
		ignoreError: ignoreError,
	}
	for _, s := range src {
		segs, err := makeSegments(s)
		if err != nil {
			return nil, err
		}
		g.segs = append(g.segs, segs)
		if segs.iterable() {
			g.isIterable = true
		}
	}
	return g, nil
}
