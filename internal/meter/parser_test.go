package meter

import "testing"

func TestYyParser(t *testing.T) {
	m := map[string]string{
		"abc == bc":                            _false,
		"bc == bc":                             _true,
		"'abc' == 'bc'":                        _false,
		"'bc' == 'bc'":                         _true,
		" 22 > 33":                             _false,
		" 22 >= 22":                            _true,
		" 22 < 33":                             _true,
		" 22 <= 22":                            _true,
		" 22 == 33":                            _false,
		" 22 != 33":                            _true,
		"V=3; VAR=43; $(VAR) > 33":             _true,
		"V=3; VAR=43; $(VAR) > 33 && $(V) < 4": _true,
		"V=3; VAR=43; $(VAR) > 33 || $(V) < 1": _true,
		"V=3; VAR=43; $(VAR) < 33 || $(V) < 1": _false,
		"V=3; VAR=43; ($(VAR) < 33 && $(V) < 1) || ($(VAR) > 40 && $(V) > 1)": _true,
		"3 < 2; VAR=43; $(VAR) > 33":                                          _true,
		"$(a)":                                                                "",
		"a=TRUE;$(a)":                                                         "TRUE",
		"a=FALSE;$(a)":                                                        "FALSE",
		"a=true;$(a)":                                                         "true",
		"a=false;$(a)":                                                        "false",
		"a='hello';$(a)":                                                      "hello",
		"(2 < 3)":                                                             _true,
		"$(@echo jiang)":                                                      "jiang",
		"$<key>":                                                              "KEY",
		"$":                                                                   "VALUE",
		"OUTPUT='output'; $$":                                                 "output",
		"22.3 == 22.299999":                                                   _false,
		"22.3 == 22.299999999999":                                             _true,
		"22.3 != 22.299999999999":                                             _false,
		"22 != 21.999999999999":                                               _false,
		"22 == 21.999999999999":                                               _true,
		"${GLOBAL}":                                                           "global",
		"a=1; ++$(a)":                                                         "2",
		"a=1; $(a)++":                                                         "2",
		"a=1; $(a)--":                                                         "0",
		"a=1; --$(a)":                                                         "0",
		"a=1; --$(a)--":                                                       "-1",
		"!(2 < 3)":                                                            _false,
		"!(2 > 3)":                                                            _true,
		"!(TRUE)":                                                             _false,
		"!FALSE":                                                              _true,
		"! 'false'":                                                           _true,
		"45==12+33":                                                           _true,
		"0.5==3-2.5":                                                          _true,
		"7.5==3*2.5":                                                          _true,
		"3.5==7/2":                                                            _true,
		"1==7%2":                                                              _true,
		"1!=7%2":                                                              _false,
		"a=1;a+=2;$(a)==3":                                                    _true,
		"a=3;a-=2;$(a)==1":                                                    _true,
		"a=3;a*=2;$(a)==6":                                                    _true,
		"a=9;a/=3;$(a)==3":                                                    _true,
		"a=9;a%=3;$(a)==0":                                                    _true,
		"3 > 2 == TRUE":                                                       _true,
		"3 > 2 != TRUE":                                                       _false,
		"3 < 2 == TRUE":                                                       _false,
		"3 < 2 != TRUE":                                                       _true,
	}

	for k, v := range m {
		//t.Logf("run %s, expect %s", k, v)

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

		c := makeExpression(k)
		t.Logf("%v, expect %s", c, v)
		res, err := c.compose(bg)
		if err != nil {
			t.Fatalf(err.Error())
		} else if res != v {
			t.Fatalf("expect %s got %s", v, res)
		}
	}
}
