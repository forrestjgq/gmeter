package meter

import (
	"bufio"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pkg/errors"

	"github.com/golang/glog"

	"github.com/forrestjgq/gmeter/internal/argv"
)

// composable is an entity which generates a string,
type composable interface {
	compose(bg *background) (string, error)
}

////////////////////////////////////////////////////////////////////////////////
//////////                          segments                         ///////////
////////////////////////////////////////////////////////////////////////////////

type segment interface {
	iterable() bool
	produce(bg *background) (string, error)
}
type staticSegment string

func (ss staticSegment) iterable() bool {
	return false
}
func (ss staticSegment) produce(_ *background) (string, error) {
	return string(ss), nil
}

type dynamicSegment struct {
	isIterable bool
	f          func(bg *background) (string, error)
}

func (ds dynamicSegment) iterable() bool {
	return ds.isIterable
}
func (ds dynamicSegment) produce(bg *background) (string, error) {
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
		str, err := seg.produce(bg)
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
//////////                          Commands                         ///////////
////////////////////////////////////////////////////////////////////////////////

// command is an executable entity
type command interface {
	iterable() bool
	execute(bg *background) (string, error)
	close()
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

func (c *cmdCvt) execute(bg *background) (string, error) {
	output := ""
	if content, err := c.content.compose(bg); err != nil {
		return "", errors.Wrapf(err, "%s compose content", c.raw)
	} else {
		if c.toBool {
			if content == "0" || content == "false" {
				output = "`false`"
			} else if content == "1" || content == "true" {
				output = "`true`"
			} else {
				return "", errors.Errorf("%s convert %s to bool fail", c.raw, content)
			}
		} else if c.toFloat {
			if !c.exp.MatchString(content) {
				return "", errors.Errorf("%s convert %s to number fail", c.raw, content)
			}
			output = "`" + content + "`"
		} else if c.toInt {
			if !c.exp.MatchString(content) {
				return "", errors.Errorf("%s convert %s to int fail", c.raw, content)
			}
			idx := strings.Index(content, ".")
			if idx >= 0 {
				content = content[:idx]
			}
			output = "`" + content + "`"
		} else if c.toRaw {
			output = "`" + content + "`"
		} else {
			output = content
		}
	}

	return output, nil
}

func makeCvt(v []string) (command, error) {
	raw := "cvt " + strings.Join(v, " ")

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
		return nil, errors.Wrapf(err, "parse on making %s", raw)
	}

	content := "$(" + KeyInput + ")"

	v = fs.Args()
	if len(v) == 1 {
		content = v[0]
	} else if len(v) > 1 {
		return nil, errors.Errorf("%s invalid args", raw)
	}

	seg, err := makeSegments(content)
	if err != nil {
		return nil, errors.Wrapf(err, "%s make content", raw)
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
//////////                              nop                          ///////////
////////////////////////////////////////////////////////////////////////////////

// cmdNop does nothing
type cmdNop struct{}

func (c *cmdNop) close() {
}

func (c *cmdNop) iterable() bool {
	return false
}

func (c *cmdNop) execute(_ *background) (string, error) {
	return "", nil
}

func makeNop(_ []string) (command, error) {
	return &cmdNop{}, nil
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

func (c *cmdEscape) execute(bg *background) (string, error) {
	content := ""
	var err error
	if content, err = c.content.compose(bg); err != nil {
		return "", errors.Wrapf(err, "escape %s compose content", c.raw)
	}
	if len(content) > 0 {
		content = strings.ReplaceAll(content, "\"", "\\\"")
	}
	return content, nil
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
		return nil, errors.Errorf("escape %s invalid args", raw)
	}

	var err error
	if c.content, err = makeSegments(content); err != nil {
		return nil, errors.Wrapf(err, "escape %s make content", raw)
	}
	return c, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                            strlen                         ///////////
////////////////////////////////////////////////////////////////////////////////

type cmdStrLen struct {
	raw     string
	content segments
}

func (c *cmdStrLen) iterable() bool {
	return false
}

func (c *cmdStrLen) close() {
	c.content = nil
}

func (c *cmdStrLen) execute(bg *background) (string, error) {
	if content, err := c.content.compose(bg); err != nil {
		return "", errors.Wrapf(err, "strlen %s compose content", c.raw)
	} else {
		length := utf8.RuneCountInString(content)
		return strconv.Itoa(length), nil
	}
}

func makeStrLen(v []string) (command, error) {
	raw := strings.Join(v, " ")
	content := "$(" + KeyInput + ")"
	if len(v) > 0 {
		content = raw
	}
	c := &cmdStrLen{
		raw: raw,
	}
	var err error
	c.content, err = makeSegments(content)
	if err != nil {
		return nil, errors.Wrapf(err, "strlen %s make content", raw)
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

func (c *cmdStrRepl) execute(bg *background) (string, error) {
	content := ""
	newstring := ""
	substring := ""
	var err error
	if content, err = c.content.compose(bg); err != nil {
		return "", errors.Wrapf(err, "%s compose content", c.raw)
	}
	if substring, err = c.substring.compose(bg); err != nil {
		return "", errors.Wrapf(err, "%s compose substring", c.raw)
	}
	if newstring, err = c.newstring.compose(bg); err != nil {
		return "", errors.Wrapf(err, "%s compose new string", c.raw)
	}
	if len(content) > 0 && len(substring) > 0 {
		content = strings.ReplaceAll(content, substring, newstring)
	}

	return content, nil
}

func makeStrRepl(v []string) (command, error) {
	raw := "strrepl " + strings.Join(v, " ")
	c := &cmdStrRepl{
		raw: raw,
	}
	if len(v) >= 2 {
		var err error
		if c.content, err = makeSegments(v[0]); err != nil {
			return nil, errors.Wrapf(err, "%s make content", c.raw)
		}
		if c.substring, err = makeSegments(v[1]); err != nil {
			return nil, errors.Wrapf(err, "%s make substring", c.raw)
		}
		if len(v) > 2 {
			if c.newstring, err = makeSegments(v[2]); err != nil {
				return nil, errors.Wrapf(err, "%s make new string", c.raw)
			}
		} else {
			if c.newstring, err = makeSegments(""); err != nil {
				return nil, errors.Wrapf(err, "%s make new string", c.raw)
			}
		}

		return c, nil
	}
	return nil, fmt.Errorf("%s invalid args", raw)
}

////////////////////////////////////////////////////////////////////////////////
//////////                             fail                          ///////////
////////////////////////////////////////////////////////////////////////////////

type cmdFail struct {
	raw     string
	content segments
}

func (c *cmdFail) iterable() bool {
	return false
}

func (c *cmdFail) close() {
	c.content = nil
}

func (c *cmdFail) execute(bg *background) (string, error) {
	if content, err := c.content.compose(bg); err != nil {
		return "", errors.Wrapf(err, "%s compose ", c.raw)
	} else {
		return "", errors.New(content)
	}
}

func makeFail(v []string) (command, error) {
	raw := "fail " + strings.Join(v, " ")
	content := "$(" + KeyInput + ")"

	if len(v) > 0 {
		content = raw
	}

	seg, err := makeSegments(content)
	if err != nil {
		return nil, errors.Wrapf(err, "%s make content", raw)
	}

	return &cmdFail{
		raw:     raw,
		content: seg,
	}, nil
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

func (c *cmdEcho) execute(bg *background) (string, error) {
	if content, err := c.content.compose(bg); err != nil {
		return "", errors.Wrapf(err, "%s compose content", c.raw)
	} else {
		return content, nil
	}
}

func makeEcho(v []string) (command, error) {
	raw := "echo " + strings.Join(v, " ")
	content := "$(" + KeyInput + ")"

	if len(v) > 0 {
		content = strings.Join(v, " ")
	}

	seg, err := makeSegments(content)
	if err != nil {
		return nil, errors.Wrapf(err, "%s ,make content", raw)
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

func (c *cmdCat) execute(bg *background) (string, error) {
	if len(c.content) == 0 {
		path, err := c.path.compose(bg)
		if err != nil {
			return "", errors.Wrapf(err, "%s compose path ", c.raw)
		}

		b, err := ioutil.ReadFile(filepath.Clean(path))
		if err != nil {
			return "", errors.Wrapf(err, "read %s", path)
		}
		c.content = b
	}
	return string(c.content), nil
}

func makeCat(v []string) (command, error) {
	raw := "cat " + strings.Join(v, " ")

	path := "$(" + KeyInput + ")"
	if len(v) == 1 {
		path = v[0]
	} else if len(v) > 1 {
		return nil, errors.Errorf("%s invalid argument number: %d", raw, len(v))
	}
	seg, err := makeSegments(path)
	if err != nil {
		return nil, errors.Wrapf(err, "%s make path", raw)
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

func (c *cmdWrite) execute(bg *background) (string, error) {
	content, err := c.content.compose(bg)
	if err != nil {
		return "", errors.Wrapf(err, "%s compose content", c.raw)
	}
	// do not check content here
	path, err := c.path.compose(bg)
	if err != nil {
		return "", errors.Wrapf(err, "%s compose path", c.raw)
	}
	if len(path) == 0 {
		return "", errors.Errorf("%s compose empty file path", c.raw)
	}

	err = ioutil.WriteFile(filepath.Clean(path), []byte(content), os.ModePerm)
	if err != nil {
		return "", errors.Wrapf(err, "%s write file %s", c.raw, path)
	}
	return "", nil
}

func makeWrite(v []string) (command, error) {
	raw := "write " + strings.Join(v, " ")
	content := ""
	fs := flag.NewFlagSet("write", flag.ContinueOnError)
	fs.StringVar(&content, "c", "$(INPUT)", "content to write to file, default using local input")
	err := fs.Parse(v)
	if err != nil {
		return nil, errors.Wrapf(err, "%s parse argument", raw)
	}
	v = fs.Args()
	if len(v) != 1 {
		return nil, errors.Errorf("%s path not specified", raw)
	}
	path := v[0]
	c := &cmdWrite{raw: raw}
	if c.path, err = makeSegments(path); err != nil {
		return nil, errors.Wrapf(err, "%s make path", raw)
	}
	if c.content, err = makeSegments(content); err != nil {
		return nil, errors.Wrapf(err, "%s make content", raw)
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

func (c *cmdIf) execute(bg *background) (string, error) {
	_, err := c.condition.execute(bg)
	if err == nil {
		return c.cthen.execute(bg)
	}

	if c.celse != nil {
		return c.celse.execute(bg)
	}

	return "", nil
}

func makeIf(v []string) (command, error) {
	c := &cmdIf{
		raw: "if " + strings.Join(v, " "),
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
		return nil, errors.Errorf("%s: then not found", c.raw)
	}
	cmdCondition = v[:thenIdx]
	for i := thenIdx + 1; i < len(v); i++ {
		if v[i] == "else" {
			elseIdx = i
			break
		}
	}
	if elseIdx == thenIdx+1 {
		return nil, errors.Errorf("%s: then command not found", c.raw)
	}
	cmdThen = v[thenIdx+1 : elseIdx]
	if elseIdx < len(v)-1 {
		cmdElse = v[elseIdx+1:]
	}

	if len(cmdCondition) == 0 || len(cmdThen) == 0 {
		return nil, errors.Errorf("%s: invalid if clause", c.raw)
	}

	var err error
	c.condition, err = makeAssert(cmdCondition)
	if err != nil {
		return nil, errors.Wrapf(err, "%s make condition", c.raw)
	}
	c.cthen, err = parseCmdArgs(cmdThen)
	if err != nil {
		return nil, errors.Wrapf(err, "%s make then", c.raw)
	}
	if len(cmdElse) > 0 {
		c.celse, err = parseCmdArgs(cmdElse)
		if err != nil {
			return nil, errors.Wrapf(err, "%s make else", c.raw)
		}
	}

	return c, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                           sleep                           ///////////
////////////////////////////////////////////////////////////////////////////////

type cmdSleep struct {
	raw string
	du  time.Duration
}

func (c *cmdSleep) iterable() bool {
	return false
}
func (c *cmdSleep) close() {
}

func (c *cmdSleep) execute(_ *background) (string, error) {
	if c.du > 0 {
		time.Sleep(c.du)
	}
	return "", nil
}

func makeSleep(v []string) (command, error) {
	raw := "sleep " + strings.Join(v, " ")
	if len(v) == 1 {
		s := v[0]
		du, err := time.ParseDuration(s)
		if err != nil {
			return nil, errors.Wrapf(err, "parse duration %s", s)
		}
		return &cmdSleep{
			raw: raw,
			du:  du,
		}, nil
	} else {
		return nil, errors.New("sleep requires a duration like 3m, 1m20s...")
	}

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

func (c *cmdPrint) execute(bg *background) (string, error) {
	if c.format != nil {
		content, err := c.format.compose(bg)
		if err != nil {
			return "", errors.Wrapf(err, "%s compose content", c.raw)
		}
		fmt.Print(content, "\n")
	}
	return "", nil
}

func makePrint(v []string) (command, error) {
	raw := "print " + strings.Join(v, " ")
	content := "$(" + KeyInput + ")"

	if len(v) > 0 {
		content = strings.Join(v, " ")
	}

	seg, err := makeSegments(content)
	if err != nil {
		return nil, errors.Wrapf(err, "%s make content", raw)
	}
	return &cmdPrint{
		raw:    raw,
		format: seg,
	}, nil

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

func (c *cmdReport) execute(bg *background) (string, error) {
	if c.format != nil {
		content, err := c.format.compose(bg)
		if err != nil {
			return "", errors.Wrapf(err, "%s compose content", c.raw)
		}
		if c.template {
			bg.reportTemplate(content, c.newline)
		} else {
			bg.report(content, c.newline)
		}
	} else {
		bg.reportDefault(c.newline)
	}

	return "", nil
}

func makeReport(v []string) (command, error) {
	raw := "report " + strings.Join(v, " ")
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
		return nil, errors.Wrapf(err, "%s parse argument", raw)
	}

	if len(format) > 0 {
		if c.format, err = makeSegments(format); err != nil {
			return nil, errors.Wrapf(err, "%s make format", raw)
		}
	} else if len(template) > 0 {
		if c.format, err = makeSegments(template); err != nil {
			return nil, errors.Wrapf(err, "%s make template", raw)
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
	envRead
	envMove
)

type cmdEnv struct {
	op       int
	variable segments
	value    segments
	raw      string
}

func (c *cmdEnv) iterable() bool {
	return false
}
func (c *cmdEnv) close() {
	c.variable = nil
	c.value = nil
}

func (c *cmdEnv) execute(bg *background) (string, error) {
	variable, err := c.variable.compose(bg)
	if err != nil {
		return "", errors.Wrapf(err, "%s compose variable", c.raw)
	}
	if c.op == envRead {
		return bg.getLocalEnv(variable), nil
	} else if c.op == envDelete {
		bg.delLocalEnv(variable)
	} else if c.op == envWrite {
		value, err := c.value.compose(bg)
		if err != nil {
			return "", errors.Wrapf(err, "%s compose value", c.raw)
		}
		bg.setLocalEnv(variable, value)
	} else if c.op == envMove {
		dst, err := c.value.compose(bg)
		if err != nil {
			return "", errors.Wrapf(err, "%s compose dst value", c.raw)
		}
		bg.setLocalEnv(dst, bg.getLocalEnv(variable))
		bg.delLocalEnv(variable)
	} else {
		return "", errors.Errorf("%s: unknown operator %d", c.raw, c.op)
	}
	return "", nil
}

/*
	env -r/-d var
    env -w var [content/$$]
	env -m src dst
*/
func makeEnv(v []string) (command, error) {
	raw := "env " + strings.Join(v, " ")
	read, write, del, mv := false, false, false, false
	fs := flag.NewFlagSet("env", flag.ContinueOnError)
	fs.BoolVar(&read, "r", false, "read from local variable")
	fs.BoolVar(&write, "w", false, "write to local variable")
	fs.BoolVar(&del, "d", false, "delete from local variable")
	fs.BoolVar(&mv, "m", false, "move local variable to another")
	err := fs.Parse(v)
	if err != nil {
		return nil, errors.Wrapf(err, "%s parse argument", raw)
	}
	v = fs.Args()
	if len(v) == 0 {
		return nil, errors.Errorf("%s variable not provided", raw)
	}
	c := &cmdEnv{
		raw: raw,
		op:  envRead,
	}
	if c.variable, err = makeSegments(v[0]); err != nil {
		return nil, errors.Wrapf(err, "%s make variable", raw)
	}
	v = v[1:]

	cnt := 0
	if read {
		cnt++
		if len(v) > 0 {
			return nil, errors.New("env: too much argument")
		}
	}
	if write {
		cnt++
		c.op = envWrite
		content := "$(" + KeyInput + ")"
		if len(v) > 0 {
			content = strings.Join(v, " ")
		}
		if c.value, err = makeSegments(content); err != nil {
			return nil, errors.Wrapf(err, "%s make content", raw)
		}
	}
	if del {
		cnt++
		c.op = envDelete
		if len(v) > 0 {
			return nil, errors.New("env: too much argument")
		}
	}
	if mv {
		cnt++
		c.op = envMove
		if len(v) != 1 {
			return nil, errors.New("env -m <src> <dst>")
		}
		if c.value, err = makeSegments(v[0]); err != nil {
			return nil, errors.Wrapf(err, "%s make content", raw)
		}
	}

	if cnt > 1 {
		return nil, errors.New("db only accept one of -r/-d/-w")
	}

	return c, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                            db                             ///////////
////////////////////////////////////////////////////////////////////////////////

const (
	dbWrite = iota
	dbDelete
	dbRead
)

type cmdDB struct {
	op       int
	variable segments
	value    segments
	raw      string
}

func (c *cmdDB) iterable() bool {
	return false
}
func (c *cmdDB) close() {
	c.variable = nil
	c.value = nil
}

func (c *cmdDB) execute(bg *background) (string, error) {
	variable, err := c.variable.compose(bg)
	if err != nil {
		return "", errors.Wrapf(err, "%s compose variable", c.raw)
	}
	if c.op == dbDelete {
		bg.dbDelete(variable)
	} else if c.op == dbWrite {
		value, err := c.value.compose(bg)
		if err != nil {
			return "", errors.Wrapf(err, "%s compose value", c.raw)
		}
		bg.dbWrite(variable, value)
	} else if c.op == dbRead {
		return bg.dbRead(variable), nil
	} else {
		return "", errors.Errorf("%s: unknown operator %d", c.raw, c.op)
	}
	return "", nil
}

// db -r key // default
// db -w key value...
// db -d key
func makeDB(v []string) (command, error) {
	raw := "db " + strings.Join(v, " ")
	read := false
	write := false
	del := false
	fs := flag.NewFlagSet("env", flag.ContinueOnError)
	fs.BoolVar(&read, "r", false, "read from database")
	fs.BoolVar(&write, "w", false, "write to database")
	fs.BoolVar(&del, "d", false, "delete from database")
	err := fs.Parse(v)
	if err != nil {
		return nil, errors.Wrapf(err, "%s parse argument", raw)
	}
	v = fs.Args()
	if len(v) == 0 {
		return nil, errors.Errorf("%s variable not provided", raw)
	}
	cnt := 0
	if read {
		cnt++
	}
	if write {
		cnt++
	}
	if del {
		cnt++
	}
	if cnt == 0 {
		read = true
	} else if cnt > 1 {
		return nil, errors.New("db only accept one of -r/-d/-w")
	}
	variable := v[0]
	c := &cmdDB{
		raw: raw,
		op:  dbRead,
	}
	if c.variable, err = makeSegments(variable); err != nil {
		return nil, errors.Wrapf(err, "%s make variable", raw)
	}
	if write {
		c.op = dbWrite
		content := "$(" + KeyInput + ")"
		if len(v) > 1 {
			content = strings.Join(v[1:], " ")
		}

		if c.value, err = makeSegments(content); err != nil {
			return nil, errors.Wrapf(err, "%s make content", raw)
		}
	} else if del {
		c.op = dbDelete
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

func (c *cmdList) execute(bg *background) (string, error) {
	if c.file == nil {
		var err error
		path, err := c.path.compose(bg)
		if err != nil {
			return "", errors.Wrapf(err, "%s compose path", c.raw)
		}
		path, err = loadFilePath(bg.getGlobalEnv(KeyTPath), path)
		if err != nil {
			return "", errors.Wrapf(err, "%s load path %s", path, c.raw)
		}
		c.file, err = os.Open(path)
		if err != nil {
			return "", errors.Wrapf(err, "%s: open file %s", c.raw, path)
		}
		c.scan = bufio.NewScanner(c.file)
	}

	if c.scan.Scan() {
		t := c.scan.Text()
		if len(t) > 0 {
			return t, nil
		}
		c.close()
		return "", io.EOF
	} else {
		c.close()
		return "", io.EOF
	}
}

func makeList(v []string) (command, error) {
	raw := "list " + strings.Join(v, " ")
	if len(v) != 1 {
		return nil, fmt.Errorf("%s invalid argument number %d", raw, len(v))
	}
	path := v[0]
	if len(path) == 0 {
		return nil, errors.Errorf("%s: file path not provided", raw)
	}
	seg, err := makeSegments(path)
	if err != nil {
		return nil, errors.Wrapf(err, "%s make path", raw)
	}
	return &cmdList{
		raw:  raw,
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
	decode  bool
	path    segments
	content segments
	result  string
}

func (c *cmdB64) iterable() bool {
	return false
}
func (c *cmdB64) close() {
	c.content = nil
	c.path = nil
}

func (c *cmdB64) execute(bg *background) (string, error) {
	var err error
	if len(c.result) == 0 {
		result := ""
		if c.file {
			path, err := c.path.compose(bg)
			if err != nil {
				return "", errors.Wrapf(err, "%s compose path", c.raw)
			}

			if len(path) == 0 {
				return "", errors.Errorf("%s: file path is empty", c.raw)
			}

			if f, err := os.Open(filepath.Clean(path)); err != nil {
				return "", errors.Wrapf(err, "%s: open file %s", c.raw, path)
			} else {
				if b, err1 := ioutil.ReadAll(f); err1 != nil {
					_ = f.Close()
					return "", errors.Wrapf(err1, "%s read file %s ", c.raw, path)
				} else {
					result = string(b)
				}
				_ = f.Close()
			}
		} else {
			if result, err = c.content.compose(bg); err != nil {
				return "", errors.Wrapf(err, "%s compose content ", c.raw)
			}
		}

		if c.decode {
			b, err := base64.StdEncoding.DecodeString(result)
			if err != nil {
				return "", errors.Wrap(err, "base64 decode")
			}
			result = string(b)
		} else {
			result = base64.StdEncoding.EncodeToString([]byte(result))
		}
		if c.static {
			c.result = result
		}
		return result, nil
	} else {
		return c.result, nil
	}
}

func makeBase64(v []string) (command, error) {
	raw := "b64 " + strings.Join(v, " ")
	c := &cmdB64{
		raw: raw,
	}
	content := ""
	path := ""
	file := false
	fs := flag.NewFlagSet("b64", flag.ContinueOnError)
	fs.BoolVar(&file, "f", false, "encode file content to base64")
	fs.BoolVar(&c.decode, "d", false, "decode file or content to raw string")
	err := fs.Parse(v)
	if err != nil {
		return nil, errors.Wrapf(err, "%s parse argument", raw)
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
		return nil, errors.Errorf("%s: parse error", raw)
	}

	c.file = file
	c.path, err = makeSegments(path)
	if err != nil {
		return nil, errors.Wrapf(err, "%s make path", raw)
	}
	c.content, err = makeSegments(content)
	if err != nil {
		return nil, errors.Wrapf(err, "%s make content", raw)
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
	hint  segments
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

func (c *cmdAssert) doFloat(lhs, rhs string, _ *background) error {
	var (
		a, b float64
		err  error
	)
	if a, err = strconv.ParseFloat(lhs, 64); err != nil {
		return errors.Wrapf(err, "convert %s", lhs)
	}
	if b, err = strconv.ParseFloat(rhs, 64); err != nil {
		return errors.Wrapf(err, "convert %s", rhs)
	}

	delta := a - b
	switch c.op {
	case opEqual:
		if delta < -eps || delta > eps {
			return errors.Errorf("assert fail: %s == %s", lhs, rhs)
		}
	case opNotEqual:
		if delta >= -eps && delta <= eps {
			return errors.Errorf("assert fail: %s != %s", lhs, rhs)
		}
	case opGreater:
		if delta <= 0 {
			return errors.Errorf("assert fail: %s > %s", lhs, rhs)
		}
	case opGreaterEqual:
		if delta < 0 {
			return errors.Errorf("assert fail: %s >= %s", lhs, rhs)
		}
	case opLess:
		if delta >= 0 {
			return errors.Errorf("assert fail: %s < %s", lhs, rhs)
		}
	case opLessEqual:
		if delta > 0 {
			return errors.Errorf("assert fail: %s <= %s", lhs, rhs)
		}
	default:
		return errors.Errorf("assert(%s): unknown operator %d", c.raw, c.op)
	}
	return nil
}
func (c *cmdAssert) doNum(lhs, rhs string, _ *background) error {
	var (
		a, b int
		err  error
	)
	if a, err = strconv.Atoi(lhs); err != nil {
		return errors.Wrapf(err, "convert %s", lhs)
	}
	if b, err = strconv.Atoi(rhs); err != nil {
		return errors.Wrapf(err, "convert %s", rhs)
	}

	delta := a - b
	switch c.op {
	case opEqual:
		if delta != 0 {
			return errors.Errorf("assert fail: %s == %s", lhs, rhs)
		}
	case opNotEqual:
		if delta == 0 {
			return errors.Errorf("assert fail: %s != %s", lhs, rhs)
		}
	case opGreater:
		if delta <= 0 {
			return errors.Errorf("assert fail: %s > %s", lhs, rhs)
		}
	case opGreaterEqual:
		if delta < 0 {
			return errors.Errorf("assert fail: %s >= %s", lhs, rhs)
		}
	case opLess:
		if delta >= 0 {
			return errors.Errorf("assert fail: %s < %s", lhs, rhs)
		}
	case opLessEqual:
		if delta > 0 {
			return errors.Errorf("assert fail: %s <= %s", lhs, rhs)
		}
	default:
		return errors.Errorf("assert(%s): unknown operator %d", c.raw, c.op)
	}
	return nil
}
func (c *cmdAssert) doStr(lhs, rhs string, _ *background) error {
	if c.op == opEqual {
		if lhs != rhs {
			return errors.Errorf("assert fail: %s == %s", lhs, rhs)
		}
	} else if c.op == opNotEqual {
		if lhs == rhs {
			return errors.Errorf("assert fail: %s != %s", lhs, rhs)
		}
	} else {
		return errors.Errorf("assert not support, op: %d, lhs %s rhs %s", c.op, lhs, rhs)
	}
	return nil
}
func (c *cmdAssert) judge(bg *background) error {
	var (
		a, b string
		err  error
	)
	if a, err = c.a.compose(bg); err != nil {
		return errors.Wrapf(err, "compose lhs")
	}
	if c.op == opIs {
		if a == "1" || a == "true" {
			return nil
		}
		return errors.Errorf("assert %s", a)
	}
	if c.op == opNot {
		if a == "0" || a == "false" || a == "" {
			return nil
		}
		return errors.Errorf("assert !%s", a)
	}
	if b, err = c.b.compose(bg); err != nil {
		return errors.Wrapf(err, "compose rhs fail")
	}

	ta, tb := c.kindOf(a), c.kindOf(b)
	if ta == isStr || tb == isStr {
		err = c.doStr(a, b, bg)
	} else if ta == isFloat || tb == isFloat {
		err = c.doFloat(a, b, bg)
	} else {
		err = c.doNum(a, b, bg)
	}

	if err != nil {
		return errors.Wrapf(err, "judge")
	}
	return nil
}
func (c *cmdAssert) execute(bg *background) (string, error) {
	err := c.judge(bg)
	if err != nil {
		if c.hint != nil {
			if hint, _ := c.hint.compose(bg); len(hint) > 0 {
				return "", errors.Wrapf(err, hint)
			}
		}
		return "", errors.Wrapf(err, c.raw)
	}

	return "", nil
}

func makeAssert(v []string) (command, error) {
	var a string
	var b string
	raw := "assert " + strings.Join(v, " ")
	c := &cmdAssert{
		raw: raw,
	}
	if len(v) == 0 {
		return nil, errors.New("assert nothing")
	}

	for i, s := range v {
		if s == "-h" {
			if i < len(v)-1 {
				hint := strings.Join(v[i+1:], " ")
				seg, err := makeSegments(hint)
				if err != nil {
					return nil, errors.Wrapf(err, "%s make hint", raw)
				}
				c.hint = seg
			}
			v = v[0:i]
			break
		}
	}

	if v[0] == "!" {
		c.op = opNot
		if len(v) > 2 {
			return nil, errors.Errorf("%s: expect ! variable, but more comes", raw)
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
			return nil, errors.Errorf("%s: invalid operator %s", raw, v[1])
		}
	} else {
		return nil, errors.Errorf("%s: expect expr as <a op b>", raw)
	}

	var err error
	if c.a, err = makeSegments(a); err != nil {
		return nil, errors.Wrapf(err, "%s make lhs %s", raw, a)
	}
	if c.b, err = makeSegments(b); err != nil {
		return nil, errors.Wrapf(err, "%s make rhs %s", raw, b)
	}

	if c.float, err = regexp.Compile(`^-?[0-9]+\.[0-9]*$`); err != nil {
		glog.Fatalf("compile float expr fail")
	}
	if c.num, err = regexp.Compile("^-?[0-9]+$"); err != nil {
		glog.Fatalf("compile num expr fail")
	}
	return c, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                             json                          ///////////
////////////////////////////////////////////////////////////////////////////////

type cmdJson struct {
	raw     string
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

	// if path is "." or ".." ..., remove useless path segs
	var temp []string
	for i := range segs {
		s := strings.TrimSpace(segs[i])
		if i == 0 || len(s) > 0 {
			temp = append(temp, s)
		}
	}
	segs = temp

	var value interface{}
	if err := json.Unmarshal([]byte(content), &value); err != nil {
		return nil, errors.Wrapf(err, "unmarshal: %s", content)
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
			if len(key) == 0 {
				continue
			}
			if len(r) < 2 || r[0] != '[' || r[len(r)-1] != ']' {
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
func (c *cmdJson) execute(bg *background) (string, error) {
	content, err := c.content.compose(bg)
	if err != nil {
		return "", errors.Wrapf(err, "%s make content", c.raw)
	}
	// do not check content here
	path, err := c.path.compose(bg)
	if err != nil {
		return "", errors.Wrapf(err, "%s compose path", c.raw)
	}
	if len(path) == 0 {
		return "", errors.Errorf("%s: empty path", c.raw)
	}

	v, err := c.find(content, path)

	if c.numer {
		// not found -> zero value of number -> 0
		if err != nil {
			return "0", nil
		}

		if cc, ok := v.([]interface{}); !ok {
			return "", errors.Errorf("%s: %s is not a list", c.raw, path)
		} else {
			return strconv.Itoa(len(cc)), nil
		}
	}

	if c.exist {
		if err != nil {
			return "", errors.Wrapf(err, "%s: check exist", c.raw)
		}
	} else {
		// exist flag is not set, so set output to empty string
		if err != nil {
			return "", nil
		}
	}

	out := ""
	switch cc := v.(type) {
	case bool:
		if cc {
			out = "1"
		} else {
			out = "0"
		}
	case float64:
		out = strconv.FormatFloat(cc, 'f', 8, 64)
	case string:
		out = cc
	case json.Number:
		out = string(cc)
	case []interface{}, map[string]interface{}:
		if b, err := json.Marshal(cc); err != nil {
			return "", errors.Wrapf(err, "%s: marshal %v", c.raw, cc)
		} else {
			out = string(b)
		}
	default:
		return "", errors.Errorf("%s: unknown value type %T", c.raw, cc)
	}
	return out, nil
}

func makeJson(v []string) (command, error) {
	content := "$(INPUT)"
	raw := "json " + strings.Join(v, " ")
	c := &cmdJson{raw: raw}

	fs := flag.NewFlagSet("json", flag.ContinueOnError)
	fs.BoolVar(&c.exist, "e", false, "check if path exists")
	fs.BoolVar(&c.numer, "n", false, "get number of list item")
	err := fs.Parse(v)
	if err != nil {
		return nil, errors.Wrapf(err, "%s parse argument", raw)
	}
	if c.exist && c.numer {
		return nil, errors.Errorf("%s: can not take both -n and -e option", raw)
	}
	v = fs.Args()
	if len(v) > 2 {
		return nil, errors.Errorf("%s: [-n] [-e] <path> [<content>]", raw)
	}
	if len(v) < 1 {
		return nil, errors.Errorf("%s: path not specified", raw)
	}
	path := v[0]
	if c.path, err = makeSegments(path); err != nil {
		return nil, errors.Wrapf(err, "%s: make path %s", raw, path)
	}

	if len(v) == 2 {
		content = v[1]
	}

	if c.content, err = makeSegments(content); err != nil {
		return nil, errors.Wrapf(err, "%s: make content %s", raw, content)
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
func (p pipeline) execute(bg *background) (string, error) {
	output := ""
	var err error
	for i, c := range p {
		bg.setInput(output)
		output, err = c.execute(bg)
		if err != nil {
			return "", errors.Wrapf(err, "pipeline[%d]", i)
		}
	}
	return output, nil
}
func (p pipeline) close() {
	for _, c := range p {
		c.close()
	}
}

type cmdMaker func(v []string) (command, error)

var cmdMap map[string]cmdMaker

func init() {
	// we can not init it as global var due to initialize loop error
	cmdMap = map[string]cmdMaker{
		"echo":    makeEcho,
		"fail":    makeFail,
		"cat":     makeCat,
		"env":     makeEnv,
		"db":      makeDB,
		"write":   makeWrite,
		"assert":  makeAssert,
		"json":    makeJson,
		"list":    makeList,
		"b64":     makeBase64,
		"cvt":     makeCvt,
		"if":      makeIf,
		"report":  makeReport,
		"print":   makePrint,
		"sleep":   makeSleep,
		"strrepl": makeStrRepl,
		"strlen":  makeStrLen,
		"escape":  makeEscape,
		"nop":     makeNop,
	}
}
func isCmd(s string) bool {
	idx := -1
	s = strings.TrimSpace(s)
	for i, r := range s {
		if r == '$' || r == '\t' || r == ' ' {
			idx = i
			break
		}
	}

	first := s
	if idx != -1 {
		first = s[:idx]
	}

	if _, ok := cmdMap[first]; ok {
		return true
	}
	return false
}

func parseCmdArgs(args []string) (command, error) {
	name := args[0]
	args = args[1:]
	for i, s := range args {
		if s == "$" {
			args[i] = "$<" + jsonEnvValue + ">"
		} else if s == "$$" {
			args[i] = "$(" + KeyInput + ")"
		}
	}

	if f, ok := cmdMap[name]; ok {
		return f(args)
	}
	return nil, errors.Errorf("cmd %s not supported", name)
}
func parse(str string) (command, error) {
	args, err := argv.Argv(str, nil, func(s string) (string, error) {
		return s, nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "parse %s ", str)
	}

	var pp pipeline
	for _, v := range args {
		if len(v) == 0 {
			continue
		}
		var cmd command

		cmd, err = parseCmdArgs(v)
		if err != nil {
			return nil, errors.Wrapf(err, "parse %s", str)
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
	phaseJsonEnv
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
			} else if c == '<' {
				phase = phaseJsonEnv
			} else {
				return nil, errors.Errorf("[%d]: expect '(' or '{' after '$'", i)
			}
		case phaseLocal:
			if c == ')' {
				phase = phaseString
			}
		case phaseJsonEnv:
			if c == '>' {
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
						return nil, errors.Wrapf(err, "parse cmd")
					}
					seg := &dynamicSegment{f: func(bg *background) (string, error) {
						return cmd.execute(bg)
					}}

					seg.isIterable = cmd.iterable()
					segs = append(segs, seg)
				}
			case phaseEnv:
			case phaseJsonEnv:
				if i > start {
					name := string(r[start:i])
					if len(name) == 0 {
						return nil, errors.New("json env variable name is missing")
					}
					segs = append(segs, &dynamicSegment{f: func(bg *background) (string, error) {
						return bg.getJsonEnv(name), nil
					}})
				}
			case phaseLocal:
				if i > start {
					name := string(r[start:i])
					name = strings.TrimSpace(name)
					if len(name) == 0 {
						return nil, errors.New("local variable name is missing")
					}
					if isCmd(name) {
						// treat as command instead of variable
						cmd, err := parse(name)
						if err != nil {
							return nil, errors.Wrapf(err, "parse cmd")
						}
						seg := &dynamicSegment{f: func(bg *background) (string, error) {
							return cmd.execute(bg)
						}}

						seg.isIterable = cmd.iterable()
						segs = append(segs, seg)
					} else {
						segs = append(segs, &dynamicSegment{f: func(bg *background) (string, error) {
							return bg.getLocalEnv(name), nil
						}})
					}
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
		return nil, errors.Errorf("parse finish with phase %d, source: %s", phase, str)
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
	var err error
	for i, seg := range g.segs {
		bg.setInput("")
		_, err = seg.compose(bg)
		if err != nil {
			if !g.ignoreError {
				return "", errors.Wrapf(err, "group[%d]", i)
			}
		}
	}
	return "", nil
}

func (g *group) iterable() bool {
	return g.isIterable
}
func makeGroup(src []string, ignoreError bool) (*group, error) {
	g := &group{
		segs:        nil,
		ignoreError: ignoreError,
	}
	for i, s := range src {
		segs, err := makeSegments(s)
		if err != nil {
			return nil, errors.Wrapf(err, "make group[%d]", i)
		}
		g.segs = append(g.segs, segs)
		if segs.iterable() {
			g.isIterable = true
		}
	}
	return g, nil
}
