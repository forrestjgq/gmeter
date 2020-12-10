package meter

import (
	"bufio"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
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

func makeWrite(path, content string) (command, error) {
	if len(path) == 0 {
		return nil, errors.New("write file path not provided")
	}
	if len(content) == 0 {
		content = "$(" + KeyInput + ")"
	}
	c := &cmdWrite{}
	var err error
	if c.path, err = makeSegments(path); err != nil {
		return nil, err
	}
	if c.content, err = makeSegments(content); err != nil {
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

		case "write":
			path, content := "", ""

			if len(v) == 1 || len(v) > 3 {
				return nil, fmt.Errorf("write invalid: %v", v)
			}
			path = v[1]
			if len(v) == 3 {
				content = v[2]
			}

			if cmd, err := makeWrite(path, content); err != nil {
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
	if len(r)-start > 1 {
		segs = append(segs, staticSegment(r[start:]))
	}

	return segs, nil
}
