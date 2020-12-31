package meter

import (
	"errors"
	"fmt"
	"sync/atomic"

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

	// Temp
	KeyTemp = "TEMP"
)

var (
	EofError error = errors.New(EOF)
)

func isEof(err error) bool {
	return err.Error() == EOF
}

type simpEnv map[string]string

func (s simpEnv) pop(bg *background) {
	panic("implement me")
}

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

// container to store environment
type env interface {
	get(key string) string
	put(key string, value string)
	delete(key string)
	has(key string) bool
	dup() env
}
type counter struct {
	seq uint64
}

func (c *counter) next() uint64 {
	return atomic.AddUint64(&c.seq, 1)
}

type background struct {
	name          string // global test name
	counter       *counter
	seq           uint64
	local, global env
	dyn           []env
	lr            gmi.Marker
	err           error
	rpt           *reporter
	predefine     map[string]string
}

const (
	GMeterExit = "gmeter-exit"
)

// globalClose should only be called by root background
func (bg *background) globalClose() {
	bg.rpt.close()
}
func (bg *background) dup() *background {
	return &background{
		name:    bg.name,
		counter: bg.counter,
		local:   bg.local.dup(),
		global:  bg.global,
		lr:      bg.lr,
		rpt:     bg.rpt,
	}
}
func (bg *background) next() {
	bg.cleanup()
	bg.seq = bg.counter.next()
}
func (bg *background) cleanup() {
	bg.local = make(simpEnv)
	if bg.predefine != nil {
		for k, v := range bg.predefine {
			bg.setLocalEnv(k, v)
		}
	}
}
func (bg *background) reportDefault(newline bool) {
	bg.rpt.reportDefault(bg, newline)
}
func (bg *background) report(content string, newline bool) {
	bg.rpt.report(content, newline)
}
func (bg *background) reportTemplate(template string, newline bool) {
	bg.rpt.reportTemplate(bg, template, newline)
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
	e := bg.topEnv()
	if e == nil {
		panic("no json env " + key)
	}
	return e.get(key)
}
func (bg *background) setJsonEnv(key, value string) {
	e := bg.topEnv()
	if e == nil {
		panic("no json env " + key)
	}
	e.put(key, value)
}
func (bg *background) deleteJsonEnv(key string) {
	e := bg.topEnv()
	if e == nil {
		panic("no json env")
	}
	e.delete(key)
}
func (bg *background) getTemp() string {
	return bg.local.get(KeyTemp)
}
func (bg *background) getInput() string {
	return bg.local.get(KeyInput)
}
func (bg *background) getOutput() string {
	return bg.local.get(KeyOutput)
}
func (bg *background) getError() string {
	return bg.local.get(KeyError)
}
func (bg *background) hasError() bool {
	return len(bg.local.get(KeyError)) > 0
}

func (bg *background) setTemp(value string) {
	bg.local.put(KeyTemp, value)
}
func (bg *background) setInput(value string) {
	bg.local.put(KeyInput, value)
}
func (bg *background) setOutput(value string) {
	bg.local.put(KeyOutput, value)
}
func (bg *background) setError(value string) {
	bg.local.put(KeyError, value)
}
func (bg *background) setErrorf(format string, a ...interface{}) {
	err := fmt.Errorf(format, a...)
	bg.setError(err.Error())
}

func (bg *background) getLocalEnv(key string) string {
	return bg.local.get(key)
}
func (bg *background) setLocalEnv(key string, value string) {
	bg.local.put(key, value)
}
func (bg *background) delLocalEnv(key string) {
	bg.local.delete(key)
}

func (bg *background) getGlobalEnv(key string) string {
	return bg.global.get(key)
}
func (bg *background) setGlobalEnv(key string, value string) {
	bg.global.put(key, value)
}

type runnable interface {
	run(bg *background) next
}
