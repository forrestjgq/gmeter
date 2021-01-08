package meter_test

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/forrestjgq/gmeter/internal/meter"

	"github.com/forrestjgq/gmeter/config"

	"github.com/gorilla/mux"
)

type mockServer struct {
	rspBody []byte
	r       *mux.Router
	s       *http.Server
	port    string
	delay   int
	fruit   *TestFruit
}

type TestFruit struct {
	Fruit string
	Qty   int
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
	if len(rspFile) > 0 {
		b, err := readExample(rspFile)
		if err != nil {
			return err
		}
		m.rspBody = b
	}

	m.r = mux.NewRouter()
	m.r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if m.delay > 0 {
			time.Sleep(time.Duration(m.delay) * time.Second)
		}
		_, _ = w.Write(m.rspBody)
	})
	m.r.HandleFunc("/add", func(w http.ResponseWriter, r *http.Request) {
		var f TestFruit
		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(400)
			return
		}
		_ = r.Body.Close()

		err = json.Unmarshal(b, &f)
		if err != nil {
			w.WriteHeader(400)
			return
		}

		m.fruit = &f
	})
	m.r.HandleFunc("/query", func(w http.ResponseWriter, r *http.Request) {
		if m.fruit == nil {
			w.WriteHeader(500)
			return
		}
		b, err := json.Marshal(m.fruit)
		if err != nil {
			w.WriteHeader(400)
			return
		}
		_, _ = w.Write(b)
	})
	m.r.HandleFunc("/del", func(w http.ResponseWriter, r *http.Request) {
		if m.fruit == nil {
			w.WriteHeader(500)
			return
		}
		m.fruit = nil
	})

	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return err
	}
	m.s = &http.Server{
		Handler: m.r,
	}
	go func() {
		_ = m.s.Serve(l)
	}()
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
	err = meter.StartConfig(cfg)
	if err != nil {
		t.Fatalf("failed: %+v", err)
	}

	//b, err = ioutil.ReadFile(dir + "/report.log")
	//if err != nil {
	//	t.Fatalf(err.Error())
	//}
	//fmt.Print(string(b))
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
	err = meter.StartConfig(cfg)
	if err != nil {
		t.Fatalf("failed: %+v", err)
	}

}
func TestConfigFile(t *testing.T) {
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
	cfg.Schedules[0].Reporter.Path = dir + "/report.log"

	cfgContent, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf(err.Error())
	}

	path := dir + "/config.json"
	_ = ioutil.WriteFile(path, cfgContent, os.ModePerm)

	err = meter.Start(path)
	if err != nil {
		t.Fatalf("failed: %+v", err)
	}

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
	cfg.Schedules[0].Reporter.Path = dir + "/report.log"

	cfg.Options[config.OptionCfgPath] = examplePath()
	err = meter.StartConfig(cfg)
	if err != nil {
		t.Fatalf("failed: %+v", err)
	}

}
func TestStartTimeout(t *testing.T) {
	m := &mockServer{}
	err := m.start("ai_res.json")
	if err != nil {
		t.Fatalf(err.Error())
	}

	defer m.stop()
	m.delay = 10

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

	cfg.Tests["recognize"].Timeout = "3s"
	cfg.Messages["req"].Path = "/"
	cfg.Hosts["vse"].Host = "http://127.0.0.1:" + m.port
	cfg.Tests["recognize"].PreProcess[0] = "`list " + list + " | envw JSON`"
	cfg.Schedules[0].Reporter.Path = dir + "/report.log"

	cfg.Options[config.OptionCfgPath] = examplePath()
	err = meter.StartConfig(cfg)
	if err == nil {
		t.Fatal("expect a failure")
	}

}
func TestStartMultiTest(t *testing.T) {
	m := &mockServer{}
	err := m.start("")
	if err != nil {
		t.Fatalf(err.Error())
	}

	defer m.stop()

	b, err := readExample("multiple.json")
	if err != nil {
		t.Fatalf(err.Error())
	}

	cfg := &config.Config{}
	err = json.Unmarshal(b, cfg)
	if err != nil {
		t.Fatalf(err.Error())
	}

	cfg.Hosts["vse"].Host = "http://127.0.0.1:" + m.port
	err = meter.StartConfig(cfg)
	if err != nil {
		t.Fatal("expect a failure")
	}

}
func TestStartURLExplicit(t *testing.T) {
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

	b, err := readExample("explicit_host.json")
	if err != nil {
		t.Fatalf(err.Error())
	}

	cfg := &config.Config{}
	err = json.Unmarshal(b, cfg)
	if err != nil {
		t.Fatalf(err.Error())
	}

	cfg.Messages["req"].Path = "/"
	cfg.Tests["recognize"].Host = "http://127.0.0.1:" + m.port
	cfg.Tests["recognize"].PreProcess[0] = "`list " + list + " | envw JSON`"
	cfg.Schedules[0].Reporter.Path = dir + "/report.log"

	cfg.Options[config.OptionCfgPath] = examplePath()
	err = meter.StartConfig(cfg)
	if err != nil {
		t.Fatalf("failed: %+v", err)
	}

}
func TestStartMultiSchedule(t *testing.T) {
	m := &mockServer{}
	err := m.start("")
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

	b, err := readExample("multiple_schedule.json")
	if err != nil {
		t.Fatalf(err.Error())
	}

	cfg := &config.Config{}
	err = json.Unmarshal(b, cfg)
	if err != nil {
		t.Fatalf(err.Error())
	}

	cfg.Hosts["vse"].Host = "http://127.0.0.1:" + m.port
	err = meter.StartConfig(cfg)
	if err != nil {
		t.Fatalf("failed: %+v", err)
	}

}