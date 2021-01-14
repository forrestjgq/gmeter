package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os/exec"
	"runtime"
	"strconv"
	"strings"

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

func run() {
	cfg := ""
	httpsrv := ""
	arceeCfg := ""
	call := ""
	flag.StringVar(&cfg, "config", "", "config file path")
	flag.StringVar(&httpsrv, "httpsrv", "", "config file path for http server")
	flag.StringVar(&arceeCfg, "arcee", "", "arcee configuration file path")
	flag.StringVar(&call, "call", "", "extra program command line")
	flag.Parse()

	if len(cfg) == 0 {
		glog.Fatalf("config file not present")
	}

	_, err := StartPerf(0)
	if err != nil {
		defer StopPerf()
	}

	if len(httpsrv) > 0 {
		err := meter.StartHTTPServer(httpsrv)
		if err != nil {
			glog.Fatalf("HTTP server start failed: %+v", err)
		}
		defer func() {
			meter.StopAll()
		}()
	}

	if len(arceeCfg) > 0 {
		_, err = arcee.StartArcee(arceeCfg)
		if err != nil {
			glog.Fatalf("start arcee fail: %+v", err)
		}
	}

	if len(call) > 0 {
		go func() {
			startSubProcess("child", call)
		}()
	}

	err = meter.Start(cfg)
	if err != nil {
		glog.Fatalf("test failed: %+v", err)
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
