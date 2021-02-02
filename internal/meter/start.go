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

	"github.com/forrestjgq/gmeter/config"
)

var hosts = make(map[string]*http.Client)

func loadFilePath(root string, path string) (string, error) {
	cpath := filepath.Clean(path)
	if len(path) == 0 {
		return "", errors.New("file path invalid")
	}

	if cpath[0] != '/' {
		if len(root) > 0 {
			cpath = root + "/" + cpath
		}
	}

	return cpath, nil
}
func loadHTTPClient(h *config.Host, timeout string) (*http.Client, error) {
	key := h.Proxy + "|" + h.Host + "|" + timeout
	if host, ok := hosts[key]; !ok {
		host := &http.Client{}
		if len(timeout) != 0 {
			du, err := time.ParseDuration(timeout)
			if err != nil {
				return nil, errors.Wrapf(err, "parse timeout %s", timeout)
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
		db:     createDB(),
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
	str, err := os.Getwd()
	if err == nil {
		bg.setGlobalEnv(KeyCWD, str)
	}
	return bg, nil
}
func createBackground(cfg *config.Config, sched *config.Schedule) (*background, error) {
	bg := &background{
		name:   cfg.Name,
		db:     createDB(),
		local:  makeSimpEnv(),
		global: makeSimpEnv(),
	}

	bg.setGlobalEnv(KeySchedule, sched.Name)
	bg.setGlobalEnv(KeyTPath, cfg.Options[config.OptionCfgPath])
	bg.setGlobalEnv(KeyConfig, cfg.Name)
	str, err := os.Getwd()
	if err == nil {
		bg.setGlobalEnv(KeyCWD, str)
	}
	if sched.Env != nil {
		bg.predefineLocalEnv(sched.Env)
	}

	if debug, ok := cfg.Options[config.OptionDebug]; ok {
		bg.setGlobalEnv(KeyDebug, debug)
	}
	for k, v := range cfg.Env {
		if k != "" {
			bg.setGlobalEnv(k, v)
		}
	}

	// report
	if len(sched.Reporter.Path) > 0 {
		s, err := makeSegments(sched.Reporter.Path)
		if err != nil {
			return nil, errors.Wrapf(err, "make report path")
		}
		sched.Reporter.Path, err = s.compose(bg)
		if err != nil {
			return nil, errors.Wrapf(err, "compose report path")
		}
		sched.Reporter.Path, err = loadFilePath(cfg.Options[config.OptionCfgPath], sched.Reporter.Path)
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
func constructTest(t, base *config.Test) *config.Test {
	t = t.Dup()
	if len(t.Host) == 0 && len(base.Host) > 0 {
		t.Host = base.Host
	}
	if len(t.Request) == 0 && t.RequestMessage == nil {
		if base.RequestMessage != nil {
			t.RequestMessage = base.RequestMessage
		} else if len(base.Request) > 0 {
			t.Request = base.Request
		}
	}
	if len(base.PreProcess) > 0 {
		t.PreProcess = append(base.PreProcess, t.PreProcess...)
	}
	if len(t.Timeout) == 0 && len(base.Timeout) > 0 {
		t.Timeout = base.Timeout
	}
	if t.Response == nil {
		if base.Response != nil {
			t.Response = base.Response
		}
	} else if base.Response != nil {
		src, dst := t.Response, base.Response
		if len(src.Template) == 0 && len(dst.Template) > 0 {
			src.Template = dst.Template
		}
		if len(dst.Success) > 0 {
			src.Success = append(dst.Success, src.Success...)
		}
		if len(dst.Check) > 0 {
			src.Check = append(dst.Check, src.Check...)
		}
		if len(dst.Failure) > 0 {
			src.Failure = append(dst.Failure, src.Failure...)
		}
	}
	return t
}
func create(cfg *config.Config) ([]*plan, error) {
	if len(cfg.Schedules) == 0 {
		return nil, errors.Errorf("no schedule is defined")
	}
	var plans []*plan

	for _, s := range cfg.Schedules {
		bg, err := createBackground(cfg, s)
		if err != nil {
			return nil, errors.Wrapf(err, "schedule %s create background ", s.Name)
		}
		tests := strings.Split(s.Tests, "|")
		if len(tests) == 0 {
			return nil, errors.Errorf("schedule %s contains no tests", s.Name)
		}

		var baseTest *config.Test
		if len(s.TestBase) > 0 {
			t, ok := cfg.Tests[s.TestBase]
			if !ok && t != nil {
				return nil, errors.Errorf("test base %s not found", s.TestBase)
			}
			baseTest = t
		}

		var runners []runnable
		for _, name := range tests {
			t, ok := cfg.Tests[name]
			if !ok || t == nil {
				return nil, errors.Errorf("test %s not found", name)
			}

			if baseTest != nil {
				t = constructTest(t, baseTest)
			}

			var h *config.Host
			var req *config.Request
			rsp := t.Response

			// host
			if t.Host == "" {
				if len(cfg.Hosts) == 1 {
					for k := range cfg.Hosts {
						t.Host = k
						break
					}
				} else {
					t.Host = "-" // "-" is the default host
				}
			}
			h, ok = cfg.Hosts[t.Host]
			if !ok {
				urls := strings.Split(t.Host, "|")
				if len(urls) == 0 || len(urls) > 2 {
					return nil, errors.Errorf("unknown host definition: %s", t.Host)
				}
				h = &config.Host{}

				if len(urls) == 1 {
					h.Host = urls[0]
				} else {
					h.Proxy = urls[0]
					h.Host = urls[1]
				}
			}
			if err = h.Check(); err != nil {
				return nil, errors.Wrapf(err, "host %s check", t.Host)
			}

			if len(t.Timeout) == 0 {
				if du, ok := s.Env["TIMEOUT"]; ok {
					t.Timeout = du
				}
			}
			if len(t.Timeout) == 0 {
				t.Timeout = "1m"
			}

			client, err := loadHTTPClient(h, t.Timeout)
			if err != nil {
				return nil, errors.Wrap(err, "load http client")
			}

			// request
			req = t.RequestMessage
			if req == nil {
				str := t.Request
				if len(str) > 0 {
					req, ok = cfg.Messages[str]
					if !ok {
						return nil, errors.Errorf("unexpected request %s", str)
					}
				}
			}
			if req == nil {
				return nil, errors.Errorf("test %s misses request", name)
			}

			if req.Method == "" {
				req.Method = "GET"
			}

			if err := req.Check(); err != nil {
				return nil, errors.Wrapf(err, "test %s request check", name)
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
				return nil, errors.Wrapf(err, "test %s create feeder", name)
			}

			prv, err := makeFeedProvider(feeder)
			if err != nil {
				return nil, errors.Wrapf(err, "test %s create provider", name)
			}

			var csm consumer
			decision := ignoreOnFail
			if cfg.Options[config.OptionAbortIfFail] == "true" {
				decision = abortOnFail
			}
			if rsp != nil {
				csm, err = makeDynamicConsumer(rsp.Check, rsp.Success, rsp.Failure, rsp.Template, decision)
				if err != nil {
					return nil, errors.Wrapf(err, "make test %s consumer", name)
				}
			}

			runner, err := makeRunner(name, prv, client, csm)
			if err != nil {
				return nil, errors.Wrapf(err, "make test %s runner", name)
			}
			runners = append(runners, runner)
		}

		if len(runners) == 0 {
			return nil, errors.Errorf("schedule %s does not define any tests", s.Name)
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
				return nil, errors.Wrapf(err, "schedule %s make preprocesss", s.Name)
			}
		}
		plans = append(plans, p)
	}

	return plans, nil
}

func loadCfg(root, path string) (*config.Config, error) {
	p, err := loadFilePath(root, path)
	if err != nil {
		return nil, errors.Wrapf(err, "load path %s from %s", path, root)
	}

	b, err := ioutil.ReadFile(p)
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
func StartConfig(cfg *config.Config) error {
	for _, base := range cfg.Imports {
		if len(base) == 0 {
			continue
		}
		root := ""
		if r, ok := cfg.Options[config.OptionCfgPath]; ok {
			root = r
		}
		baseCfg, err := loadCfg(root, base)
		if err != nil {
			return errors.Wrapf(err, "load config %s from %s", base, root)
		}
		override(baseCfg, cfg)
	}

	plans, err := create(cfg)
	if err != nil {
		return errors.Wrapf(err, "create test")
	}

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
	var cases []string
	for k, v := range results {
		str := "success"
		if v != nextFinished {
			str = "fail"
			failed = true
			cases = append(cases, k)
		}
		fmt.Printf("\t%s: %s\n", k, str)
	}

	if failed {
		return errors.Errorf("failed schedules: %v", cases)
	}
	return nil
}

// Start a test, path is the configure json file path, which must be able to be
// unmarshal to config.Config
func Start(path string) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return errors.Wrap(err, "read config file")
	}

	var cfg config.Config
	err = json.Unmarshal(b, &cfg)
	if err != nil {
		return errors.Wrap(err, "unmarshal json")
	}

	cfg.Options[config.OptionCfgPath], err = filepath.Abs(filepath.Dir(path))
	if err != nil {
		return errors.Wrapf(err, "get config path(%s)", path)
	}

	return StartConfig(&cfg)
}
