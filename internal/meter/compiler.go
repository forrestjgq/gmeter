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
	"unicode"
	"unicode/utf8"

	"github.com/tidwall/sjson"

	"github.com/forrestjgq/gmeter/gplugin"
	"github.com/pkg/errors"

	"github.com/forrestjgq/glog"

	"github.com/forrestjgq/gmeter/internal/argv"
)

const (
	eps = 0.00000001
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
	String() string
}
type staticSegment string

func (ss staticSegment) String() string {
	return string(ss)
}
func (ss staticSegment) iterable() bool {
	return false
}
func (ss staticSegment) produce(bg *background) (string, error) {
	if bg.inDebug() {
		fmt.Printf("static produce: %v\n", ss)
	}
	return string(ss), nil
}

type dynamicSegment struct {
	isIterable bool
	f          func(bg *background) (string, error)
	desc       string
}

func (ds *dynamicSegment) String() string {
	return ds.desc
}
func (ds *dynamicSegment) iterable() bool {
	return ds.isIterable
}
func (ds *dynamicSegment) produce(bg *background) (string, error) {
	if bg.inDebug() {
		fmt.Printf("dynamic produce: %v\n", ds)
	}
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
func (s segments) String() string {
	str := ""
	for i, s := range s {
		if i > 0 {
			str += " | "
		}
		str += s.String()
	}
	return str
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
	if bg.inDebug() {
		fmt.Printf("segments compose: %v\n", s)
	}

	//fmt.Printf("compose %+v\n", s)
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
			if isFalse(content) {
				output = "`" + _false + "`"
			} else if isTrue(content) {
				output = "`" + _true + "`"
			} else {
				return "", errors.Errorf("%s convert %s to bool fail", c.raw, content)
			}
		} else if c.toFloat {
			if !c.exp.MatchString(content) {
				return "", errors.Errorf("%s convert %s to number fail", c.raw, content)
			}
			_, err = strconv.ParseFloat(content, 64)
			if err != nil {
				return "", errors.Wrapf(err, "parse float %s", content)
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
//////////                            ja                             ///////////
////////////////////////////////////////////////////////////////////////////////

// json append
// ja src-json sub-json
type cmdJA struct {
	raw            string
	src, path, sub segments
}

func (c *cmdJA) iterable() bool {
	return false
}

func (c *cmdJA) close() {
}

func (c *cmdJA) execute(bg *background) (string, error) {
	src, err := c.src.compose(bg)
	if err != nil {
		return "", errors.Wrapf(err, "ja %s: compose src", c.raw)
	}
	path, err := c.path.compose(bg)
	if err != nil {
		return "", errors.Wrapf(err, "ja %s: compose path", c.raw)
	}
	sub, err := c.sub.compose(bg)
	if err != nil {
		return "", errors.Wrapf(err, "ja %s: compose sub", c.raw)
	}

	ret, err := sjson.SetRaw(src, path, sub)
	if err != nil {
		return "", errors.Wrapf(err, "ja %s: append (%s) to (%s)", c.raw, sub, src)
	}
	return ret, nil
}

func makeJA(v []string) (command, error) {
	raw := strings.Join(v, " ")
	if len(v) < 2 {
		return nil, errors.Errorf("ja expect at least 2 argument")
	}
	if len(v) > 3 {
		return nil, errors.Errorf("ja %s: accept 2 or 3 arguments", raw)
	}

	src := v[0]
	path := v[1]
	sub := "$(" + KeyInput + ")"
	if len(v) == 3 {
		sub = v[2]
	}
	c := &cmdJA{
		raw: raw,
	}
	var err error
	c.src, err = makeSegments(src)
	if err != nil {
		return nil, errors.Errorf("ja %s: compile src fail", raw)
	}
	c.path, err = makeSegments(path)
	if err != nil {
		return nil, errors.Errorf("ja %s: compile path fail", raw)
	}
	c.sub, err = makeSegments(sub)
	if err != nil {
		return nil, errors.Errorf("ja %s: compile sub fail", raw)
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
//////////                            until                          ///////////
////////////////////////////////////////////////////////////////////////////////

// plugin <pluginName> <message>
type cmdPlugin struct {
	raw     string
	plugin  segments
	message segments
}

func (c *cmdPlugin) iterable() bool {
	return false
}
func (c *cmdPlugin) close() {
}

func (c *cmdPlugin) execute(bg *background) (string, error) {
	name, err := c.plugin.compose(bg)
	if err != nil {
		return "", errors.Wrapf(err, "%s: compose plugin name", c.raw)
	}
	content, err := c.message.compose(bg)
	if err != nil {
		return "", errors.Wrapf(err, "%s: compose message", c.raw)
	}
	err = gplugin.Send(name, content)
	if err != nil {
		return "", errors.Wrapf(err, "%s: send message to plugin %s", c.raw, name)
	}
	return "", nil
}

func makePlugin(v []string) (command, error) {
	c := &cmdPlugin{
		raw: "plugin " + strings.Join(v, " "),
	}
	if len(v) == 0 {
		return nil, errors.Errorf("plugin: no plugin name is given")
	}

	plugin := v[0]
	message := "$(INPUT)"
	if len(v) >= 2 {
		message = strings.Join(v[1:], " ")
	}
	var err error
	c.plugin, err = makeSegments(plugin)
	if err != nil {
		return nil, errors.Wrapf(err, "%s: make plugin", c.plugin)
	}
	c.message, err = makeSegments(message)
	if err != nil {
		return nil, errors.Wrapf(err, "%s: make message", c.message)
	}

	return c, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                            until                          ///////////
////////////////////////////////////////////////////////////////////////////////

// if <condition> then <cmd> [else <cmd>]
type cmdUntil struct {
	raw       string
	condition command
}

func (c *cmdUntil) iterable() bool {
	return true
}
func (c *cmdUntil) close() {
	if c.condition != nil {
		c.condition.close()
	}
}

func (c *cmdUntil) execute(bg *background) (string, error) {
	_, err := c.condition.execute(bg)
	if err != nil {
		return "", nil
	}

	return "", io.EOF
}

func makeUntil(v []string) (command, error) {
	c := &cmdUntil{
		raw: "until " + strings.Join(v, " "),
	}

	if len(v) == 0 {
		return nil, errors.Errorf("%s: invalid until condition: ", strings.Join(v, " "))
	}

	var err error
	c.condition, err = makeAssert(v)
	if err != nil {
		return nil, errors.Wrapf(err, "%s make condition", c.raw)
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

	for {
		if c.scan.Scan() {
			t := c.scan.Text()
			t = strings.TrimSpace(t)
			if len(t) > 0 && t[0] != '#' {
				return t, nil
			}
		} else {
			c.close()
			return "", io.EOF
		}

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
//////////                           call                            ///////////
////////////////////////////////////////////////////////////////////////////////
type cmdCall struct {
	raw  string
	args arguments
}

func (c *cmdCall) iterable() bool {
	return false
}

func (c *cmdCall) close() {
}

func (c *cmdCall) execute(bg *background) (string, error) {
	if bg.inDebug() {
		fmt.Printf("execute call %s\n", c.raw)
	}
	return c.args.call(bg)
}

func makeCall(v []string) (command, error) {
	raw := strings.Join(v, " ")
	c := &cmdCall{
		raw: raw,
	}
	if len(v) == 0 {
		return nil, errors.New("eval nothing")
	}

	var err error
	c.args, err = makeArguments(v)
	if err != nil {
		return nil, errors.Wrapf(err, "make function call(%s)", raw)
	}
	return c, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                           eval                            ///////////
////////////////////////////////////////////////////////////////////////////////
type cmdEval struct {
	raw  string
	eval composable
}

func (c *cmdEval) iterable() bool {
	return false
}

func (c *cmdEval) close() {
}

func (c *cmdEval) execute(bg *background) (string, error) {
	if bg.inDebug() {
		fmt.Printf("execute eval %s\n", c.raw)
	}
	return c.eval.compose(bg)
}

func makeEval(v []string) (command, error) {
	raw := strings.Join(v, " ")
	c := &cmdEval{
		raw: raw,
	}
	if len(v) == 0 {
		return nil, errors.New("eval nothing")
	}

	c.eval = makeExpression(raw)
	return c, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                          assert                           ///////////
////////////////////////////////////////////////////////////////////////////////
type cmdAssert struct {
	raw  string
	eval composable
}

func (c *cmdAssert) iterable() bool {
	return false
}

func (c *cmdAssert) close() {
}

func (c *cmdAssert) execute(bg *background) (string, error) {
	str, err := c.eval.compose(bg)
	if err != nil {
		return "", errors.Wrapf(err, "assert %s", c.raw)
	}

	if isTrue(str) {
		return "", nil
	}
	if isFalse(str) {
		return "", errors.Errorf("assert %s fail", c.raw)
	}

	return "", errors.Errorf("assert %s: result %s is not a bool", c.raw, str)
}

func makeAssert(v []string) (command, error) {
	raw := strings.Join(v, " ")
	c := &cmdAssert{
		raw: raw,
	}
	if len(v) == 0 {
		return nil, errors.New("assert nothing")
	}

	c.eval = makeExpression(raw)
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
	mapping bool
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
		if key == "#" {
			if i < len(segs)-1 {
				return nil, errors.New("# must be last segment in path " + path)
			}
		}

		r := []rune(key)
		switch c := value.(type) {
		case []interface{}:
			if len(key) == 0 {
				continue
			}
			if key == "#" {
				value = len(c)
				break
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
			if key == "#" {
				value = len(c)
				break
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

func (c *cmdJson) mapJsonMap(bg *background, m map[string]interface{}, base string) {
	for k, v := range m {
		s := ""
		if len(base) > 0 {
			k = base + "." + k
		}
		switch t := v.(type) {
		case float64:
			s = strconv.FormatFloat(t, 'f', 8, 64)
		case string:
			s = t
		case map[string]interface{}:
			c.mapJsonMap(bg, t, k)
			continue
		default:
			continue
		}
		bg.setLocalEnv(k, s)
	}
}
func (c *cmdJson) mapJson(bg *background, v interface{}) (string, error) {
	m, ok := v.(map[string]interface{})
	if !ok {
		return "", errors.Errorf("invalid json mapping type %T", v)
	}
	if m != nil {
		c.mapJsonMap(bg, m, "")
	}

	return bg.getLocalEnv(KeyInput), nil

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

	if c.mapping {
		return c.mapJson(bg, v)
	}

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
	case int:
		out = strconv.Itoa(cc)
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
	fs.BoolVar(&c.mapping, "m", false, "map json key value to local environment")
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
		"eval":    makeEval,
		"call":    makeCall,
		"json":    makeJson,
		"list":    makeList,
		"b64":     makeBase64,
		"cvt":     makeCvt,
		"if":      makeIf,
		"until":   makeUntil,
		"plugin":  makePlugin,
		"report":  makeReport,
		"print":   makePrint,
		"sleep":   makeSleep,
		"strrepl": makeStrRepl,
		"strlen":  makeStrLen,
		"ja":      makeJA,
		"escape":  makeEscape,
		"nop":     makeNop,
	}
}
func isCmd(s string) bool {
	return len(s) > 1 && s[0] == '@'
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
func parseCmd(str string) (command, error) {
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
	//phaseEnv
	phaseLocal
	phaseGlobal
	phaseJsonEnv
	phaseArguments
	phaseString
)

type stream struct {
	phase int
	src   []byte // source
	segs  segments

	// scanning state
	ch         rune // current character
	buf        []rune
	offset     int // character offset
	rdOffset   int // reading offset (position after current character)
	prevOffset int
	err        error
}

func (s *stream) errorFrom(err error) {
	if s.err == nil {
		s.err = err
	}
}
func (s *stream) error(offset int, format string, args ...interface{}) {
	if s.err == nil {
		e := errors.Errorf(format, args...)
		s.err = errors.Wrapf(e, "offset: %d", offset)
	}
}
func (s *stream) init(str string) {
	s.src = []byte(str)

	s.ch = ' '
	s.offset = 0
	s.rdOffset = 0
	s.prevOffset = 0
	s.phase = phaseString
}
func (s *stream) clear() string {
	str := string(s.buf)
	s.buf = nil
	return str
}
func (s *stream) read() {
	if s.ch != -1 {
		s.buf = append(s.buf, s.ch)
	}
}
func (s *stream) unread() {
	if s.ch != -1 {
		s.rdOffset = s.offset
	}
}

func (s *stream) next() {
	if s.rdOffset < len(s.src) {
		s.offset = s.rdOffset
		r, w := rune(s.src[s.rdOffset]), 1
		switch {
		case r == 0:
			s.error(s.offset, "illegal character NUL")
		case r >= utf8.RuneSelf:
			// not ASCII
			r, w = utf8.DecodeRune(s.src[s.rdOffset:])
			if r == utf8.RuneError && w == 1 {
				s.error(s.offset, "illegal UTF-8 encoding")
			} else if r == bom && s.offset > 0 {
				s.error(s.offset, "illegal byte order mark")
			}
		}
		s.rdOffset += w
		s.ch = r
		//fmt.Printf("ch: %c\n", s.ch)
	} else {
		s.offset = len(s.src)
		s.ch = -1 // eof
		//fmt.Printf("ch: eof\n")
	}
}
func (s *stream) end() {
	str := s.clear()
	switch s.phase {
	case phaseString:
		if len(str) > 0 {
			s.segs = append(s.segs, staticSegment(str))
		}
	case phaseCmd:
		if len(str) == 0 {
			return
		}

		cmd, err := parseCmd(str)
		if err != nil {
			s.errorFrom(errors.Wrapf(err, "parse cmd"))
			return
		}
		seg := &dynamicSegment{
			f:          cmd.execute,
			desc:       "`" + str + "`",
			isIterable: cmd.iterable(),
		}

		s.segs = append(s.segs, seg)
	case phaseJsonEnv:
		if len(str) == 0 {
			s.errorFrom(errors.New("json env variable name is missing"))
			return
		}
		seg := &dynamicSegment{
			f: func(bg *background) (string, error) {
				return bg.getJsonEnv(str), nil
			},
			desc: "$<" + str + ">",
		}
		s.segs = append(s.segs, seg)
	case phaseGlobal:
		if len(str) == 0 {
			s.errorFrom(errors.New("global variable name is missing"))
			return
		}
		seg := &dynamicSegment{
			f: func(bg *background) (string, error) {
				return bg.getGlobalEnv(str), nil
			},
			desc: "${" + str + "}",
		}
		s.segs = append(s.segs, seg)
	case phaseLocal:
		name := strings.TrimSpace(str)
		if len(name) == 0 {
			s.errorFrom(errors.New("local variable name is missing"))
			return
		}
		if isCmd(name) {
			// treat as command instead of variable
			name = name[1:]
			cmd, err := parseCmd(name)
			if err != nil {
				s.errorFrom(errors.Wrapf(err, "parse cmd"))
				return
			}
			seg := &dynamicSegment{
				f: func(bg *background) (string, error) {
					// here background should be duplicated
					// assume we has a pipeline: echo 5 | assert $(@eval $$ + 3) > $(@eval $$ + 2)
					input := bg.getLocalEnv(KeyInput)
					defer bg.setInput(input)
					return cmd.execute(bg)
				},
				desc:       "$(@" + str + ")",
				isIterable: cmd.iterable(),
			}

			s.segs = append(s.segs, seg)
		} else {
			seg := &dynamicSegment{
				f: func(bg *background) (string, error) {
					return bg.getLocalEnv(str), nil
				},
				desc: "$(" + str + ")",
			}
			s.segs = append(s.segs, seg)
		}
	case phaseArguments:
		if len(str) == 0 {
			s.errorFrom(errors.New("no arguments"))
			return
		}
		idx, err := strconv.Atoi(str)
		if err != nil {
			s.errorFrom(errors.Wrapf(err, "make arg index %s", str))
			return
		}
		seg := &dynamicSegment{
			f: func(bg *background) (string, error) {
				return bg.getArgument(idx)
			},
			desc: "$" + str,
		}
		s.segs = append(s.segs, seg)
	}
}

func (s *stream) compile() (segments, error) {
	depth := 0
	end := false

	for !end {
		if s.err != nil {
			return nil, errors.Wrapf(s.err, "compile %s", string(s.src))
		}
		s.next()
		if s.ch == -1 {
			end = true
		}
		switch s.phase {
		case phaseString:
			if end {
				s.end()
			} else if s.ch == '$' {
				s.end()
				s.next()
				if s.ch == '(' {
					s.phase = phaseLocal
				} else if s.ch == '{' {
					s.phase = phaseGlobal
				} else if s.ch == '<' {
					s.phase = phaseJsonEnv
				} else if unicode.IsDigit(s.ch) {
					s.phase = phaseArguments
					s.unread()
				} else {
					return nil, errors.Errorf("[%d]: expect '(' or '{' after '$'", s.offset)
				}
			} else if s.ch == '`' {
				s.end()
				s.phase = phaseCmd
			} else {
				s.read()
			}
		case phaseCmd:
			if end {
				return nil, errors.Errorf("expect ` for command ending, but got eof")
			} else if s.ch == '`' {
				s.end()
				s.phase = phaseString
			} else {
				s.read()
			}
		case phaseLocal:
			if end {
				return nil, errors.Errorf("expect local variable ending, but got eof")
			} else if s.ch == '(' {
				s.read()
				depth++
			} else if s.ch == ')' {
				if depth > 0 {
					s.read()
					depth--
				} else {
					s.end()
					s.phase = phaseString
				}
			} else {
				s.read()
			}
		case phaseJsonEnv:
			if end {
				return nil, errors.Errorf("expect json variable ending, but got eof")
			} else if s.ch == '>' {
				s.end()
				s.phase = phaseString
			} else {
				s.read()
			}
		case phaseArguments:
			if unicode.IsDigit(s.ch) {
				s.read()
			} else {
				s.end()
				if !end {
					s.unread()
				}
				s.phase = phaseString
			}
		case phaseGlobal:
			if end {
				return nil, errors.Errorf("expect global variable ending, but got eof")
			} else if s.ch == '{' {
				s.read()
				depth++
			} else if s.ch == '}' {
				if depth > 0 {
					s.read()
					depth--
				} else {
					s.end()
					s.phase = phaseString
				}
			} else {
				s.read()
			}
		}
	}

	if s.phase != phaseString {
		return nil, errors.Errorf("parse finish with phase %d, source: %s", s.phase, string(s.src))
	}

	return s.segs, nil
}

// makeSegments creates a segment list which will create a string eventually.
func makeSegments(str string) (segments, error) {
	s := stream{}
	s.init(str)
	return s.compile()
}

type group struct {
	segs        []segments
	ignoreError bool
	isIterable  bool
}

func (g *group) compose(bg *background) (string, error) {
	if bg.inDebug() {
		fmt.Printf("group compose: %v\n", g)
	}

	var err error
	var output string
	for i, seg := range g.segs {
		bg.setInput("")
		output, err = seg.compose(bg)
		if err != nil {
			if !g.ignoreError {
				return "", errors.Wrapf(err, "group[%d]", i)
			}
		}
	}
	return output, nil
}

func (g *group) String() string {
	str := ""
	for i, s := range g.segs {
		if i > 0 {
			str += " ||| "
		}
		str += s.String()
	}
	return str
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

type composeOption struct {
	ignoreOnError bool
}

type composeOptionSetter func(opt *composeOption)

func optIgnoreError() composeOptionSetter {
	return func(opt *composeOption) {
		opt.ignoreOnError = true
	}
}

// makeComposable accepts:
//    - string
//    - []string
// return composable, iterable, error.
func makeComposable(src interface{}, setter ...composeOptionSetter) (composable, bool, error) {
	if src == nil {
		return nil, false, nil
	}
	opt := &composeOption{}
	for _, set := range setter {
		if set != nil {
			set(opt)
		}
	}
	switch v := src.(type) {
	case string:
		segs, err := makeSegments(v)
		if err != nil {
			return nil, false, err
		}
		return segs, segs.iterable(), nil
	case []string:
		if len(v) == 0 {
			return nil, false, nil
		}
		g, err := makeGroup(v, opt.ignoreOnError)
		if err != nil {
			return nil, false, err
		}
		return g, g.iterable(), nil
	case []interface{}:
		if len(v) == 0 {
			return nil, false, nil
		}
		var strs []string
		for _, m := range v {
			if s, ok := m.(string); ok {
				strs = append(strs, s)
			} else {
				return nil, false, errors.Errorf("composable list accept string only, now found type %T value %v", m, m)
			}
		}
		g, err := makeGroup(strs, opt.ignoreOnError)
		if err != nil {
			return nil, false, err
		}
		return g, g.iterable(), nil
	default:
		return nil, false, errors.Errorf("invalid composable type %T value %v", v, v)
	}
}
