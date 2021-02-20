package meter

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/forrestjgq/gmeter/config"
	"github.com/forrestjgq/gmeter/internal/arcee"
	"github.com/golang/glog"
	"github.com/pkg/errors"
)

func Execute(opt *config.GOptions) error {
	_, err := startPerf(0)
	if err != nil {
		defer stopPerf()
	}

	if len(opt.Vars) > 0 {
		for k, v := range opt.Vars {
			AddGlobalVariable(k, v)
		}
	}

	hasServer := false
	if len(opt.HTTPServerCfg) > 0 {
		err := StartHTTPServer(opt.HTTPServerCfg)
		if err != nil {
			return errors.Wrapf(err, "HTTP server start")
		} else {
			hasServer = true
		}
		defer func() {
			StopAll()
		}()
	}

	if len(opt.ArceeServerCfg) > 0 {
		_, err = arcee.StartArcee(opt.ArceeServerCfg)
		if err != nil {
			return errors.Wrap(err, "start arcee")
		} else {
			hasServer = true
		}
	}

	if len(opt.Call) > 0 {
		go func() {
			startSubProcess("child", opt.Call)
		}()
	}

	executor := func(path string) error {
		fmt.Println("gmeter starts ", path)
		c, err := loadConfig(path)
		if err != nil {
			return errors.Wrapf(err, "load config %s", path)
		}
		// load base config from command line first
		if len(opt.Template) > 0 {
			opt.Template, err = filepath.Abs(opt.Template)
			if err != nil {
				return errors.Wrapf(err, "get abs path %s", opt.Template)
			}
			c.Imports, err = merge(opt.Template, c.Imports)
			if err != nil {
				return errors.Wrapf(err, "merge imports with template")
			}
		}
		if c.Options == nil {
			c.Options = make(map[config.Option]string)
		}
		c.Options[config.OptionCfgPath], err = filepath.Abs(filepath.Dir(path))
		if err != nil {
			return errors.Wrapf(err, "get abs config path %s", path)
		}
		err = StartConfig(c)
		if err != nil {
			return errors.Wrap(err, "test "+path)
		}

		return nil
	}

	doSingle := func(path string) error {

		if strings.HasSuffix(path, ".list") {
			f, err := os.Open(filepath.Clean(path))
			if err != nil {
				return errors.Wrapf(err, "open %s", path)
			}
			defer func() {
				_ = f.Close()
			}()

			dir, err := filepath.Abs(filepath.Dir(path))
			if err != nil {
				return errors.Wrapf(err, "abs of file path %s", path)
			}
			scan := bufio.NewScanner(f)

			for scan.Scan() {
				t := scan.Text()
				fmt.Println("read a line: ", t)
				n := strings.Index(t, "#")
				if n >= 0 {
					t = t[0:n]
				}
				t = strings.TrimSpace(t)
				if len(t) == 0 {
					continue
				}
				if !strings.HasSuffix(t, ".json") {
					continue
				}

				if !filepath.IsAbs(t) {
					t = filepath.Clean(filepath.Join(dir, t))
				}
				err = executor(t)
				if err != nil {
					return errors.Wrapf(err, "list %s file(%s) execute", path, t)
				}
			}
			return nil
		} else if strings.HasSuffix(path, ".json") {
			return executor(path)
		} else {
			fi, err := os.Stat(path)
			if err != nil {
				return errors.Wrapf(err, "Stat file %s", path)
			}
			if !fi.IsDir() {
				return errors.Errorf("%s is not a directory", path)
			}
			return walk(path, executor)
		}
	}

	if len(opt.Final) > 0 {
		defer func() {
			_ = doSingle(opt.Final)
		}()
	}

	if len(opt.Configs) > 0 {
		for _, c := range opt.Configs {
			err = doSingle(c)
			if err != nil {
				return errors.Wrapf(err, "do %s", c)
			}
		}
	} else if hasServer {
		w := sync.WaitGroup{}
		w.Add(1)
		w.Wait()
	}

	return nil
}

var lperf net.Listener

// startPerf will start a server for pprof
func startPerf(port int) (int, error) {
	if lperf != nil {
		return 0, errors.New("already listening")
	}
	addr := "127.0.0.1:" + strconv.Itoa(port)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return 0, err
	}
	if port == 0 {
		port = l.Addr().(*net.TCPAddr).Port
	}

	lperf = l
	go func() {
		_ = http.Serve(lperf, nil)
	}()

	glog.Info("Start perf server at port ", port, " visit http://127.0.0.1:", port, "/debug/pprof to profiling.")

	return port, nil
}

// stopPerf will stop server for pprof
func stopPerf() {
	if lperf != nil {
		glog.Info("stop pprof server")
		_ = lperf.Close()
		lperf = nil
	}
}

type logger struct{}

func (l logger) Write(p []byte) (n int, err error) {
	fmt.Print(string(p))
	return len(p), nil
}

// separateLines will read bytes from an io.Reader and treat it as string separated by '\n',
// and split them so that a line ends with '\n' will be write to io.Writer one by one.
func separateLines(who string, reader io.Reader, writer io.Writer) {
	buf := make([]byte, 4*1024) // buffer of 4K
	saved := 0                  // bytes saved in buf in previous reading
	for {
		tmp := buf[saved:]
		cnt, err := reader.Read(tmp)
		if err != nil {
			glog.Errorf("pipeline of process %v breaks", who)
			break
		}
		if cnt == 0 {
			continue
		}

		base := 0          // real start pos in buf[], including previous lefts
		start := saved     // lookup start position
		end := saved + cnt // position after last
		for i := start; i < end; i++ {
			if buf[i] == 0x0A { // \n
				_, _ = writer.Write(buf[base : i+1])
				base = i + 1
			}
		}

		if base >= end {
			// all written
			saved = 0
		} else if base > 0 { // written some bytes, but also left some bytes
			saved = end - base
			copy(buf[0:saved], buf[base:end])
		} else if cap(buf)-end < 100 { // base is 0, and few bytes left for one line, dump anyway
			saved = 0
			_, _ = writer.Write(buf[0:end])
		} else { // append to last saved
			saved = end
		}
	}
}
func startSubProcess(name string, cmdline string) {
	args := strings.Split(cmdline, " ")
	if len(args) < 0 {
		panic("invalid command line: " + cmdline)
	}

	cmd := exec.Command(args[0], args[1:]...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic("get pipeline for cmdline fail: " + cmdline)
	}
	cmd.Stderr = cmd.Stdout

	if err = cmd.Start(); err != nil {
		glog.Fatalf("start sub process fail: %s, err: %v", cmdline, err)
	}

	log := &logger{}
	separateLines(name, stdout, log)

	_ = cmd.Wait()
	glog.Info(name, " exits")
}

func walk(path string, executor func(s string) error) error {
	rd, err := ioutil.ReadDir(path)
	if err != nil {
		return errors.Wrapf(err, "readdir %s", path)
	}

	for _, fi := range rd {
		pi := filepath.Join(path, fi.Name())
		if fi.IsDir() {
			err = walk(pi, executor)
			if err != nil {
				return err
			}
		} else if strings.HasSuffix(fi.Name(), ".json") {
			err = executor(pi)
			if err != nil {
				return errors.Wrapf(err, "walk to %s", fi.Name())
			}
		}
	}

	return nil
}
func loadConfig(path string) (*config.Config, error) {

	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "read config file")
	}

	var cfg config.Config
	err = json.Unmarshal(b, &cfg)
	if err != nil {
		return nil, errors.Wrap(err, "unmarshal json")
	}

	return &cfg, nil
}
