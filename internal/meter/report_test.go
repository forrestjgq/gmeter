package meter

import (
	"bufio"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/forrestjgq/glog"

	"github.com/forrestjgq/gmeter/config"
)

func TestReportDefault(t *testing.T) {
	path := "./test_report_default"
	cfg := &config.Report{
		Path:      path,
		Format:    "`echo $(TSEQ)`",
		Templates: nil,
	}

	rpt, err := makeReporter(cfg)
	if err != nil {
		t.Fatalf(err.Error())
	}

	time.Sleep(10 * time.Millisecond)

	bg, err := makeBackground(nil, nil)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if bg.rpt != nil {
		bg.rpt.close()
	}
	bg.rpt = rpt

	for i := 0; i < 100; i++ {
		bg.setLocalEnv("TSEQ", strconv.Itoa(i))
		rpt.reportDefault(bg, true)
		if bg.hasError() {
			t.Fatalf(bg.getError().Error())
		}
	}

	rpt.close()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf(err.Error())
	}

	scan := bufio.NewScanner(f)
	seq := 0
	for scan.Scan() {
		s := scan.Text()
		if len(s) == 0 {
			break
		}
		d := strconv.Itoa(seq)
		if s != d {
			t.Fatalf("%s != %s", s, d)
		}
		seq++
	}
	if seq != 100 {
		glog.Fatalf("seq %d", seq)
	}

	_ = f.Close()
	_ = os.Remove(path)
}
func TestReportTemplate(t *testing.T) {
	type Res struct {
		Seq int
	}
	path := "./test_report_default"
	cfg := &config.Report{
		Path:   path,
		Format: "`echo hello $(TSEQ)`",
		Templates: map[string]json.RawMessage{
			"com": json.RawMessage("{\"Seq\": \"`cvt -i $(TSEQ)`\"}"),
		},
	}

	rpt, err := makeReporter(cfg)
	if err != nil {
		t.Fatalf(err.Error())
	}

	time.Sleep(10 * time.Millisecond)

	bg, err := makeBackground(nil, nil)
	if err != nil {
		t.Fatalf(err.Error())
	}
	if bg.rpt != nil {
		bg.rpt.close()
	}
	bg.rpt = rpt

	for i := 0; i < 100; i++ {
		bg.setLocalEnv("TSEQ", strconv.Itoa(i))
		rpt.reportTemplate(bg, "com", true)
		if bg.hasError() {
			t.Fatalf(bg.getError().Error())
		}
	}

	rpt.close()

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf(err.Error())
	}

	scan := bufio.NewScanner(f)
	seq := 0
	for scan.Scan() {
		s := scan.Text()
		if len(s) == 0 {
			break
		}
		var v Res
		err = json.Unmarshal([]byte(s), &v)
		if err != nil {
			t.Fatalf(err.Error())
		}

		if seq != v.Seq {
			t.Fatalf("%d != %d", seq, v.Seq)
		}
		seq++
	}
	if seq != 100 {
		glog.Fatalf("seq %d", seq)
	}

	_ = f.Close()
	_ = os.Remove(path)
}
