package meter

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/golang/glog"
)

type fileLineFeeder struct {
	f   *os.File
	s   *bufio.Scanner
	mtx sync.Mutex
}

func (f *fileLineFeeder) feed(bg *background) (string, error) {
	f.mtx.Lock()
	defer f.mtx.Unlock()

	for f.s.Scan() {
		t := f.s.Text()
		if len(t) > 0 {
			return t, nil
		}
	}

	_ = f.f.Close()
	err := f.s.Err()
	if err == nil {
		err = io.EOF
	}
	return "", err
}

func makeStrFeeder(path string) *fileLineFeeder {
	fl := &fileLineFeeder{}
	var err error
	fl.f, err = os.Open(filepath.Clean(path))
	if err != nil {
		glog.Error(err)
		return nil
	}

	fl.s = bufio.NewScanner(fl.f)
	return fl
}

func _strFeeder() {
	flf := makeStrFeeder("path to file")
	_ = &feedCombiner{
		headers: nil,
		url: func(bg *background) (string, error) {
			return flf.feed(bg)
		},
		method: nil,
		body:   nil,
	}
}
