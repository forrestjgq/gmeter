package meter

import (
	"io"
	"sync"

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
	//KeyOutput   = "OUTPUT"
	KeyError = "ERROR"

	KeyFailure = "FAILURE"
	EOF        = "EOF"
)

var (
	EofError = io.EOF
)

func isEof(err error) bool {
	if err != nil {
		return errors.Cause(err).Error() == EOF
	}
	return false
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

func createDB() env {
	return &kvdb{
		m:   make(map[string]string),
		mtx: sync.Mutex{},
	}
}

// container to store environment
type env interface {
	get(key string) string
	put(key string, value string)
	delete(key string)
	has(key string) bool
	dup() env
}

type background struct {
	name              string // global test name
	db, local, global env
	dyn               []env
	lr                gmi.Marker
	err               error
	rpt               *reporter
	predefine         map[string]string
}

const (
	GMeterExit = "gmeter-exit"
)

// globalClose should only be called by root background
func (bg *background) globalClose() {
	if bg.rpt != nil {
		bg.rpt.close()
	}
}
func (bg *background) dup() *background {
	return &background{
		name:      bg.name,
		local:     bg.local.dup(),
		global:    bg.global,
		db:        bg.db,
		lr:        bg.lr,
		rpt:       bg.rpt,
		predefine: bg.predefine,
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
	e := bg.topEnv()
	if e == nil {
		panic("no json env " + key)
	}
	return e.get(key)
}

//func (bg *background) setJsonEnv(key, value string) {
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
	return bg.db.get(key)
}
func (bg *background) dbWrite(key string, value string) {
	bg.db.put(key, value)
}
func (bg *background) dbDelete(key string) {
	bg.db.delete(key)
}
func (bg *background) getLocalEnv(key string) string {
	if key == KeyError {
		if bg.err == nil {
			return ""
		}
		return bg.err.Error()
	}
	return bg.local.get(key)
}
func (bg *background) setLocalEnv(key string, value string) {
	bg.local.put(key, value)
}
func (bg *background) delLocalEnv(key string) {
	bg.local.delete(key)
}

func (bg *background) getGlobalEnv(key string) string {
	r := bg.global.get(key)
	if len(r) == 0 {
		r = GetGlobalVariable(key)
	}
	return r
}
func (bg *background) setGlobalEnv(key string, value string) {
	bg.global.put(key, value)
}

func (bg *background) inDebug() bool {
	return bg.getGlobalEnv(KeyDebug) == "true"
}

type runnable interface {
	run(bg *background) next
	close()
}
