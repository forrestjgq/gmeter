package meter

import (
	"math"
	"sync"
)

type category string

const (
	cmdExit = "exit"
)

// content that does not take a key defined as explicit category will be
// considered as http header
type content map[category]string

const (
	catMethod category = "_mtd_"
	catURL    category = "_url_"
	catBody   category = "_bdy_"
	catCmd    category = "_cmd_"
)

// feeder provide all possible elements http request requires
type feeder interface {
	feed(bg *background) (content, error)
}

// stringFeeder provides a string
type stringFeeder func(bg *background) (string, error)

// mapFeeder provides a map, usually it's header feeder
type mapFeeder func(bg *background) (map[string]string, error)

// feedCombiner is a feeder that combines several sub-feeders to provide http elements.
type feedCombiner struct {
	headers           mapFeeder
	url, method, body stringFeeder
}

// feed implements feeder
func (fc *feedCombiner) feed(bg *background) (content, error) {
	m := make(content)
	var err error

	if fc.url != nil {
		m[catURL], err = fc.url(bg)
		if err != nil {
			return nil, err
		}
	}

	if fc.method != nil {
		m[catMethod], err = fc.method(bg)
		if err != nil {
			return nil, err
		}
	}

	if fc.body != nil {
		m[catBody], err = fc.body(bg)
		if err != nil {
			return nil, err
		}
	}

	if fc.headers != nil {
		h, err := fc.headers(bg)
		if err != nil {
			return nil, err
		}
		for k, v := range h {
			m[category(k)] = v
		}
	}

	return m, nil
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
	iterable   bool
	preprocess []segments
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

		if len(f.preprocess) > 0 {
			toNext := false
			for _, segs := range f.preprocess {
				_, err := segs.compose(b.bg)
				if err != nil {
					b.err = err
					b.wg.Done()
					toNext = true
					break
				}
			}
			if toNext {
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

			if f.iterable {
				// for iterable segments, error is not tolerated, include eof or other error
				if err != nil {
					// make it full
					f.seq++
				}
			}

			if err != nil {
				b.err = err
				break
			}

			b.c[k] = str
		}
		f.seq++
		b.wg.Done()
	}
}

func makeDynamicFeeder(cfg map[string]string, count uint64, preprocess []string) (*dynamicFeeder, error) {
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
		for _, str := range preprocess {
			segs, err := makeSegments(str)
			if err != nil {
				return nil, err
			}
			f.preprocess = append(f.preprocess, segs)
			if segs.iterable() {
				iterable = true
			}
		}
	}

	if iterable {
		f.iterable = true
		f.count = math.MaxUint64 - 1
	}

	go f.run()

	return f, nil
}
