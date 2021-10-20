package meter

import (
	"net/http"
	"sync"
)

type httpcFactory interface {
	Get(create bool) *http.Client
}

type httpcWrapper struct {
	current *http.Client
	creator func() *http.Client
}

func (w *httpcWrapper) Get(create bool) *http.Client {
	if w.current == nil {
		create = true
	}
	if create {
		w.current = w.creator()
	}
	return w.current
}

func createHttpClientWrapper(creator func() *http.Client) httpcFactory  {
	return &httpcWrapper{
		current: nil,
		creator: creator,
	}
}

type concurrentHttpcWrapper struct {
	mtx sync.Mutex
	current *http.Client
	creator func() *http.Client
}

func (w *concurrentHttpcWrapper) Get(create bool) *http.Client {
	w.mtx.Lock()
	defer w.mtx.Unlock()

	if w.current == nil {
		create = true
	}
	if create {
		w.current = w.creator()
	}
	return w.current
}
func createConcurrentHttpClientWrapper(creator func() *http.Client) httpcFactory  {
	return &concurrentHttpcWrapper {
		mtx: sync.Mutex{},
		current: nil,
		creator: creator,
	}
}

