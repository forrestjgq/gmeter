package meter

import (
	"github.com/forrestjgq/gomark/gmi"
)

const (
	escape byte = '`' // embedded section
)

type next int

const (
	nextContinue next = iota
	nextRetry
	nextAbortPlan
	nextAbortAll
	nextFinished
)

const (
	KeyInput  = "INPUT"
	KeyOutput = "OUTPUT"
	KeyError  = "ERROR"

	EOF = "EOF"
)

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

func makeSimpEnv() env {
	return make(simpEnv)
}

// container to store environment
type env interface {
	get(key string) string
	put(key string, value string)
	delete(key string)
	has(key string) bool
}
type background struct {
	name          string
	seq           int64
	local, global env
	lr            gmi.Marker
	err           error
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

func (bg *background) getGlobalEnv(key string) string {
	return bg.global.get(key)
}
func (bg *background) setGlobalEnv(key string, value string) {
	bg.global.put(key, value)
}

type runnable interface {
	run(bg *background) next
}
