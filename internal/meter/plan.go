package meter

import (
	"github.com/golang/glog"
)


type plan struct {
	target runnable
	bg     background
}

func (p *plan) runOneByOne() next {
	for {
		decision := p.target.run(&p.bg)
		if decision == nextContinue {
			p.bg.seq++
		} else {
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
			for !stop {
				if decision := p.target.run(&p.bg); decision != nextContinue {
					// maybe error, may finished
					c <- decision
				}
				p.bg.seq++
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

