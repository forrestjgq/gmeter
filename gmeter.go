package main

import (
	"errors"
	"flag"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strconv"

	"github.com/forrestjgq/gmeter/internal/meter"

	"github.com/golang/glog"
)

func main() {
	cfg := ""
	httpsrv := ""
	flag.StringVar(&cfg, "config", "", "config file path")
	flag.StringVar(&httpsrv, "httpsrv", "", "config file path for http server")
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
	err = meter.Start(cfg)
	if err != nil {
		glog.Fatalf("test failed: %+v", err)
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
