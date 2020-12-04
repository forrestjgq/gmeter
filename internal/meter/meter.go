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

type variable string

// container to store environment
type env interface {
	get(key string) variable
	put(key string, value variable)
	delete(key string)
	has(key string)
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

type runnable interface {
	run(bg *background) next
}
