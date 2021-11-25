package meter

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/huandu/go-clone"

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
func createHTTPClient(h *config.Host, timeout string) (*http.Client, error) {
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
func loadHTTPClient(t *config.Test, s *config.Schedule, cfg *config.Config) (*http.Client, string, error) {
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

	h, ok := cfg.Hosts[t.Host]
	if !ok {
		urls := strings.Split(t.Host, "|")
		if len(urls) == 0 || len(urls) > 2 {
			return nil, "", errors.Errorf("unknown host definition: %s", t.Host)
		}
		h = &config.Host{}

		if len(urls) == 1 {
			h.Host = urls[0]
		} else {
			h.Proxy = urls[0]
			h.Host = urls[1]
		}
	}
	if err := h.Check(); err != nil {
		return nil, "", errors.Wrapf(err, "host %s check", t.Host)
	}

	if len(t.Timeout) == 0 {
		if du, ok := s.Env["TIMEOUT"]; ok {
			t.Timeout = du
		}
	}
	if len(t.Timeout) == 0 {
		t.Timeout = "1m"
	}

	c, err := createHTTPClient(h, t.Timeout)
	if err != nil {
		return nil, "", err
	}
	return c, h.Host, nil
}

// create a test from a base.
func constructTest(t, base *config.Test) (*config.Test, error) {

	t = clone.Clone(t).(*config.Test)

	if len(t.Host) == 0 && len(base.Host) > 0 {
		t.Host = base.Host
	}

	// request not defined, use base's, and make RequestMessage preferred.
	if len(t.Request) == 0 && t.RequestMessage == nil {
		if base.RequestMessage != nil {
			t.RequestMessage = base.RequestMessage
		} else if len(base.Request) > 0 {
			t.Request = base.Request
		}
	}

	var err error
	t.PreProcess, err = merge(base.PreProcess, t.PreProcess)
	if err != nil {
		return nil, errors.Wrapf(err, "merge PreProcess")
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
		src.Success, err = merge(dst.Success, src.Success)
		if err != nil {
			return nil, errors.Wrapf(err, "merge Success")
		}
		src.Check, err = merge(dst.Check, src.Check)
		if err != nil {
			return nil, errors.Wrapf(err, "merge Check")
		}
		src.Failure, err = merge(dst.Failure, src.Failure)
		if err != nil {
			return nil, errors.Wrapf(err, "merge Failure")
		}
	}
	return t, nil
}

func loadFunctions(cfg *config.Config) (map[string]composable, error) {
	functions := map[string]composable{}
	if cfg.Functions != nil {
		for k, v := range cfg.Functions {
			c, _, err := makeComposable(v)
			if err != nil {
				return nil, errors.Wrapf(err, "make function %s", k)
			}
			functions[k] = c
		}
	}

	return functions, nil
}
func loadRequest(t *config.Test, cfg *config.Config) (*config.Request, error) {
	ok := false
	req := t.RequestMessage
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
		return nil, errors.Errorf("misses request")
	}

	if req.Method == "" {
		req.Method = "GET"
	}

	if err := req.Check(); err != nil {
		return nil, errors.Wrap(err, "request check")
	}
	return req, nil
}
func loadProvider(host string, t *config.Test, s *config.Schedule, cfg *config.Config) (providerSource, error) {
	req, err := loadRequest(t, cfg)
	if err != nil {
		return nil, errors.Wrap(err, "load request")
	}

	m := make(map[string]string)
	m[string(catMethod)] = req.Method
	m[string(catURL)] = host + req.Path
	m[string(catBody)] = string(req.Body)
	for k, v := range req.Headers {
		m[k] = v
	}

	feeder, err := makeDynamicFeeder(m, s.Count, t.PreProcess)
	if err != nil {
		return nil, errors.Wrap(err, "create feeder")
	}

	return makeFeedProvider(feeder)
}
func loadConsumer(t *config.Test, cfg *config.Config) (consumer, error) {

	var csm consumer
	decision := ignoreOnFail
	if cfg.Options[config.OptionAbortIfFail] == "true" {
		decision = abortOnFail
	}

	rsp := t.Response
	if rsp != nil {
		var err error
		csm, err = makeDynamicConsumer(rsp.Check, rsp.Success, rsp.Failure, rsp.Template, decision)
		if err != nil {
			return nil, errors.Wrapf(err, "make consumer")
		}
	}
	return csm, nil
}
func loadPlan(cfg *config.Config, s *config.Schedule) (*plan, error) {
	var err error

	tests := strings.Split(s.Tests, "|")
	star := -1
	testMap := make(map[string]struct{})
	var filtered  []string
	var void  struct {}
	for _, t := range tests {
		t = strings.TrimSpace(t)
		if len(t) == 0 {
			continue
		}
		if t == "*" {
			star = len(filtered)
		}
		testMap[t] = void
		filtered = append(filtered, t)
	}
	if star >= 0 {
		var rest  []string
		for k := range cfg.Tests {
			if _, exist := testMap[k]; exist{
				// ignore included test
				continue
			}
			rest = append(rest, k)
		}

		var tmp []string
		if star > 0 {
			tmp = append(tmp, filtered[:star]...)
		}
		if len(rest) > 0 {
			tmp = append(tmp, rest...)
		}
		if star < len(filtered)-1 {
			tmp = append(tmp, filtered[star+1:]...)
		}
		tests = tmp
	} else {
		tests = filtered
	}

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
			t, err = constructTest(t, baseTest)
			if err != nil {
				return nil, errors.Wrapf(err, "schedule %s construct test %s", s.Name, name)
			}
		}

		client, host, err := loadHTTPClient(t, s, cfg)
		if err != nil {
			return nil, errors.Wrapf(err, "config %s schedule %s test %s load host", cfg.Name, s.Name, name)
		}

		prv, err := loadProvider(host, t, s, cfg)
		if err != nil {
			return nil, errors.Wrapf(err, "config %s schedule %s test %s load provider", cfg.Name, s.Name, name)
		}

		csm, err := loadConsumer(t, cfg)
		if err != nil {
			return nil, errors.Wrapf(err, "config %s schedule %s test %s load consumer", cfg.Name, s.Name, name)
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
		bg:         nil,
		concurrent: s.Concurrency,
	}

	p.preprocess, _, err = makeComposable(s.PreProcess)
	if err != nil {
		return nil, errors.Wrapf(err, "schedule %s make PreProcess", s.Name)
	}
	p.postprocess, _, err = makeComposable(s.PostProcess)
	if err != nil {
		return nil, errors.Wrapf(err, "schedule %s make PostProcess", s.Name)
	}
	return p, nil
}
func create(cfg *config.Config) ([]*plan, error) {
	if len(cfg.Schedules) == 0 {
		return nil, errors.Errorf("no schedule is defined")
	}
	var plans []*plan

	// functions that could be used
	functions, err := loadFunctions(cfg)
	if err != nil {
		return nil, errors.Wrapf(err, "config %s load functions", cfg.Name)
	}

	for _, s := range cfg.Schedules {
		if s.Count == 0 {
			s.Count = math.MaxUint64 - 1
		}

		if s.Concurrency > 1 {
			if s.Parallel > s.Concurrency {
				s.Parallel = s.Concurrency
			} else if s.Parallel < 2 {
				s.Parallel = 0
			}
		} else {
			s.Concurrency = 1
			s.Parallel = 0
		}

		p, err := loadPlan(cfg, s)
		if err != nil {
			return nil, errors.Wrapf(err, "config %s schedule %s load plan", cfg.Name, s.Name)
		}

		p.bg, err = makeBackground(cfg, s)
		if err != nil {
			return nil, errors.Wrapf(err, "schedule %s create background ", s.Name)
		}

		p.bg.functions = functions
		if s.QPS > 1 || s.Parallel > 1 {
			p.fc = makeFlowControl(s.QPS, s.Parallel)
			p.bg.fc = p.fc
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

	for k, v := range template.Functions {
		if cfg.Functions != nil {
			if _, ok := cfg.Functions[k]; ok {
				// already defined, skip
				continue
			}

		} else {
			cfg.Functions = make(map[string]interface{})
		}
		cfg.Functions[k] = v
	}
}
func StartConfig(cfg *config.Config) error {
	imports, err := iface2strings(cfg.Imports)
	if err != nil {
		return errors.Wrapf(err, "convert imports to strings")
	}
	for _, base := range imports {
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
