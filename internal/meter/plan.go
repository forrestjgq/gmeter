package meter

import (
	"fmt"
	"strconv"
	"sync/atomic"

	"github.com/golang/glog"
)

type plan struct {
	name       string
	target     runnable
	bg         *background
	concurrent int
	preprocess *group
	seq        int64
}

func (p *plan) close() {
	if p.bg != nil {
		p.bg.globalClose()
	}
}
func (p *plan) runOneByOne() next {
	for {
		p.bg.next()
		p.bg.setLocalEnv("ROUTINE", "-1")
		seq := atomic.AddInt64(&p.seq, 1)
		p.bg.setLocalEnv(KeySequence, strconv.Itoa(int(seq)))

		decision := p.target.run(p.bg)
		if decision != nextContinue {
			if decision != nextFinished {
				fmt.Printf("plan %s failed, error: %s\n", p.name, p.bg.getError())
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
		go func(n int) {
			sn := strconv.Itoa(n)
			bg := p.bg.dup()
			for !stop {
				bg.next()
				bg.setLocalEnv("ROUTINE", sn)
				seq := atomic.AddInt64(&p.seq, 1)
				bg.setLocalEnv(KeySequence, strconv.Itoa(int(seq)))
				if decision := p.target.run(bg); decision != nextContinue {
					// maybe error, may finished
					c <- decision
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
			p.bg.setErrorf("preprocess fail, err: %v", err)
		}
		if p.bg.hasError() {
			return nextAbortPlan
		}
	}
	if p.concurrent > 1 {
		return p.runConcurrent(p.concurrent)
	}
	return p.runOneByOne()
}
