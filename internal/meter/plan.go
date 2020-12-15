package meter

import (
	"github.com/golang/glog"
)

type plan struct {
	name       string
	target     runnable
	bg         *background
	concurrent int
}

func (p *plan) runOneByOne() next {
	for {
		p.bg.next()

		decision := p.target.run(p.bg)
		if decision != nextContinue {
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
		go func() {
			bg := p.bg.dup()
			for !stop {
				bg.next()
				if decision := p.target.run(bg); decision != nextContinue {
					// maybe error, may finished
					c <- decision
				}
			}

			c <- nextAbortPlan
		}()
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
	if p.concurrent > 1 {
		return p.runConcurrent(p.concurrent)
	}
	return p.runOneByOne()
}
