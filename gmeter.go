package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"

	"github.com/forrestjgq/gmeter/config"
	"github.com/pkg/errors"

	"github.com/forrestjgq/gmeter/internal/arcee"
	"github.com/forrestjgq/gmeter/internal/meter"

	"github.com/golang/glog"
)

type logger struct{}

func (l logger) Write(p []byte) (n int, err error) {
	fmt.Print(string(p))
	return len(p), nil
}

// SeparateLines will read bytes from an io.Reader and treat it as string separated by '\n',
// and split them so that a line ends with '\n' will be write to io.Writer one by one.
func SeparateLines(who string, reader io.Reader, writer io.Writer) {
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
		panic("start sub process fail: " + cmdline)
	}

	log := &logger{}
	SeparateLines(name, stdout, log)

	_ = cmd.Wait()
	glog.Info(name, " exits")
}

func walk(path string, executor func(s string)) {
	rd, err := ioutil.ReadDir(path)
	if err != nil {
		glog.Fatalf("readdir %s: %v", path, err)
	}

	for _, fi := range rd {
		pi := filepath.Join(path, fi.Name())
		if fi.IsDir() {
			walk(pi, executor)
		} else if strings.HasSuffix(fi.Name(), ".json") {
			executor(pi)
		}
	}

}
func loadCfg(path string) (*config.Config, error) {

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

func override(template, cfg *config.Config) {
	if template == nil {
		return
	}

	for k, v := range template.Hosts {
		if cfg.Hosts != nil {
			if _, ok := cfg.Hosts[k]; ok {
				// already defined, skip
				continue
			}
		} else {
			cfg.Hosts = make(map[string]*config.Host)
		}

		cfg.Hosts[k] = v
	}
	for k, v := range template.Messages {
		if cfg.Messages != nil {
			if _, ok := cfg.Messages[k]; ok {
				// already defined, skip
				continue
			}
		} else {
			cfg.Messages = make(map[string]*config.Request)
		}

		cfg.Messages[k] = v
	}
	for k, v := range template.Tests {
		if cfg.Tests != nil {
			if _, ok := cfg.Tests[k]; ok {
				// already defined, skip
				continue
			}
		} else {
			cfg.Tests = make(map[string]*config.Test)
		}

		cfg.Tests[k] = v
	}
	for k, v := range template.Env {
		if cfg.Env != nil {
			if _, ok := cfg.Env[k]; ok {
				// already defined, skip
				continue
			}
		} else {
			cfg.Env = make(map[string]string)
		}

		cfg.Env[k] = v
	}
	for k, v := range template.Options {
		if cfg.Options != nil {
			if _, ok := cfg.Options[k]; ok {
				// already defined, skip
				continue
			}
		} else {
			cfg.Options = make(map[config.Option]string)
		}

		cfg.Options[k] = v
	}
}
func run() {
	cfg := ""
	httpsrv := ""
	arceeCfg := ""
	call := ""
	template := ""
	flag.StringVar(&template, "template", "", "template config file path")
	flag.StringVar(&cfg, "config", "", "config file path, could be a .json, or .list, or a directory")
	flag.StringVar(&httpsrv, "httpsrv", "", "config file path for http server")
	flag.StringVar(&arceeCfg, "arcee", "", "arcee configuration file path")
	flag.StringVar(&call, "call", "", "extra program command line")
	flag.Parse()

	_, err := StartPerf(0)
	if err != nil {
		defer StopPerf()
	}

	hasServer := false
	if len(httpsrv) > 0 {
		err := meter.StartHTTPServer(httpsrv)
		if err != nil {
			glog.Fatalf("HTTP server start failed: %+v", err)
		} else {
			hasServer = true
		}
		defer func() {
			meter.StopAll()
		}()
	}

	if len(arceeCfg) > 0 {
		_, err = arcee.StartArcee(arceeCfg)
		if err != nil {
			glog.Fatalf("start arcee fail: %+v", err)
		} else {
			hasServer = true
		}
	}

	if len(call) > 0 {
		go func() {
			startSubProcess("child", call)
		}()
	}

	var baseCfg *config.Config
	if len(template) > 0 {
		baseCfg, err = loadCfg(template)
		if err != nil {
			glog.Fatalf("load template config %s fail, err: %+v", template, err)
		}
	}

	executor := func(path string) {
		fmt.Println("gmeter starts ", path)
		c, err := loadCfg(path)
		if err != nil {
			glog.Fatalf("load config %s fail, err: %+v", path, err)
		}
		if baseCfg != nil {
			override(baseCfg, c)
		}
		if c.Options == nil {
			c.Options = make(map[config.Option]string)
		}
		c.Options[config.OptionCfgPath], err = filepath.Abs(filepath.Dir(path))
		if err != nil {
			glog.Fatalf("get abs config path %s fail, err %+v", path, err)
		}
		err = meter.StartConfig(c)
		if err != nil {
			glog.Fatalf("test failed: %+v", err)
		}

	}
	if len(cfg) > 0 {
		if strings.HasSuffix(cfg, ".list") {
			f, err := os.Open(filepath.Clean(cfg))
			if err != nil {
				glog.Fatalf("open %s fail, err: %v", cfg, err)
			}
			defer func() {
				_ = f.Close()
			}()

			dir, err := filepath.Abs(filepath.Dir(cfg))
			if err != nil {
				glog.Fatalf("abs of file path %s fail, err: %v", cfg, err)
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
				executor(t)
			}
		} else if strings.HasSuffix(cfg, ".json") {
			executor(cfg)
		} else {
			fi, err := os.Stat(cfg)
			if err != nil {
				glog.Fatalf("Stat file %s: %v", cfg, err)
			}
			if !fi.IsDir() {
				glog.Fatalf("%s is not a directory", cfg)
			}
			walk(cfg, executor)
		}
	} else {
		left := flag.Args()
		if len(left) > 0 {
			for _, c := range left {
				executor(c)
			}
		} else if hasServer {
			w := sync.WaitGroup{}
			w.Add(1)
			w.Wait()
		}
	}
}
func main() {
	run()

	if false {
		buf := make([]byte, 500*1024)
		n := runtime.Stack(buf, true)
		buf = buf[0:n]
		fmt.Print(string(buf))
	}
}

var lperf net.Listener

// StartPerf will start a server for pprof
func StartPerf(port int) (int, error) {
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

// StopPerf will stop server for pprof
func StopPerf() {
	if lperf != nil {
		glog.Info("stop pprof server")
		_ = lperf.Close()
		lperf = nil
	}
}
