package meter_test

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/forrestjgq/gmeter/internal/meter"

	"github.com/forrestjgq/gmeter/config"

	"github.com/gorilla/mux"
)

type mockServer struct {
	rspBody []byte
	r       *mux.Router
	s       *http.Server
	port    string
}

func examplePath() string {
	_, f, _, _ := runtime.Caller(0)
	d := filepath.Dir(f) + "/../../example"
	return filepath.Clean(d)
}
func readExample(name string) ([]byte, error) {
	path := examplePath() + "/" + name
	return ioutil.ReadFile(path)
}
func (m *mockServer) start(rspFile string) error {
	b, err := readExample(rspFile)
	if err != nil {
		return err
	}
	m.rspBody = b

	m.r = mux.NewRouter()
	m.r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(m.rspBody)
	})

	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return err
	}
	m.s = &http.Server{
		Handler: m.r,
	}
	go m.s.Serve(l)
	list := strings.Split(l.Addr().String(), ":")
	m.port = list[len(list)-1]
	return nil
}
func (m *mockServer) stop() {
	_ = m.s.Close()
}

func TestStart(t *testing.T) {
	m := &mockServer{}
	err := m.start("ai_res.json")
	if err != nil {
		t.Fatalf(err.Error())
	}

	defer m.stop()

	b, err := readExample("ai.json")
	if err != nil {
		t.Fatalf(err.Error())
	}

	cfg := &config.Config{}
	err = json.Unmarshal(b, cfg)
	if err != nil {
		t.Fatalf(err.Error())
	}

	cfg.Messages["req"].Path = "/"
	cfg.Hosts["vse"].Host = "http://127.0.0.1:" + m.port
	dir := t.TempDir()
	cfg.Schedules[0].Reporter.Path = dir + "/report.log"

	cfg.Options[config.OptionCfgPath] = examplePath()
	meter.StartConfig(cfg)

	b, err = ioutil.ReadFile(dir + "/report.log")
	if err != nil {
		t.Fatalf(err.Error())
	}

	fmt.Print(string(b))
}
func TestStartConcurrent(t *testing.T) {
	m := &mockServer{}
	err := m.start("ai_res.json")
	if err != nil {
		t.Fatalf(err.Error())
	}

	defer m.stop()

	b, err := readExample("concurrent.json")
	if err != nil {
		t.Fatalf(err.Error())
	}

	cfg := &config.Config{}
	err = json.Unmarshal(b, cfg)
	if err != nil {
		t.Fatalf(err.Error())
	}

	cfg.Messages["req"].Path = "/"
	cfg.Hosts["vse"].Host = "http://127.0.0.1:" + m.port
	dir := t.TempDir()
	cfg.Schedules[0].Reporter.Path = dir + "/report.log"

	cfg.Options[config.OptionCfgPath] = examplePath()
	meter.StartConfig(cfg)

}
func TestStartIterable(t *testing.T) {
	m := &mockServer{}
	err := m.start("ai_res.json")
	if err != nil {
		t.Fatalf(err.Error())
	}

	defer m.stop()

	dir := t.TempDir()
	list := dir + "/list"
	s := "/file/path/to/iterable"
	var strs []string
	for i := 0; i < 1000; i++ {
		strs = append(strs, s)
	}
	err = ioutil.WriteFile(list, []byte(strings.Join(strs, "\n")), os.ModePerm)
	if err != nil {
		t.Fatalf(err.Error())
	}

	b, err := readExample("sample.json")
	if err != nil {
		t.Fatalf(err.Error())
	}

	cfg := &config.Config{}
	err = json.Unmarshal(b, cfg)
	if err != nil {
		t.Fatalf(err.Error())
	}

	cfg.Messages["req"].Path = "/"
	cfg.Hosts["vse"].Host = "http://127.0.0.1:" + m.port
	cfg.Tests["recognize"].PreProcess[0] = "`list " + list + " | envw JSON`"

	cfg.Options[config.OptionCfgPath] = examplePath()
	meter.StartConfig(cfg)

}
