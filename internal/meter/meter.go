package meter

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/forrestjgq/glog"

	"github.com/forrestjgq/gmeter/config"
	"github.com/forrestjgq/gomark"

	"github.com/pkg/errors"

	"github.com/forrestjgq/gomark/gmi"
)

type next int

const (
	nextContinue next = iota
	nextAbortPlan
	nextAbortAll
	nextFinished
)

const (
	// Global
	KeyDebug    = "DEBUG"
	KeyConfig   = "CONFIG"
	KeySchedule = "SCHEDULE"
	KeyTPath    = "TPATH"
	KeyCWD      = "CWD"

	// Local
	KeyTest     = "TEST"
	KeyRoutine  = "ROUTINE"
	KeySequence = "SEQUENCE"
	KeyURL      = "URL"
	KeyRequest  = "REQUEST"
	KeyStatus   = "STATUS"
	KeyResponse = "RESPONSE"
	KeyInput    = "INPUT"
	KeyOutput   = "OUTPUT"
	KeyError    = "ERROR"

	KeyFailure = "FAILURE"
	EOF        = "EOF"
)

var (
	EofError = io.EOF
)

func hasSign(s string) bool {
	if len(s) > 1 && (s[0] == '#' || s[0] == '?') {
		return true
	}
	return false
}
func getSign(s string) (rune, string) {
	if len(s) > 1 {
		switch s[0] {
		case '#', '?':
			return rune(s[0]), s[1:]
		}
	}
	return 0, s
}
func calcSign(sign rune, s string) string {
	switch sign {
	case '#':
		return strconv.Itoa(len(s))
	case '?':
		if len(s) > 0 {
			return _true
		}
		return _false
	default:
		return s
	}
}

type simpEnv map[string]string

func (s simpEnv) get(key string) string {
	return s[key]
}

func (s simpEnv) put(key string, value string) {
	s[key] = value
}

func (s simpEnv) delete(key string) {
	delete(s, key)
}

func (s simpEnv) has(key string) bool {
	_, ok := s[key]
	return ok
}
func (s simpEnv) dup() env {
	ret := make(simpEnv)
	for k, v := range s {
		ret[k] = v
	}
	return ret
}

func makeSimpEnv() env {
	return make(simpEnv)
}

type kvdb struct {
	m   map[string]string
	mtx sync.Mutex
}

func (db *kvdb) get(key string) string {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	return db.m[key]
}

func (db *kvdb) put(key string, value string) {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	db.m[key] = value
}

func (db *kvdb) delete(key string) {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	delete(db.m, key)
}

func (db *kvdb) has(key string) bool {
	db.mtx.Lock()
	defer db.mtx.Unlock()
	_, ok := db.m[key]
	return ok
}
func (db *kvdb) dup() env {
	return db
}

var globaldb env

func createDB() env {
	if globaldb == nil {
		globaldb = &kvdb{
			m:   make(map[string]string),
			mtx: sync.Mutex{},
		}
	}
	return globaldb
}

// container to store environment
type env interface {
	get(key string) string
	put(key string, value string)
	delete(key string)
	has(key string) bool
	dup() env
}

type arguments []composable

func (a arguments) call(bg *background) (string, error) {
	// Before call a function, the arguments must be composed first, and the
	// result as strings will be pushing into background as argument of function
	// call.
	var args []string
	for i, arg := range a {
		s, err := arg.compose(bg)
		if err != nil {
			return "", errors.Wrapf(err, "arg %d compose", i)
		}
		args = append(args, s)
	}
	if len(args) == 0 {
		return "", errors.New("arguments without function name")
	}
	bg.fargs = append(bg.fargs, args)
	defer func() {
		bg.fargs = bg.fargs[:len(bg.fargs)-1]
	}()

	function := args[0]
	if f, exist := bg.functions[function]; exist {
		s, err := f.compose(bg)
		if err != nil {
			return "", errors.Wrapf(err, "call function %s", function)
		}
		return s, nil
	}

	return "", errors.Errorf("function %s not found", function)
}

func makeArguments(args []string) (arguments, error) {
	var a arguments
	for _, arg := range args {
		c, err := makeSegments(arg)
		if err != nil {
			return nil, errors.Wrapf(err, "compose argument %s", arg)
		}
		a = append(a, c)
	}
	return a, nil
}

type perf struct {
	lr           gmi.Marker
	adder        gmi.Marker
	cLatency     chan int32
	maxLatency   int32
	minLatency   int32
	totalLatency int64
	start        time.Time
	count        int64
}

func (p *perf) close() {
	if p.cLatency != nil {
		close(p.cLatency)
		p.cLatency = nil
	}
}

func (p *perf) report(latency int32) {
	if p.cLatency != nil {
		p.cLatency <- latency
	}
}

func (p *perf) commit() (max, min, avg int32, qps int64) {
	if p.count == 0 {
		return
	}
	max = p.maxLatency
	min = p.minLatency
	avg = int32(p.totalLatency / p.count)
	du := time.Since(p.start).Milliseconds()
	//fmt.Printf("count %d du %d ms", p.count, du)

	if du > 0 {
		qps = p.count * 1000 / du
	}
	return
}

func makePerf(name string) *perf {
	p := &perf{
		lr:       gomark.NewLatencyRecorder(name),
		adder:    gomark.NewAdder(name),
		cLatency: make(chan int32, 1000),
	}
	go func() {
		for lr := range p.cLatency {
			if p.count == 0 {
				p.start = time.Now()
			}
			p.count++
			p.totalLatency += int64(lr)
			if p.maxLatency == 0 || p.maxLatency < lr {
				p.maxLatency = lr
			}
			if p.minLatency == 0 || p.minLatency > lr {
				p.minLatency = lr
			}
		}
	}()
	return p
}

type background struct {
	name              string // global test name
	db, local, global env
	dyn               []env
	err               error
	rpt               *reporter
	predefine         map[string]string
	fc                *flowControl
	fargs             [][]string // arguments stacks
	functions         map[string]composable
	perf              *perf
}

func makeBackground(cfg *config.Config, sched *config.Schedule) (*background, error) {
	bg := &background{
		name:   "default",
		db:     createDB(),
		local:  makeSimpEnv(),
		global: makeSimpEnv(),
	}

	str, err := os.Getwd()
	if err == nil {
		bg.setGlobalEnv(KeyCWD, str)
	}

	if cfg != nil {
		bg.name = cfg.Name
		bg.setGlobalEnv(KeyTPath, cfg.Options[config.OptionCfgPath])

		if debug, ok := cfg.Options[config.OptionDebug]; ok {
			bg.setGlobalEnv(KeyDebug, debug)
		}

		for k, v := range cfg.Env {
			if k != "" {
				bg.setGlobalEnv(k, v)
			}
		}
	} else {
		path, err := filepath.Abs(filepath.Dir("."))
		if err != nil {
			return nil, err
		}

		bg.setGlobalEnv(KeyTPath, path)
	}

	bg.setGlobalEnv(KeyConfig, bg.name)

	if sched != nil {
		bg.perf = makePerf(sched.Name)
		bg.setGlobalEnv(KeySchedule, sched.Name)
		if sched.Env != nil {
			bg.predefineLocalEnv(sched.Env)
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

			root := ""
			if cfg != nil {
				root = cfg.Options[config.OptionCfgPath]
			}
			sched.Reporter.Path, err = loadFilePath(root, sched.Reporter.Path)
			if err != nil {
				return nil, err
			}
		}

		bg.rpt, err = makeReporter(&sched.Reporter)
		if err != nil {
			return nil, err
		}
	} else {
		bg.setGlobalEnv(KeySchedule, "default-schedule")
	}

	return bg, nil
}

// globalClose should only be called by root background
func (bg *background) globalClose() {
	if bg.rpt != nil {
		bg.rpt.close()
	}
	if bg.perf != nil {
		bg.perf.close()
	}
}
func (bg *background) dup() *background {
	return &background{
		name:      bg.name,
		local:     bg.local.dup(),
		global:    bg.global,
		db:        bg.db,
		perf:      bg.perf,
		rpt:       bg.rpt,
		predefine: bg.predefine,
		fc:        bg.fc,
		functions: bg.functions,
	}
}
func (bg *background) next() {
	bg.cleanup()
}
func (bg *background) cleanup() {
	bg.local = make(simpEnv)
	if bg.predefine != nil {
		for k, v := range bg.predefine {
			bg.setLocalEnv(k, v)
		}
	}
	bg.err = nil
}
func (bg *background) reportLatency(latency int32) {
	if bg.perf != nil {
		bg.perf.report(latency)
	}
}
func (bg *background) reportDefault(newline bool) {
	if bg.rpt != nil {
		bg.rpt.reportDefault(bg, newline)
	}
}
func (bg *background) report(content string, newline bool) {
	if bg.rpt != nil {
		bg.rpt.report(content, newline)
	}
}
func (bg *background) reportTemplate(template string, newline bool) {
	if bg.rpt != nil {
		bg.rpt.reportTemplate(bg, template, newline)
	}
}
func (bg *background) predefineLocalEnv(m map[string]string) {
	bg.predefine = m
}
func (bg *background) pushJsonEnv(e env) {
	bg.dyn = append(bg.dyn, e)
}
func (bg *background) popJsonEnv() {
	if len(bg.dyn) == 0 {
		return
	}
	bg.dyn = bg.dyn[:len(bg.dyn)-1]
}
func (bg *background) topEnv() env {
	if len(bg.dyn) == 0 {
		return nil
	}
	return bg.dyn[len(bg.dyn)-1]
}
func (bg *background) getJsonEnv(key string) string {
	sign, k := getSign(key)
	e := bg.topEnv()
	if e == nil {
		panic("no json env " + k)
	}
	return calcSign(sign, e.get(k))
}

//func (bg *background) setJsonEnv(key, value string) {
//if hasSign(key) {
//glog.Fatalf("set local key %s value %s", key, value)
//}
//	e := bg.topEnv()
//	if e == nil {
//		panic("no json env " + key)
//	}
//	e.put(key, value)
//}
//func (bg *background) deleteJsonEnv(key string) {
//	e := bg.topEnv()
//	if e == nil {
//		panic("no json env")
//	}
//	e.delete(key)
//}
//func (bg *background) getInput() string {
//	return bg.local.get(KeyInput)
//}
//func (bg *background) getOutput() string {
//	return bg.local.get(KeyOutput)
//}

//func (bg *background) getErrorString() string {
//	if bg.err != nil {
//		return bg.err.Error()
//	}
//	return ""
//}
func (bg *background) getError() error {
	return bg.err
}
func (bg *background) hasError() bool {
	return bg.err != nil
}

func (bg *background) setInput(value string) {
	bg.local.put(KeyInput, value)
}

//func (bg *background) setOutput(value string) {
//	bg.local.put(KeyOutput, value)
//}
func (bg *background) setError(err error) {
	bg.err = err
}

func (bg *background) dbRead(key string) string {
	sign, k := getSign(key)
	return calcSign(sign, bg.db.get(k))
}
func (bg *background) dbWrite(key string, value string) {
	if hasSign(key) {
		glog.Fatalf("set DB key %s value %s", key, value)
	}
	bg.db.put(key, value)
}
func (bg *background) dbDelete(key string) {
	bg.db.delete(key)
}
func (bg *background) getArgument(index int) (string, error) {
	length := len(bg.fargs)
	if length == 0 {
		return "", errors.Errorf("not in function call")
	}

	args := bg.fargs[length-1]
	if index >= len(args) {
		return "", errors.Errorf("arg %d exceed range %d", index, len(args))
	}
	return args[index], nil
}
func (bg *background) getLocalEnv(key string) string {
	v := ""
	sign, k := getSign(key)
	if k == KeyError {
		if bg.err != nil {
			v = bg.err.Error()
		}
	} else {
		v = bg.local.get(k)
	}
	return calcSign(sign, v)
}
func (bg *background) setLocalEnv(key string, value string) {
	if hasSign(key) {
		glog.Fatalf("set local key %s value %s", key, value)
	}
	bg.local.put(key, value)
}
func (bg *background) delLocalEnv(key string) {
	bg.local.delete(key)
}

func (bg *background) getGlobalEnv(key string) string {
	sign, k := getSign(key)
	r := bg.global.get(k)
	if len(r) == 0 {
		r = GetGlobalVariable(k)
	}
	return calcSign(sign, r)
}
func (bg *background) setGlobalEnv(key string, value string) {
	if hasSign(key) {
		glog.Fatalf("set global key %s value %s", key, value)
	}
	bg.global.put(key, value)
}

func (bg *background) inDebug() bool {
	return bg.getGlobalEnv(KeyDebug) == "true"
}

func (bg *background) commit() {
	if bg.perf != nil {
		max, min, avg, qps := bg.perf.commit()
		bg.setLocalEnv("_.latency.max", strconv.Itoa(int(max)))
		bg.setLocalEnv("_.latency.min", strconv.Itoa(int(min)))
		bg.setLocalEnv("_.latency.avg", strconv.Itoa(int(avg)))
		bg.setLocalEnv("_.qps", strconv.Itoa(int(qps)))
	}
}

type runnable interface {
	run(bg *background) next
	close()
}
