package meter

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"

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
	KeySchedule = "SCHEDULE"
	KeyTPath    = "TPATH"

	// Local
	KeyTest     = "TEST"
	KeyURL      = "URL"
	KeyRequest  = "REQUEST"
	KeyStatus   = "STATUS"
	KeyResponse = "RESPONSE"
	KeyInput    = "INPUT"
	KeyOutput   = "OUTPUT"
	KeyError    = "ERROR"

	EOF = "EOF"
)

var (
	EofError error = errors.New(EOF)
)

func isEof(err error) bool {
	return err.Error() == EOF
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
	lr            gmi.Marker
	err           error
	rptc          chan string
	rptf          io.WriteCloser
	rptFormater   segments
	rptRuns       bool
}

const (
	GMeterExit = "gmeter-exit"
)

// globalClose should only be called by root background
func (bg *background) globalClose() {
	if bg.requireReport() {
		bg.report(GMeterExit)
		for bg.rptRuns {
			time.Sleep(100 * time.Millisecond)
		}
		close(bg.rptc)
	}
}
func (bg *background) dup() *background {
	return &background{
		name:        bg.name,
		counter:     bg.counter,
		local:       bg.local.dup(),
		global:      bg.global,
		lr:          bg.lr,
		rptc:        bg.rptc,
		rptf:        nil,
		rptFormater: bg.rptFormater,
		rptRuns:     false,
	}
}
func (bg *background) next() {
	bg.cleanup()
	bg.seq = bg.counter.next()
}
func (bg *background) cleanup() {
	bg.setInput("")
	bg.setOutput("")
	bg.setError("")
}
func (bg *background) createReport(path, format string) error {
	if len(path) > 0 {
		f, err := os.Create(path)
		if err != nil {
			return err
		}
		fmt.Printf("report will be written to %s\n", path)
		bg.rptf = f
		bg.rptc = make(chan string, 1000)
		if len(format) > 0 {
			bg.rptFormater, err = makeSegments(format)
			if err != nil {
				return err
			}
		}
		go bg.waitReport()
	}
	return nil
}
func (bg *background) waitReport() {
	bg.rptRuns = true
	for c := range bg.rptc {
		if c == GMeterExit {
			break
		}
		if bg.rptf != nil {
			_, _ = bg.rptf.Write([]byte(c))
		}
	}
	if bg.rptf != nil {
		_ = bg.rptf.Close()
	}
	bg.rptRuns = false
}
func (bg *background) requireReport() bool {
	return bg.rptRuns && bg.rptc != nil
}
func (bg *background) reportDefault() {
	if bg.requireReport() {
		if bg.rptFormater != nil {
			str, err := bg.rptFormater.compose(bg)
			if err != nil {
				bg.setError(err.Error())
			} else {
				bg.report(str)
			}
		}
	}
}
func (bg *background) report(content string) {
	if bg.requireReport() {
		bg.rptc <- content
	}
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
	bg.local.put(KeyError, err.Error())
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
