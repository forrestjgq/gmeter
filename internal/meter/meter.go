package meter

import (
	"github.com/forrestjgq/gomark/gmi"
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
	seq           int
	local, global env
	lr            gmi.Marker
}

func (bg *background) report() {

}


type runnable interface {
	run(bg *background) next
}
