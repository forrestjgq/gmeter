package meter

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/forrestjgq/gmeter/config"
)

const (
	exitRpt = "gmeter-exit"
)

type reporter struct {
	c         chan string
	f         io.WriteCloser
	fmt       segments
	templates map[string]segments
	running   bool
}

// globalClose should only be called by root background
func (r *reporter) close() {
	if r.requireReport() {
		r.report(exitRpt, false)
		for r.running {
			time.Sleep(100 * time.Millisecond)
		}
		close(r.c)
	}
}
func (r *reporter) run() {
	r.running = true
	for c := range r.c {
		if c == exitRpt {
			break
		}
		if r.f != nil {
			_, _ = r.f.Write([]byte(c))
		}
	}
	if r.f != nil {
		_ = r.f.Close()
	}
	r.running = false
}
func (r *reporter) requireReport() bool {
	return r.running && r.c != nil
}
func (r *reporter) reportTemplate(bg *background, template string, newline bool) {
	if r.requireReport() && r.templates != nil {
		if t, ok := r.templates[template]; ok {
			str, err := t.compose(bg)
			if err != nil {
				bg.setError(err.Error())
			} else {
				r.report(str, newline)
			}
		}
	}
}
func (r *reporter) reportDefault(bg *background, newline bool) {
	if r.requireReport() {
		if r.fmt != nil {
			str, err := r.fmt.compose(bg)
			if err != nil {
				bg.setError(err.Error())
			} else {
				r.report(str, newline)
			}
		}
	}
}
func (r *reporter) report(content string, newline bool) {
	if r.requireReport() {
		r.c <- content
		if newline {
			r.c <- "\n"
		}
	}
}
func makeReporter(rpt *config.Report) (*reporter, error) {
	var err error
	path := rpt.Path
	r := &reporter{}

	if len(path) > 0 {
		dir := filepath.Dir(path)
		if err = os.MkdirAll(dir, os.ModePerm); err != nil {
			return nil, err
		}
		r.f, err = os.Create(path)
		if err != nil {
			return nil, err
		}
		fmt.Printf("report will be written to %s\n", path)
	} else {
		r.f = os.Stdout
		fmt.Printf("report will be written to stdout\n")
	}

	r.c = make(chan string, 1000)
	if len(rpt.Format) > 0 {
		r.fmt, err = makeSegments(rpt.Format)
		if err != nil {
			return nil, err
		}
	}
	r.templates = make(map[string]segments)
	if len(rpt.Templates) > 0 {
		for k, v := range rpt.Templates {
			t, err := makeSegments(string(v))
			if err != nil {
				return nil, err
			}
			r.templates[k] = t
		}
	}
	go r.run()
	return r, nil
}
