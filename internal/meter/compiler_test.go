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

	json := `
	{
       "bool": true,
       "int": 3,
       "float": 3.0,
       "string": "string",
       "map": {
           "k1": "this",
           "k2": 2
       },
       "list": [
           "line1", "line2"
       ]
    }
`
	bg.setLocalEnv("JSON", json)
	m := map[string]string{
		"LocalEnv: this is something $(LENV)":                                        "LocalEnv: this is something local variable",
		"GlobalEnv: this is something ${GENV}":                                       "GlobalEnv: this is something global variable",
		"echo: `echo \"$(FILE)\"` ends":                                              "echo: " + name + " ends",
		"echo: `echo` ends":                                                          "echo: input ends",
		"echo: `echo \"what oop\"s` ends":                                            "echo: what oops ends",
		"echo: `echo what oops` ends":                                                "echo: what oops ends",
		"cat: `cat " + name + "` ends":                                               "cat: " + content + " ends",
		"cat: `cat $(FILE)` ends":                                                    "cat: " + content + " ends",
		"write: `write -c $(FILE) ${OUT} | cat ${OUT}` ends":                         "write: " + name + " ends",
		"b64: `b64 \"hello world\"` hello world ":                                    "b64: " + enc + " hello world ",
		"b64: `b64 $(BASE)` hello world ":                                            "b64: " + enc + " hello world ",
		"b64 file: `b64 -f " + name + "` hello world ":                               "b64 file: " + fenc + " hello world ",
		"pipe: `cat " + name + " | b64` hello world ":                                "pipe: " + fenc + " hello world ",
		"env: `envw -c \"content\" ENVW |echo $(ENVW)`":                              "env: content",
		"env: `envw ENVW |echo $(ENVW)`":                                             "env: input",
		"env: `envw ENVW |envd ENVW | echo $(ENVW)`":                                 "env: ",
		"`assert 1 != 1 -h jgq $(OUTPUT)`":                                           "ERROR",
		"`assert 1 == 1 | assert 1.0 == 1.0 | assert abc == abc | echo $(ERROR)`":    "",
		"`assert 1 != 2 | assert 1.1 != 1.0 | assert abc != bbc | echo $(ERROR)`":    "",
		"`assert 2 >= 1 | assert 2 > 1 | assert 2 >= 2 | echo $(ERROR)`":             "",
		"`assert 1 <= 1 | assert 1 < 2 | assert 1 <= 2 | echo $(ERROR)`":             "",
		"`assert 2.0 >= 1.0 | assert 2.0 > 1.0 | assert 2.0 >= 2.0 | echo $(ERROR)`": "",
		"`assert 1.0 <= 1.0 | assert 1.0 < 2.0 | assert 1.0 <= 2.0 | echo $(ERROR)`": "",
		"`assert !false | assert true | assert !0 | assert 1 | echo $(ERROR)`":       "",
		"`assert 1 != 1 | echo $(ERROR)`":                                            "ERROR",
		"`json -e .bool $(JSON) | echo $(ERROR)`":                                    "",
		"`json  .bool $(JSON)`":                                                      "1",
		"`json  .int $(JSON) | assert $(OUTPUT) == 3 | echo $(ERROR)`":               "",
		"`json  .float $(JSON) | assert $(OUTPUT) == 3.0 | echo $(ERROR)`":           "",
		"`json  .string $(JSON)`":                                                    "string",
		"`json  .map.k1 $(JSON)`":                                                    "this",
		"`json  -n .list $(JSON)`":                                                   "2",
		"`json  .list.[1] $(JSON)`":                                                  "line2",
		"`json  .list $(JSON) | json [1]. `":                                         "line2",
		"cvt-d: `cvt -i 3.00`":                                                       "cvt-d: 3",
		"`envw -c jgq TEMP | echo I am $(TEMP)`":                                     "I am jgq",
	}

	for k, v := range m {
		bg.setOutput("input") // output will be put into input while pipeline starts
		bg.setError(nil)

		t.Log(k)
		if seg, err := makeSegments(k); err != nil {
			t.Fatal(err)
		} else {
			if res, err := seg.compose(bg); err != nil {
				if v != "ERROR" {
					t.Fatal(err)
				} else {
					t.Log("get error: ", err)
				}
			} else if res != v {
				if v == "NE" {
					if len(res) == 0 {
						t.Fatalf("expect not empty, got empty")
					}
				} else {
					t.Fatalf("expect %s, get %s", v, res)
				}
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
		} else if !isEof(err) {
			t.Fatalf("expect EOF, got %s", err)
		}
	}

}
