package meter

import (
	"encoding/base64"
	"io/ioutil"
	"os"
	"testing"
)

func TestSegments(t *testing.T) {
	bg := &background{
		name:   "",
		seq:    0,
		local:  makeSimpEnv(),
		global: makeSimpEnv(),
		lr:     nil,
		err:    nil,
	}

	os.TempDir()
	f, err := ioutil.TempFile(os.TempDir(), "test_seg_*")
	if err != nil {
		t.Fatal(err)
	}
	name := f.Name()
	out := name + "_out"
	base := "hello world"
	bg.setLocalEnv("NAME", name)
	bg.setLocalEnv("BASE", base)
	bg.setGlobalEnv("OUT", out)

	content := "line1\nline2\n"
	_, _ = f.WriteString(content)
	_ = f.Close()

	defer func() {
		_ = os.Remove(name)
	}()

	bg.setGlobalEnv("GENV", "global variable")
	bg.setLocalEnv("LENV", "local variable")
	bg.setLocalEnv("FILE", name)
	enc := base64.StdEncoding.EncodeToString([]byte(base))
	fenc := base64.StdEncoding.EncodeToString([]byte(content))

	m := map[string]string{
		"LocalEnv: this is something $(LENV)":             "LocalEnv: this is something local variable",
		"GlobalEnv: this is something ${GENV}":            "GlobalEnv: this is something global variable",
		"echo: `echo $(FILE)` ends":                       "echo: " + name + " ends",
		"echo: `echo` ends":                               "echo: input ends",
		"cat: `cat " + name + "` ends":                    "cat: " + content + " ends",
		"cat: `cat $(FILE)` ends":                         "cat: " + content + " ends",
		"write: `write ${OUT} $(FILE) | cat ${OUT}` ends": "write: " + name + " ends",
		"b64: `b64 \"hello world\"` hello world ":         "b64: " + enc + " hello world ",
		"b64: `b64 $(BASE)` hello world ":                 "b64: " + enc + " hello world ",
		"b64 file: `b64 -f " + name + "` hello world ":    "b64 file: " + fenc + " hello world ",
		"pipe: `cat " + name + " | b64` hello world ":     "pipe: " + fenc + " hello world ",
	}

	for k, v := range m {
		bg.setOutput("input")

		t.Log(k)
		if seg, err := makeSegments(k); err != nil {
			t.Fatal(err)
		} else {
			if res, err := seg.compose(bg); err != nil {
				t.Fatal(err)
			} else if res != v {
				t.Fatalf("expect %s, get %s", v, res)
			}
		}
	}

	res := []string{"line1", "line2"}
	list := "`list " + name + "`"
	if seg, err := makeSegments(list); err != nil {
		t.Fatal(err)
	} else {
		for _, line := range res {
			if out, err := seg.compose(bg); err != nil {
				t.Fatal(err)
			} else if out != line {
				t.Fatalf("expect %s, get %s", line, out)
			}
		}
		_, err = seg.compose(bg)
		if err == nil {
			t.Fatal("expect EOF")
		} else if err.Error() != EOF {
			t.Fatalf("expect EOF, got %s", err)
		}
	}

}
