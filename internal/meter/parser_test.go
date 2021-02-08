package meter

import "testing"

func TestYyParser(t *testing.T) {
	m := map[string]string{
		"abc == bc":                            "FALSE",
		"bc == bc":                             "TRUE",
		"'abc' == 'bc'":                        "FALSE",
		"'bc' == 'bc'":                         "TRUE",
		" 22 > 33":                             "FALSE",
		" 22 >= 22":                            "TRUE",
		" 22 < 33":                             "TRUE",
		" 22 <= 22":                            "TRUE",
		" 22 == 33":                            "FALSE",
		" 22 != 33":                            "TRUE",
		"V=3; VAR=43; $(VAR) > 33":             "TRUE",
		"V=3; VAR=43; $(VAR) > 33 && $(V) < 4": "TRUE",
		"V=3; VAR=43; $(VAR) > 33 || $(V) < 1": "TRUE",
		"V=3; VAR=43; $(VAR) < 33 || $(V) < 1": "FALSE",
		"V=3; VAR=43; ($(VAR) < 33 && $(V) < 1) || ($(VAR) > 40 && $(V) > 1)": "TRUE",
		"3 < 2; VAR=43; $(VAR) > 33":                                          "TRUE",
		"$(a)":                                                                "",
		"a=TRUE;$(a)":                                                         "TRUE",
		"a=FALSE;$(a)":                                                        "FALSE",
		"a=true;$(a)":                                                         "true",
		"a=false;$(a)":                                                        "false",
		"a='hello';$(a)":                                                      "hello",
		"(2 < 3)":                                                             "TRUE",
		"$(@echo jiang)":                                                      "jiang",
		"$<key>":                                                              "KEY",
		"$":                                                                   "VALUE",
		"OUTPUT='output'; $$":                                                 "output",
		"22.3 == 22.299999":                                                   "FALSE",
		"22.3 == 22.299999999999":                                             "TRUE",
		"22.3 != 22.299999999999":                                             "FALSE",
		"22 != 21.999999999999":                                               "FALSE",
		"22 == 21.999999999999":                                               "TRUE",
		"${GLOBAL}":                                                           "global",
		"a=1; ++$(a)":                                                         "2",
		"a=1; $(a)++":                                                         "2",
		"a=1; $(a)--":                                                         "0",
		"a=1; --$(a)":                                                         "0",
		"a=1; --$(a)--":                                                       "-1",
		"!(2 < 3)":                                                            "FALSE",
		"!(2 > 3)":                                                            "TRUE",
		"!(TRUE)":                                                             "FALSE",
		"!FALSE":                                                              "TRUE",
		"45==12+33":                                                           "TRUE",
		"0.5==3-2.5":                                                          "TRUE",
		"7.5==3*2.5":                                                          "TRUE",
		"3.5==7/2":                                                            "TRUE",
		"1==7%2":                                                              "TRUE",
		"1!=7%2":                                                              "FALSE",
		"a=1;a+=2;$(a)==3":                                                    "TRUE",
		"a=3;a-=2;$(a)==1":                                                    "TRUE",
		"a=3;a*=2;$(a)==6":                                                    "TRUE",
		"a=9;a/=3;$(a)==3":                                                    "TRUE",
		"a=9;a%=3;$(a)==0":                                                    "TRUE",
		"3 > 2 == TRUE":                                                       "TRUE",
		"3 > 2 != TRUE":                                                       "FALSE",
		"3 < 2 == TRUE":                                                       "FALSE",
		"3 < 2 != TRUE":                                                       "TRUE",
	}

	for k, v := range m {
		t.Logf("run %s, expect %s", k, v)
		s := &Scanner{}
		s.Init([]byte(k), func(pos int, msg string) {
			t.Fatalf("scan fail: %s", msg)
		})
		yyDebug = 0
		//yyErrorVerbose = true
		yyParse(s)

		if yyComposer == nil {
			t.Fatalf("expect non-nil yy composer")
		} else {
			bg, err := createDefaultBackground()
			if err != nil {
				t.Fatalf(err.Error())
			}
			bg.setGlobalEnv("GLOBAL", "global")

			type A struct {
				a int
			}
			je := &jsonEnv{
				value: &A{a: 1},
				simp:  makeSimpEnv(),
			}
			je.simp.put(jsonEnvValue, "VALUE")
			je.simp.put(jsonEnvKey, "KEY")
			bg.pushJsonEnv(je)

			res, err := yyComposer.compose(bg)
			if err != nil {
				t.Fatalf(err.Error())
			} else if res != v {
				t.Fatalf("expect %s got %s", v, res)
			}
		}

	}
}
