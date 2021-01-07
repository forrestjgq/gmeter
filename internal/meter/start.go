package meter

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/golang/glog"

	"github.com/forrestjgq/gmeter/config"
)

var hosts = make(map[string]*http.Client)

func loadFilePath(cfg *config.Config, path string) (string, error) {
	cpath := filepath.Clean(path)
	if len(path) == 0 {
		return "", errors.New("file path invalid")
	}

	if cpath[0] != '/' {
		cpath = cfg.Options[config.OptionCfgPath] + "/" + cpath
	}

	return cpath, nil
}
func loadFile(cfg *config.Config, path string) ([]byte, error) {
	cpath, err := loadFilePath(cfg, path)
	if err != nil {
		return nil, err
	}

	f, err := os.Open(cpath)
	if err != nil {
		return nil, err
	}

	b, err := ioutil.ReadAll(f)
	_ = f.Close()
	return b, err
}
func loadHTTPClient(h *config.Host, timeout string) (*http.Client, error) {
	key := h.Proxy + "|" + h.Host + "|" + timeout
	if host, ok := hosts[key]; !ok {
		host := &http.Client{}
		if len(timeout) != 0 {
			du, err := time.ParseDuration(timeout)
			if err != nil {
				return nil, err
			}
			host.Timeout = du
		}
		if len(h.Proxy) > 0 {
			proxy := func(_ *http.Request) (*url.URL, error) {
				return url.Parse(h.Proxy)
			}
			transport := &http.Transport{
				Proxy: proxy,
			}
			host.Transport = transport
		}
		hosts[key] = host
		return host, nil
	} else {
		return host, nil
	}
}
func createDefaultBackground() (*background, error) {
	bg := &background{
		name:   "default",
		local:  makeSimpEnv(),
		global: makeSimpEnv(),
	}

	var err error
	var path string
	path, err = filepath.Abs(filepath.Dir("."))
	if err != nil {
		return nil, err
	}

	bg.setGlobalEnv(KeySchedule, "default-schedule")
	bg.setGlobalEnv(KeyTPath, path)
	bg.setGlobalEnv(KeyConfig, "default")
	return bg, nil
}
func createBackground(cfg *config.Config, sched *config.Schedule) (*background, error) {
	bg := &background{
		name:   cfg.Name,
		local:  makeSimpEnv(),
		global: makeSimpEnv(),
	}

	bg.setGlobalEnv(KeySchedule, sched.Name)
	bg.setGlobalEnv(KeyTPath, cfg.Options[config.OptionCfgPath])
	bg.setGlobalEnv(KeyConfig, cfg.Name)
	if sched.Env != nil {
		bg.predefineLocalEnv(sched.Env)
	}

	if debug, ok := cfg.Options[config.OptionDebug]; ok {
		bg.setGlobalEnv(KeyDebug, debug)
	}

	// report
	var err error
	if len(sched.Reporter.Path) > 0 {
		sched.Reporter.Path, err = loadFilePath(cfg, sched.Reporter.Path)
		if err != nil {
			return nil, err
		}
	}
	bg.rpt, err = makeReporter(&sched.Reporter)
	if err != nil {
		return nil, err
	}
	return bg, nil
}
func create(cfg *config.Config) []*plan {
	if len(cfg.Schedules) == 0 {
		glog.Fatalf("no schedule is defined")
	}
	var plans []*plan

	for _, s := range cfg.Schedules {
		bg, err := createBackground(cfg, s)
		if err != nil {
			glog.Fatalf("schedule %s create background fail, err: %v", s.Name, err)
		}
		tests := strings.Split(s.Tests, "|")
		if len(tests) == 0 {
			glog.Fatalf("schedule %s contains no tests", s.Name)
		}

		var runners []runnable
		for _, name := range tests {
			t, ok := cfg.Tests[name]
			if !ok {
				glog.Fatalf("test %s not found", name)
			}

			var h *config.Host
			var req *config.Request
			rsp := t.Response

			// host
			h, ok = cfg.Hosts[t.Host]
			if !ok {
				urls := strings.Split(t.Host, "|")
				if len(urls) == 0 || len(urls) > 2 {
					glog.Fatal("unknown host definition: ", t.Host)
				}
				h := &config.Host{}

				if len(urls) == 1 {
					h.Host = urls[0]
				} else {
					h.Proxy = urls[0]
					h.Host = urls[1]
				}
			}
			if err = h.Check(); err != nil {
				glog.Fatalf("host %s check fail: %v", t.Host, err)
			}
			client, err := loadHTTPClient(h, t.Timeout)
			if err != nil {
				glog.Fatalf("load http client fail, err: %v", err)
			}

			// request
			str := t.Request
			if len(str) == 0 {
				glog.Fatalf("request missing in test %s ", name)
			}
			req, ok = cfg.Messages[str]
			if !ok {
				if len(str) > 2 && str[0] == '`' && str[len(str)-1] == '`' {
					b, err := loadFile(cfg, str[1:len(str)-1])
					if err != nil {
						glog.Fatalf("test %s has invalid path %s", name, str)
					} else {
						str = string(b)
					}
				} else {
					glog.Fatalf("unexpected request %s", str)
				}

				req = &config.Request{}
				err := json.Unmarshal([]byte(str), req)
				if err != nil {
					glog.Fatalf("test %s has invalid request", name)
				}
			}

			if err := req.Check(); err != nil {
				glog.Fatalf("test %s request check failed: %v", name, err)
			}

			m := make(map[string]string)
			m[string(catMethod)] = req.Method
			m[string(catURL)] = h.Host + req.Path
			m[string(catBody)] = string(req.Body)
			for k, v := range req.Headers {
				m[k] = v
			}

			if s.Count == 0 {
				s.Count = math.MaxUint64 - 1
			}

			feeder, err := makeDynamicFeeder(m, s.Count, t.PreProcess)
			if err != nil {
				glog.Fatalf("test %s create feeder fail, err: %v", name, err)
			}

			prv, err := makeFeedProvider(feeder)
			if err != nil {
				glog.Fatalf("test %s create provider fail, err: %v", name, err)
			}

			var csm consumer
			decision := ignoreOnFail
			if cfg.Options[config.OptionAbortIfFail] == "true" {
				decision = abortOnFail
			}
			if rsp != nil {
				csm, err = makeDynamicConsumer(rsp.Check, rsp.Success, rsp.Failure, rsp.Template, decision)
				if err != nil {
					glog.Fatalf("make test %s consumer fail, err %+v", name, errors.Cause(err))
				}
			}

			runner, err := makeRunner(name, prv, client, csm)
			if err != nil {
				glog.Fatalf("make test %s runner fail, err %v", name, err)
			}
			runners = append(runners, runner)
		}

		if len(runners) == 0 {
			glog.Fatalf("schedule %s does not define any tests", s.Name)
		}

		run := assembleRunners(runners...)
		p := &plan{
			name:       s.Name,
			target:     run,
			bg:         bg,
			concurrent: 1,
		}
		if s.Concurrency > 1 {
			p.concurrent = s.Concurrency
		}

		if len(s.PreProcess) > 0 {
			p.preprocess, err = makeGroup(s.PreProcess, false)
			if err != nil {
				glog.Fatalf("schedule %s make preprocesss fail, err: %v", s.Name, err)
			}
		}
		plans = append(plans, p)
	}

	return plans
}

func StartConfig(cfg *config.Config) {
	plans := create(cfg)

	type result struct {
		name string
		res  next
	}
	// save result
	results := make(map[string]next)

	if cfg.Mode == config.RunConcurrent {
		c := make(chan result)
		for _, p := range plans {
			go func(t *plan) {
				n := t.run()
				c <- result{
					name: t.name,
					res:  n,
				}
			}(p)
		}

		for r := range c {
			results[r.name] = r.res
			if len(results) == len(plans) {
				break
			}
		}
	} else {
		for _, p := range plans {
			n := p.run()
			results[p.name] = n
		}
	}

	for _, p := range plans {
		p.close()
	}

	fmt.Println("--------------------------------")
	fmt.Printf("test %s done:\n", cfg.Name)
	failed := false
	for k, v := range results {
		str := "success"
		if v != nextFinished {
			str = "fail"
			failed = true
		}
		fmt.Printf("\t%s: %s\n", k, str)
	}

	if failed {
		os.Exit(128)
	}
}

// Start a test, path is the configure json file path, which must be able to be
// unmarshal to config.Config
func Start(path string) {
	f, err := os.Open(path)
	if err != nil {
		glog.Fatal("config open fail, ", err)
		panic(err)
	}
	b, err := ioutil.ReadAll(f)
	_ = f.Close()
	if err != nil {
		glog.Fatal("read config file fail, ", err)
	}

	var cfg config.Config
	err = json.Unmarshal(b, &cfg)
	if err != nil {
		glog.Fatal("unmarshal json fail, ", err)
	}

	cfg.Options[config.OptionCfgPath], err = filepath.Abs(filepath.Dir(path))
	if err != nil {
		glog.Fatalf("get config path(%s) fail: %v", path, err)
	}

	StartConfig(&cfg)
}
