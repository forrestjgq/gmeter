package gmeter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/forrestjgq/gmeter"
)

type pingHandler struct {
	idx int64
}

func (p *pingHandler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if atomic.AddInt64(&p.idx, 1)%1000 == 0 {
		fmt.Printf("ping %d\n", p.idx)
	}
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
		Name:      "test",
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
		Count:        100000,
		CountForEach: false,
		Concurrency:  10,
	})

	time.Sleep(1 * time.Second)

	err := gmeter.Run(7777, cfg)
	if err != nil {
		t.Error(err)
	}

	server.Shutdown(context.Background())
}

func TestRecog(t *testing.T) {
	cfg := &gmeter.Config{
		Name:      "test",
		Mode:      gmeter.RunOneByOne,
		Hosts:     nil,
		Messages:  nil,
		Tests:     nil,
		Schedules: nil,
		Options:   nil,
	}

	h := &gmeter.Host{
		Host:  "http://127.0.0.1:8009",
		Proxy: "",
	}
	cfg.AddHost("test", h)

	cfg.AddMessage("ping", &gmeter.Message{
		Path:    "/ai/detect/all",
		Method:  "POST",
		Headers: nil,
		Params:  nil,
		Body: json.RawMessage(`{
  "image": {
      "url": "/mnt/cephfs/vsec/vsecTestData/upload/sence.jpg"
  }
}`),
	})

	cfg.AddTest("test", &gmeter.Test{
		Host:          "test",
		Request:       "ping",
		ResponseCheck: nil,
		Timeout:       "3s",
	})

	cfg.AddSchedule("ping-test", &gmeter.Schedule{
		Series:       []string{"test"},
		Count:        100000,
		CountForEach: false,
		Concurrency:  10,
	})

	time.Sleep(1 * time.Second)

	err := gmeter.Run(7777, cfg)
	if err != nil {
		t.Error(err)
	}

}
func TestName(t *testing.T) {
	s := "abc不行啊"
	s1 := s[1:4]

	r := []rune(s)
	r1 := r[1:4]
	s2 := string(r1)

	t.Logf("s1: %s s2: %s", s1, s2)
}
