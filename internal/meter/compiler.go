package meter

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

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

	for c.scan.Scan() {
		t := c.scan.Text()
		if len(t) > 0 {
			bg.setOutput(t)
		} else {
			bg.setError(EOF)
		}
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
	content string
}

func (c *cmdB64) close() {
	c.content = ""
}

func (c *cmdB64) produce(bg *background) {
	content := c.content
	if len(content) == 0 {
		c.content = bg.getInput()
	}
	if len(content) == 0 {
		bg.setError("b64: empty input")
		return
	}
	s := base64.StdEncoding.EncodeToString([]byte(content))
	bg.setOutput(s)
}

func makeB64(content string) (command, error) {
	return &cmdB64{
		content: content,
	}, nil
}

////////////////////////////////////////////////////////////////////////////////
//////////                             lua                           ///////////
////////////////////////////////////////////////////////////////////////////////

////////////////////////////////////////////////////////////////////////////////
//////////                          pipeline                         ///////////
////////////////////////////////////////////////////////////////////////////////
type pipeline []command

func parse(str string) (pipeline, error) {
	args, err := argv.Argv(str, nil, func(s string) (string, error) {
		return s, nil
	})
	if err != nil {
		return nil, err
	}

	var pp []command
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
			if len(v) == 2 {
				content = v[1]
			} else if len(v) > 2 {
				return nil, fmt.Errorf("b64 invalid: %v", v)
			}
			if cmd, err := makeB64(content); err != nil {
				return nil, err
			} else {
				pp = append(pp, cmd)
			}

		}
	}

	return pp, nil
}
