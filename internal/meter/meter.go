package meter

import (
	"errors"
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
}

func (bg *background) dup() *background {
	return &background{
		name:    bg.name,
		counter: bg.counter,
		local:   bg.local.dup(),
		global:  bg.global,
		lr:      bg.lr,
		err:     bg.err,
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
func (bg *background) report(err error) {
	if bg.err != nil {
		bg.err = err
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

func (bg *background) setInput(value string) {
	bg.local.put(KeyInput, value)
}
func (bg *background) setOutput(value string) {
	bg.local.put(KeyOutput, value)
}
func (bg *background) setError(value string) {
	bg.local.put(KeyError, value)
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
