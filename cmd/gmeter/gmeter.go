package main

import (
	"errors"
	"net"
	"net/http"
	_ "net/http/pprof"
	"strconv"

	"github.com/golang/glog"
)

type one struct {
	t *two
	b []int
}
type two struct {
	t *one
	b []int
}

func main() {
	StartPerf(0)
	for i := 0; i < 1000000000; i++ {
		a := &one{
			t: nil,
			b: make([]int, 1000),
		}
		b := &two{
			t: nil,
			b: make([]int, 1000),
		}
		for k := range a.b {
			a.b[k] = k + i
		}
		for k := range b.b {
			b.b[k] = k + i + 1
		}
		a.t = b
		b.t = a
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
