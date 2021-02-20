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
       ],
		"deep": {
		   "map": {
			   "k1": "this",
			   "k2": 2
		   }
		}
    }
`
	bg.setLocalEnv("JSON", json)
	var m map[string]string
	if true {
		m = map[string]string{
			"LocalEnv: this is something $(LENV)":                                        "LocalEnv: this is something local variable",
			"GlobalEnv: this is something ${GENV}":                                       "GlobalEnv: this is something global variable",
			"echo: `echo \"$(FILE)\"` ends":                                              "echo: " + name + " ends",
			"echo: `echo` ends":                                                          "echo:  ends",
			"echo: `echo \"what oop\"s` ends":                                            "echo: what oops ends",
			"echo: `echo what oops` ends":                                                "echo: what oops ends",
			"cat: `cat " + name + "` ends":                                               "cat: " + content + " ends",
			"cat: `cat $(FILE)` ends":                                                    "cat: " + content + " ends",
			"write: `write -c $(FILE) ${OUT} | cat ${OUT}` ends":                         "write: " + name + " ends",
			"b64: `b64 \"hello world\"` hello world ":                                    "b64: " + enc + " hello world ",
			"b64: `b64 $(BASE)` hello world ":                                            "b64: " + enc + " hello world ",
			"b64 file: `b64 -f " + name + "` hello world ":                               "b64 file: " + fenc + " hello world ",
			"pipe: `cat " + name + " | b64` hello world ":                                "pipe: " + fenc + " hello world ",
			"env: `env -w ENVW \"content\" |echo $(ENVW)`":                               "env: content",
			"env: `env -w ENVW input  |echo $(ENVW)`":                                    "env: input",
			"env: `env -w ENVW |env -d ENVW | echo $(ENVW)`":                             "env: ",
			"`env -w SRC hello | env -m SRC DST |env DST`":                               "hello",
			"`assert 1 != 1`":                                                            "ERROR",
			"`assert 1 == 1 | assert 1.0 == 1.0 | assert abc == abc | echo $(ERROR)`":    "",
			"`assert 1 != 2 | assert 1.1 != 1.0 | assert abc != bbc | echo $(ERROR)`":    "ERROR",
			"`assert 2 >= 1 | assert 2 > 1 | assert 2 >= 2 | echo $(ERROR)`":             "",
			"`assert 1 <= 1 | assert 1 < 2 | assert 1 <= 2 | echo $(ERROR)`":             "",
			"`assert 2.0 >= 1.0 | assert 2.0 > 1.0 | assert 2.0 >= 2.0 | echo $(ERROR)`": "",
			"`assert 1.0 <= 1.0 | assert 1.0 < 2.0 | assert 1.0 <= 2.0 | echo $(ERROR)`": "",
			"`assert !false | assert true | assert !0 | assert 1 | echo $(ERROR)`":       "",
			"`assert 1 != 1 | echo $(ERROR)`":                                            "ERROR",
			"`json -e .bool $(JSON) | echo $(ERROR)`":                                    "",
			"`json  .bool $(JSON)`":                                                      "1",
			"`json  .int $(JSON) | assert $$ == 3 | echo $(ERROR)`":                      "",
			"`json  .float $(JSON) | assert $$ == 3.0 | echo $(ERROR)`":                  "",
			"`json  .string $(JSON)`":                                                    "string",
			"`json  .map.k1 $(JSON)`":                                                    "this",
			"`json  -n .list $(JSON)`":                                                   "2",
			"`json  .list.[1] $(JSON)`":                                                  "line2",
			"`json  .list $(JSON) | json [1]. `":                                         "line2",
			"`json  -m .map $(JSON) | echo $(k1)`":                                       "this",
			"`json  -m .map $(JSON) | assert $(k2) == 2`":                                "",
			"`json  -m . $(JSON) | assert $(deep.map.k2) == 2`":                          "",
			"`assert $(@echo 3) == 3`":                                                   "",
			"`assert 3 != 2 && 1 < 2 | assert 10 > 9`":                                   "",
			"`assert 3 != 2 || 1 > 2 | assert 1 > 0`":                                    "",
			"cvt-d: `cvt -i 3.00`":                                                       "cvt-d: 3",
			"`cvt -i 3.i0`":                                                              "ERROR",
			"`cvt -b 0`":                                                                 "`false`",
			"`cvt -b 1`":                                                                 "`true`",
			"`cvt -b false`":                                                             "`false`",
			"`cvt -b true`":                                                              "`true`",
			"`cvt -b FALSE`":                                                             "`false`",
			"`cvt -b TRUE`":                                                              "`true`",
			"`cvt -b 11`":                                                                "ERROR",
			"`cvt -f 11.11.11`":                                                          "ERROR",
			"`cvt -i 11.11`":                                                             "ERROR",
			"`cvt -i 11`":                                                                "`11`",
			"`env -w TEMP jgq | echo I am $(TEMP)`":                                      "I am jgq",
			"`cvt parse`":                                                                "parse",
			"`strrepl jiangguoqing jiang zhu`":                                           "zhuguoqing",
			"`fail whatever is wrong`":                                                   "ERROR",
			"`if true then echo jiang`":                                                  "jiang",
			"`if false then echo jiang else echo guoqing`":                               "guoqing",
			"`strlen $(@echo jiang)`":                                                    "5",
			"`assert $(@json .int $(JSON)) == 3`":                                        "",
			"`assert $(@echo $(@echo $(@echo 3))) == $(@echo 3)`":                        "",
			"`if 3 == 3 then echo $(@echo 3 | env -w HELLO | env -r HELLO)`":             "3",
			"`eval 3+1|cvt -i`":                                                          "`4`",
			"`eval 3+1==4`":                                                              _true,
			"`print 江国庆`":                                                                "",
			"print 江国庆":                                                                  "print 江国庆",
		}
	} else {
		m = map[string]string{
			"`assert !false | assert true | assert !0 | assert 1 | echo $(ERROR)`": "",
		}
	}

	for k, v := range m {
		//bg.setOutput("input") // output will be put into input while pipeline starts
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
func TestSegmentMakeFailure(t *testing.T) {
	bg := &background{
		name:   "",
		local:  makeSimpEnv(),
		global: makeSimpEnv(),
		lr:     nil,
		err:    nil,
	}

	var m []string
	if true {
		m = []string{
			"`eval 3 + 5",
			"$(hello",
			"$(@hello",
			"${hello",
			"$<hello",
			"$",
			"$$",
			"$$$",
			"$()",
			"${}",
			"$<>",
			"`not a command`",
			"`cvt -f 1 1`",
			"`cvt -ff 1`",
			"`cvt -ff $(hello`",
		}
	} else {
		m = []string{"$$$"}
	}

	for _, s := range m {
		bg.setError(nil)

		t.Log(s)
		if _, err := makeSegments(s); err == nil {
			t.Fatal("expect fail, got pass")
		}
	}
}
func TestFunction(t *testing.T) {
	bg := &background{
		name:   "",
		local:  makeSimpEnv(),
		global: makeSimpEnv(),
		lr:     nil,
		err:    nil,
	}

	exceed := []string{
		"`eval $1 + $10`",
	}
	exceeding, _ := makeGroup(exceed, false)

	add := []string{
		"`print $0 $1 $2`",
		"`eval $1 + $2`",
	}
	adder, _ := makeGroup(add, false)

	mul := []string{
		//"`print mul $1 $2`",
		"`eval $1*$2`",
	}
	muler, _ := makeGroup(mul, false)

	comp := []string{
		//"`print compound $1 $2`",
		"`eval $(@call add $1 $2) + $(@call mul $1 $2)`",
	}
	comper, _ := makeGroup(comp, false)
	bg.functions = map[string]composable{
		"exceed": exceeding,
		"add":    adder,
		"mul":    muler,
		"comp":   comper,
	}

	m := map[string]string{
		"`call add 3 5 | assert $$ == 8`":   "",
		"`call mul 3 5 | assert $$ == 15`":  "",
		"`call comp 3 5 | assert $$ == 23`": "",
		"`call comp 3`":                     "ERROR",
		"`call exceed 3 3 3`":               "ERROR",
	}
	for k, v := range m {
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

}
