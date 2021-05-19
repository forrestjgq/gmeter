package meter

import (
	"fmt"
	"strconv"
	"sync/atomic"

	"github.com/pkg/errors"

	"github.com/forrestjgq/glog"
)

type plan struct {
	name        string
	target      runnable
	bg          *background
	concurrent  int
	preprocess  composable
	postprocess composable
	seq         int64
	fc          *flowControl
}

func (p *plan) close() {
	if p.bg != nil {
		p.bg.globalClose()
	}
	p.target.close()
}
func (p *plan) runOneByOne() next {
	for {
		p.bg.next()
		p.bg.setLocalEnv(KeyRoutine, "-1")
		seq := atomic.AddInt64(&p.seq, 1)
		p.bg.setLocalEnv(KeySequence, strconv.Itoa(int(seq)))

		decision := p.target.run(p.bg)
		if decision != nextContinue {
			if decision != nextFinished {
				if p.bg.inDebug() {
					fmt.Printf("plan %s failed, error: %+v\n", p.name, p.bg.getError())
				} else {
					fmt.Printf("plan %s failed, error: %v\n", p.name, p.bg.getError())
				}
				fmt.Printf("HTTP request: \n")
				fmt.Printf("\tURL: %s\n\tBody: %s\n",
					p.bg.getLocalEnv(KeyURL), p.bg.getLocalEnv(KeyRequest))
				fmt.Printf("HTTP Response: \n")
				fmt.Printf("\tStatus: %s\n\tBody: %s\n",
					p.bg.getLocalEnv(KeyStatus), p.bg.getLocalEnv(KeyResponse))
			}
			return decision
		}
	}
}

func (p *plan) runConcurrent(n int) next {
	if n <= 1 {
		glog.Errorf("concurrent number is %d, we require it at least 2", n)
		return nextAbortAll
	}
	stop := false
	c := make(chan next)
	for i := 0; i < n; i++ {
		go func(idx int) {
			sn := strconv.Itoa(idx)
			bg := p.bg.dup()
			for !stop {
				bg.next()
				bg.setLocalEnv(KeyRoutine, sn)
				seq := atomic.AddInt64(&p.seq, 1)
				bg.setLocalEnv(KeySequence, strconv.Itoa(int(seq)))
				if decision := p.target.run(bg); decision != nextContinue {
					// maybe error, may finished
					if decision != nextFinished {
						glog.Errorf("routine %d exit with err %v", idx, bg.getError())
					}
					c <- decision
					return
				}
			}

			c <- nextAbortPlan
		}(i)
	}

	waiting := n
	result := nextFinished

	for d := range c {
		if d != nextFinished {
			stop = true
			if result == nextFinished {
				result = d
			}
		}

		waiting--
		if waiting == 0 {
			break
		}
	}

	return result
}
func (p *plan) run() next {
	if p.preprocess != nil {
		_, err := p.preprocess.compose(p.bg)
		if err != nil {
			p.bg.setError(errors.Wrapf(err, "plan %s preprocess", p.name))
		}
		if p.bg.hasError() {
			return nextAbortPlan
		}
	}
	defer func() {
		p.bg.commit()
		if p.postprocess != nil {
			_, _ = p.postprocess.compose(p.bg)
		}
	}()
	if p.concurrent > 1 {
		return p.runConcurrent(p.concurrent)
	}
	return p.runOneByOne()
}
