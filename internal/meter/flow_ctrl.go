package meter

import (
	"sync"
	"time"
)

type passport interface {
	cancel()
}
type flowControl struct {
	qps      int
	parallel int
	now      int
	recent   int
	t        time.Time
	mt       sync.Mutex
}

func (fc *flowControl) cancel() {
	fc.mt.Lock()
	fc.now--
	fc.mt.Unlock()
}

func (fc *flowControl) wait() passport {
	for {
		p := fc.try()
		if p == nil {
			time.Sleep(10 * time.Millisecond)
		} else {
			return p
		}
	}
}

func (fc *flowControl) try() passport {
	fc.mt.Lock()
	defer fc.mt.Unlock()

	if fc.parallel > 1 {
		if fc.now >= fc.parallel {
			return nil
		}
	}
	if fc.qps > 1 {
		now := time.Now()
		if now.Sub(fc.t) > time.Second {
			fc.recent = 0
			fc.t = now
		}
		if fc.recent >= fc.qps {
			return nil
		}
		fc.recent++
	}

	fc.now++
	//if fc.now%10 == 0 || fc.recent%10 == 0 {
	//	fmt.Printf("qps %d parallel %d, now %d recent %d\n", fc.qps, fc.parallel, fc.now, fc.recent)
	//}
	return fc
}

func makeFlowControl(qps, parallel int) *flowControl {
	return &flowControl{
		qps:      qps,
		parallel: parallel,
		now:      0,
		mt:       sync.Mutex{},
		t:        time.Now(),
	}
}
