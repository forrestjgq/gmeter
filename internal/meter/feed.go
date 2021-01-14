package meter

import (
	"math"
	"sync"
)

type category string

// content that does not take a key defined as explicit category will be
// considered as http header
type content map[category]string

const (
	catMethod category = "_mtd_"
	catURL    category = "_url_"
	catBody   category = "_bdy_"
)

// feeder provide all possible elements http request requires
type feeder interface {
	feed(bg *background) (content, error)
	close()
}

type baby struct {
	bg  *background
	wg  sync.WaitGroup
	c   content
	err error
}
type dynamicFeeder struct {
	source     map[string]segments
	c          chan *baby
	seq        uint64
	count      uint64
	end        bool
	preprocess *group
}

func (f *dynamicFeeder) close() {
	close(f.c)
}

func (f *dynamicFeeder) feed(bg *background) (content, error) {
	b := &baby{
		bg: bg,
	}

	b.wg.Add(1)
	f.c <- b
	b.wg.Wait()

	return b.c, b.err
}

func (f *dynamicFeeder) full() bool {
	if f.end {
		return true
	}
	if f.count == 0 {
		return false
	}
	return f.seq >= f.count
}
func (f *dynamicFeeder) run() {
	for b := range f.c {
		b.c = content{
			catMethod: "GET",
			catBody:   "",
			catURL:    "",
		}

		if f.full() {
			b.err = EofError
			b.wg.Done()
			continue
		}

		f.seq++

		if f.preprocess != nil {
			_, err := f.preprocess.compose(b.bg)
			if err != nil {
				b.err = err
				if isEof(err) {
					f.end = true
				}
				b.wg.Done()
				continue
			}
		}

		for k := range b.c {
			var err error
			var str string
			s, ok := f.source[string(k)]
			if !ok {
				continue
			}

			str, err = s.compose(b.bg)
			if err != nil {
				b.err = err
				if isEof(err) {
					f.end = true
				}
				break
			}

			b.c[k] = str
		}
		b.wg.Done()
	}
}

func makeDynamicFeeder(cfg map[string]string, count uint64, preprocess []string) (feeder, error) {
	f := &dynamicFeeder{
		source: make(map[string]segments),
		c:      make(chan *baby),
		count:  count,
	}

	iterable := false
	for k, v := range cfg {
		if s, err := makeSegments(v); err != nil {
			return nil, err
		} else {
			f.source[k] = s
			if s.iterable() {
				iterable = true
			}
		}
	}

	if len(preprocess) > 0 {
		g, err := makeGroup(preprocess, false)
		if err != nil {
			return nil, err
		}
		if g.iterable() {
			iterable = true
		}
		f.preprocess = g
	}

	if iterable {
		f.count = math.MaxUint64 - 1
	}

	go f.run()

	return f, nil
}
