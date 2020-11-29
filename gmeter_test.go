package gmeter_test

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/forrest/gmeter"
)

type pingHandler struct {
	idx int
}

func (p *pingHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if p.idx%1000 == 0 {
		fmt.Printf("ping %d\n", p.idx)
	}
	p.idx++
	_, _ = writer.Write([]byte(`{"status": "OK"}`))
}

func startHttpServer() (*http.Server, int) {
	handlePing := &pingHandler{}
	http.Handle("/ping", handlePing)
	s := &http.Server{}
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		panic("listen fail")
	}
	go s.Serve(lis)
	return s, lis.Addr().(*net.TCPAddr).Port
}
func TestRunGMeter(t *testing.T) {
	server, port := startHttpServer()
	cfg := &gmeter.Config{
		Mode:      gmeter.RunOneByOne,
		Hosts:     nil,
		Messages:  nil,
		Tests:     nil,
		Schedules: nil,
		Options:   nil,
	}

	h := &gmeter.Host{
		Host:  "http://127.0.0.1:" + strconv.Itoa(port),
		Proxy: "",
	}
	cfg.AddHost("test", h)

	cfg.AddMessage("ping", &gmeter.Message{
		Path:    "/ping",
		Method:  "GET",
		Headers: nil,
		Params:  nil,
		Body:    nil,
	})

	cfg.AddTest("test", &gmeter.Test{
		Host:          "test",
		Request:       "ping",
		ResponseCheck: nil,
		Timeout:       "1s",
	})

	cfg.AddSchedule("ping-test", &gmeter.Schedule{
		Series:       []string{"test"},
		Count:        10000,
		CountForEach: false,
		Concurrency:  0,
	})

	time.Sleep(1 * time.Second)

	err := gmeter.Run(0, cfg)
	if err != nil {
		t.Error(err)
	}

	server.Shutdown(context.Background())
}
