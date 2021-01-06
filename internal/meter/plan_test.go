package meter

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"
)

type testPlanSeqRunner struct {
	seq int
	n   next
}

func (t *testPlanSeqRunner) run(bg *background) next {
	t.seq++

	s := bg.getLocalEnv("HAHA")
	if len(s) > 0 {
		bg.setError(errors.New("HAHA should not be present"))
		return nextAbortPlan
	}

	bg.setLocalEnv("HAHA", "hoho")
	s = bg.getLocalEnv(KeyRoutine)
	if len(s) == 0 {
		bg.setError(errors.New("ROUTINE not present"))
		return nextAbortPlan
	}

	i, err := strconv.Atoi(s)
	if err != nil {
		bg.setError(err)
		return nextAbortPlan
	}
	if i >= 0 {
		bg.setError(errors.New("expect negtive routine id"))
		return nextAbortPlan
	}

	s = bg.getLocalEnv(KeySequence)
	if strconv.Itoa(t.seq) != s {
		bg.setError(fmt.Errorf("expect %d get %s", t.seq, s))
		return nextAbortPlan
	}

	return t.n
}

func TestPlanSequence(t *testing.T) {
	p := &plan{
		name:       "test-plan-sequence",
		target:     nil,
		bg:         nil,
		concurrent: 0,
		preprocess: nil,
		seq:        0,
	}

	var err error
	p.bg, err = createDefaultBackground()
	if err != nil {
		t.Fatalf(err.Error())
	}

	tpr := &testPlanSeqRunner{
		n: nextContinue,
	}
	p.target = tpr

	/*
		pp := []string {
			"`strlen $(WHAT) | assert $(OUTPUT) == 0`",
			"`envw -C what WHAT",
		}

		p.preprocess, err = makeGroup(pp, false)
		if err != nil {
			t.Fatalf(err.Error())
		}
	*/

	var ret = nextContinue
	go func() {
		ret = p.run()
	}()

	time.Sleep(1 * time.Second)
	if ret != nextContinue {
		t.Fatalf("expect running")
	}

	tpr.n = nextFinished
	time.Sleep(100 * time.Millisecond)
	if ret != nextFinished {
		t.Fatalf("expect finish")
	}
	p.close()
}
func TestPlanSequenceFail(t *testing.T) {
	p := &plan{
		name:       "test-plan-sequence-fail",
		target:     nil,
		bg:         nil,
		concurrent: 0,
		preprocess: nil,
		seq:        0,
	}

	var err error
	p.bg, err = createDefaultBackground()
	if err != nil {
		t.Fatalf(err.Error())
	}

	tpr := &testPlanSeqRunner{
		n: nextContinue,
	}
	p.target = tpr

	/*
		pp := []string {
			"`strlen $(WHAT) | assert $(OUTPUT) == 0`",
			"`envw -C what WHAT",
		}

		p.preprocess, err = makeGroup(pp, false)
		if err != nil {
			t.Fatalf(err.Error())
		}
	*/

	var ret = nextContinue
	go func() {
		ret = p.run()
	}()

	time.Sleep(1 * time.Second)
	if ret != nextContinue {
		t.Fatalf("expect running")
	}

	tpr.n = nextAbortPlan
	time.Sleep(100 * time.Millisecond)
	if ret != nextAbortPlan {
		t.Fatalf("expect finish")
	}

	p.close()
}

type routine struct {
	n next
}
type testPlanConcurrentRunner struct {
	concurrent int
	routines   map[string]*routine
	seq        map[string]int
	mutex      sync.Mutex
	n          next
}

func (t *testPlanConcurrentRunner) check() bool {
	t.mutex.Lock()
	defer t.mutex.Unlock()
	n := len(t.seq)
	for i := 0; i < n; i++ {
		if _, ok := t.seq[strconv.Itoa(i+1)]; !ok {
			return false
		}
	}
	return true
}
func (t *testPlanConcurrentRunner) mark(r string, n next) {
	t.mutex.Lock()
	if t.routines == nil {
		t.routines = make(map[string]*routine)
	}

	t.routines[r] = &routine{n: n}

	t.mutex.Unlock()
}
func (t *testPlanConcurrentRunner) run(bg *background) next {
	s := bg.getLocalEnv("PREDEFINE")
	if s != "soso" {
		bg.setError(errors.New("lost predefine"))
		return nextAbortPlan
	}

	s = bg.getLocalEnv("HAHA")
	if len(s) > 0 {
		bg.setError(errors.New("HAHA should not be present"))
		return nextAbortPlan
	}

	bg.setLocalEnv("HAHA", "hoho")

	s = bg.getLocalEnv(KeyRoutine)
	if len(s) == 0 {
		bg.setError(errors.New("ROUTINE not present"))
		return nextAbortPlan
	}

	i, err := strconv.Atoi(s)
	if err != nil {
		bg.setError(err)
		return nextAbortPlan
	}
	if i < 0 || i >= t.concurrent {
		bg.setError(errors.New("invalid routine " + s))
		return nextAbortPlan
	}

	t.mutex.Lock()
	defer t.mutex.Unlock()
	t.seq[bg.getLocalEnv(KeySequence)] = 1
	if r, ok := t.routines[bg.getLocalEnv(KeyRoutine)]; ok {
		return r.n
	}
	return t.n
}
func TestPlanConcurrent(t *testing.T) {
	concurrent := 100
	p := &plan{
		name:       "test-plan-concurrent",
		target:     nil,
		bg:         nil,
		concurrent: concurrent,
		preprocess: nil,
		seq:        0,
	}

	var err error
	p.bg, err = createDefaultBackground()
	if err != nil {
		t.Fatalf(err.Error())
	}
	p.bg.predefine = map[string]string{
		"PREDEFINE": "soso",
	}

	tpr := &testPlanConcurrentRunner{
		concurrent: concurrent,
		n:          nextContinue,
		seq:        map[string]int{},
	}
	p.target = tpr

	var pp []string

	p.preprocess, err = makeGroup(pp, false)
	if err != nil {
		t.Fatalf(err.Error())
	}

	var ret = nextContinue
	go func() {
		ret = p.run()
	}()

	time.Sleep(1 * time.Second)
	if ret != nextContinue {
		t.Fatalf("expect running, err: %v", p.bg.getError())
	}

	tpr.n = nextFinished
	time.Sleep(100 * time.Millisecond)
	if ret != nextFinished {
		t.Fatalf("expect finish")
	}

	if !tpr.check() {
		t.Fatalf("seq check fail")
	}
	p.close()
}
func TestPlanConcurrentFail(t *testing.T) {
	concurrent := 100
	p := &plan{
		name:       "test-plan-concurrent",
		target:     nil,
		bg:         nil,
		concurrent: concurrent,
		preprocess: nil,
		seq:        0,
	}

	var err error
	p.bg, err = createDefaultBackground()
	if err != nil {
		t.Fatalf(err.Error())
	}
	p.bg.predefine = map[string]string{
		"PREDEFINE": "soso",
	}

	tpr := &testPlanConcurrentRunner{
		concurrent: concurrent,
		n:          nextContinue,
		seq:        map[string]int{},
	}
	p.target = tpr

	var pp []string

	p.preprocess, err = makeGroup(pp, false)
	if err != nil {
		t.Fatalf(err.Error())
	}

	var ret = nextContinue
	go func() {
		ret = p.run()
	}()

	time.Sleep(1 * time.Second)
	if ret != nextContinue {
		t.Fatalf("expect running, err: %v", p.bg.getError())
	}

	tpr.mark("1", nextAbortPlan)
	time.Sleep(100 * time.Millisecond)
	if ret != nextAbortPlan {
		t.Fatalf("expect abort")
	}

	if !tpr.check() {
		t.Fatalf("seq check fail")
	}
	p.close()
}
func TestPlanInvalidPreProcess(t *testing.T) {
	concurrent := 100
	p := &plan{
		name:       "test-plan-concurrent",
		target:     nil,
		bg:         nil,
		concurrent: concurrent,
		preprocess: nil,
		seq:        0,
	}

	var err error
	p.bg, err = createDefaultBackground()
	if err != nil {
		t.Fatalf(err.Error())
	}
	p.bg.predefine = map[string]string{
		"PREDEFINE": "soso",
	}

	tpr := &testPlanConcurrentRunner{
		concurrent: concurrent,
		n:          nextContinue,
		seq:        map[string]int{},
	}
	p.target = tpr

	pp := []string{
		"`assert 0 != 0`",
	}

	p.preprocess, err = makeGroup(pp, false)
	if err != nil {
		t.Fatalf(err.Error())
	}

	ret := p.run()
	if ret != nextAbortPlan {
		t.Fatalf("expect abort")
	}
	p.close()
}
