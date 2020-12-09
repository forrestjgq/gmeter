package meter

import (
	"bufio"
	"encoding/base64"
	"errors"
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
//////////                             cat                           ///////////
////////////////////////////////////////////////////////////////////////////////

type cmdCat struct {
	path    string
	content []byte
}

func (c *cmdCat) close() {
	c.content = nil
}

func (c *cmdCat) produce(bg *background) {
	if len(c.content) == 0 {
		path := c.path
		if len(path) == 0 {
			path = bg.getInput()
		}

		if len(path) == 0 {
			bg.setError("cat: file path is empty")
			return
		}

		if f, err := os.Open(filepath.Clean(path)); err != nil {
			bg.setError(err.Error())
		} else {
			if b, err1 := ioutil.ReadAll(f); err1 != nil {
				bg.setError("cat: " + err1.Error())
			} else {
				c.content = b
				bg.setOutput(string(b))
			}
			_ = f.Close()
		}
	} else {
		bg.setOutput(string(c.content))
	}
}

func makeCat(path string) (command, error) {
	return &cmdCat{path: path}, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                           write                           ///////////
////////////////////////////////////////////////////////////////////////////////
type cmdWrite struct {
	path    string
	content string
}

func (c *cmdWrite) close() {
	c.content = ""
}

func (c *cmdWrite) produce(bg *background) {
	content := c.content
	if len(content) == 0 {
		content = bg.getInput()
	}

	if len(c.path) == 0 {
		bg.setError("cat: file path is empty")
		return
	}
	// do not check content here

	if f, err := os.Create(filepath.Clean(c.path)); err != nil {
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
	return &cmdWrite{
		path:    path,
		content: content,
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                            list                           ///////////
////////////////////////////////////////////////////////////////////////////////
type cmdList struct {
	path string
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
		c.file, err = os.Open(filepath.Clean(c.path))
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
	return &cmdList{
		path: path,
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                             b64                           ///////////
////////////////////////////////////////////////////////////////////////////////

type cmdB64 struct {
	file    bool
	path    string
	content string
}

func (c *cmdB64) close() {
	c.content = ""
}

func (c *cmdB64) produce(bg *background) {
	if len(c.content) == 0 {
		if c.file {
			path := c.path
			write := true

			if len(path) == 0 {
				path = bg.getInput()
				write = false
			}

			if len(path) == 0 {
				bg.setError("cat: file path is empty")
				return
			}

			if f, err := os.Open(filepath.Clean(path)); err != nil {
				bg.setError(err.Error())
			} else {
				if b, err1 := ioutil.ReadAll(f); err1 != nil {
					bg.setError("cat: " + err1.Error())
				} else {
					s := base64.StdEncoding.EncodeToString(b)
					if len(s) == 0 {
						bg.setError("b64: empty input")
						return
					}
					if write {
						c.content = s
					}
					bg.setOutput(s)
				}
				_ = f.Close()
			}

		} else {
			// do not write, dynamic use input as content
			bg.setOutput(base64.StdEncoding.EncodeToString([]byte(bg.getInput())))
		}

	} else {
		bg.setOutput(c.content)
	}
}

func makeB64(content string) (command, error) {
	enc := ""
	if len(content) > 0 {
		enc = base64.StdEncoding.EncodeToString([]byte(content))
	}
	return &cmdB64{
		content: enc,
	}, nil
}
func makeFileB64(path string) (command, error) {
	return &cmdB64{
		file: true,
		path: path,
	}, nil
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
			content := ""
			path := ""
			file := false
			if len(v) == 1 {

			} else if len(v) == 2 {
				if v[1] == "-f" {
					file = true
				} else {
					content = v[1]
				}
			} else if len(v) == 3 {
				if v[1] != "-f" {
					return nil, fmt.Errorf("b64 expect -f, got %s", v[1])
				}
				path = v[2]
				file = true
			} else if len(v) > 3 {
				return nil, fmt.Errorf("b64 invalid: %v", v)
			}

			var cmd command
			var err error
			if file {
				cmd, err = makeFileB64(path)
			} else {
				cmd, err = makeB64(content)
			}
			if err != nil {
				return nil, err
			} else {
				pp = append(pp, cmd)
			}
		}
	}

	return pp, nil
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

func (s segments) compose(bg *background) (string, error) {
	arr := make([]string, len(s))
	for i, seg := range s {
		str, err := seg.getString(bg)
		if err != nil {
			return "", err
		}
		arr[i] = str

	}
	return strings.Join(arr, ""), nil
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
