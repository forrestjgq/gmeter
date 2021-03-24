package meter

import (
	"encoding/json"
	"testing"
)

func makeBg() *background {
	return &background{
		name:      "test",
		local:     makeSimpEnv(),
		global:    makeSimpEnv(),
		dyn:       nil,
		err:       nil,
		rpt:       nil,
		predefine: make(map[string]string),
	}
}

func success(t *testing.T, src, dst string) {
	r, err := makeJsonTemplate(json.RawMessage(src))
	if err != nil {
		t.Fatalf(err.Error())
	}

	bg := makeBg()

	var v interface{}
	err = json.Unmarshal([]byte(dst), &v)
	if err != nil {
		t.Fatalf(err.Error())
	}

	err = r.compare(bg, "", v)
	if err != nil {
		t.Fatalf(err.Error())
	}
}
func fail(t *testing.T, src, dst string) {
	r, err := makeJsonTemplate(json.RawMessage(src))
	if err != nil {
		t.Fatalf(err.Error())
	}

	bg := makeBg()

	var v interface{}
	err = json.Unmarshal([]byte(dst), &v)
	if err != nil {
		t.Fatalf(err.Error())
	}
	err = r.compare(bg, "", v)
	if err == nil {
		t.Fatalf("expect fail, but success: \nsrc: %s\ndst:%s", src, dst)
	}
}

func TestStatic(t *testing.T) {
	src := `
{
	"int": 11,
	"neg-int": -11,
	"float": 11.12,
	"neg-float": -11.12,
	"bool-false": false,
	"bool-true": true,
	"empty-str": "",
	"num-str": "111",
	"bool-str": "true",
	"normal-str": "this is a string",
	"nil": null
}
`
	success(t, src, src)
}
func TestStaticInt(t *testing.T) {
	src := `{ "int": 11, "neg-int": -11, "extra: optional": 110} `

	f1 := `{ "int": 12, "neg-int": -11, "extra: optional": 110} `   // 11 != 12
	f2 := `{ "int": 11, "neg-int": -12, "extra: optional": 110} `   // -11 != -12
	f3 := `{ "int": "11", "neg-int": -12, "extra: optional": 110} ` // "11" is a string
	f4 := `{ "int": 11, "extra: optional": 110} `                   // missing member
	f5 := `{ "int": 11, "extra: optional": 111} `                   // optional incorrect
	fail(t, src, f1)
	fail(t, src, f2)
	fail(t, src, f3)
	fail(t, src, f4)
	fail(t, src, f5)

	s1 := `{ "int": 11, "neg-int": -11} `
	s2 := `{ "int": 11, "neg-int": -11, "gib": 0}` // extra should be ignored
	success(t, src, s1)
	success(t, src, s2)
}
func TestStaticBool(t *testing.T) {
	src := `{"bool": true}`
	fail(t, src, `{"bool": false}`)
	fail(t, src, `{"bool": "true"}`)
	fail(t, src, `{}`)
	success(t, src, src)
}
func TestStaticFloat(t *testing.T) {
	src := `{"float": 1}`
	fail(t, src, `{"float": 1.1}`)
	fail(t, src, `{"float": 1.000001}`)
	success(t, src, `{"float": 1.00000001}`)
	success(t, src, `{"float": 1.0}`)

	src = `{"float": 1.00000001}`
	success(t, src, `{"float": 1}`)
	fail(t, src, `{"float": "1.00000001"}`)
}
func TestStaticAbsent(t *testing.T) {
	src := `{"bool: absent": true}` // either absent, or must be true
	fail(t, src, `{"bool": false}`)
	success(t, src, `{}`)
	success(t, src, `{"bool": true}`)
}

func TestDynamic(t *testing.T) {
	src := "{ \"int1\": \"`assert $ > 1 | assert $ < 9`\", \"int2\": [\"`assert $ > 1`\", \"`assert $ < 9`\"] }"

	fail(t, src, `{"int1": 1, "int2": 2}`)
	fail(t, src, `{"int1": 2, "int2": 1}`)
	success(t, src, `{"int1": 2, "int2": 2}`)
}

func TestListStatic(t *testing.T) {
	src := `[1, 2, 3, 4]`
	fail(t, src, `[1, 2, 3]`)
	fail(t, src, `[1, 2, 3, 4, 5]`)
	fail(t, src, `[1, 3, 4, 5]`)
	fail(t, src, `[]`)
	success(t, src, src)
}
func TestListDynamic(t *testing.T) {
	src := "[ \"`assert $ > 1`\", \"`assert $ > 2`\"]"
	success(t, src, `[2, 3]`)
	fail(t, src, `[ 2, 2]`)
}
func TestListBasic(t *testing.T) {
	src := "[ { \"`list`\": [ \"`json  -n .[] $ | assert $$ >= 3`\" ], \"`item`\":  \"`assert $ > 0`\"  }, 1, 2 ]"
	fail(t, src, `[1, 2]`)       // len >= 3
	fail(t, src, `[1, 2, 3, 0]`) // item > 0
	fail(t, src, `[1, 3]`)       // must start with 1, 2
	success(t, src, `[1, 2, 3, 4, 5]`)
	src = "[ {\"`default`\":  \"`assert $ > 0`\"  }, 1, 2 ]"
	success(t, src, `[1, 2, 3, 4, 5]`)
}
func TestListString(t *testing.T) {
	src := "[ \"abc\", \"def\", \"`strlen $ | assert $$ > 3`\" ]"
	s1 := ` [ "abc", "def", "sssss" ]`
	success(t, src, s1)
	f1 := ` [ "abc", "def", "s" ]`
	fail(t, src, f1)
}
func TestListStringList(t *testing.T) {
	src := "[[ \"abc\", \"def\", \"`strlen $ | assert $$ > 3`\" ],[ \"abc\", \"def\", \"`strlen $ | assert $$ > 3`\" ],[ \"abc\", \"def\", \"`strlen $ | assert $$ > 3`\" ]]"
	s1 := ` [[ "abc", "def", "sssss" ],[ "abc", "def", "sssss" ],[ "abc", "def", "sssss" ]]`
	success(t, src, s1)
	f1 := ` [[ "abc", "def", "sssss" ], [ "abc", "def", "sssss" ], [ "abc", "def", "s" ]]`
	fail(t, src, f1)
}

func TestListTemplate(t *testing.T) {
	src := "[ { \"`template`\": { \"a\": \"`assert $ > 10`\", \"b\": \"`assert $`\", \"c\": \"`assert $ < 5`\" } } ]"
	s1 := `

  [
    {
      "a": 11,
      "b": true,
      "c": 3
    },
    {
      "a": 12,
      "b": true,
      "c": 1,
      "d": 10
    }
  ]
`
	success(t, src, s1)
	f1 := `

  [
    {
      "a": 10,
      "b": true,
      "c": 3
    }
  ]
`
	fail(t, src, f1)
	f2 := `

  [
    {
      "a": 11,
      "b": false,
      "c": 3
    }
  ]
`
	fail(t, src, f2)
	f3 := `

  [
    {
      "a": 11,
      "c": 3
    }
  ]
`
	fail(t, src, f3)
}

/*
[
  {
    "`default`": "`json .b $ | assert $$ < 0`"
  },
  {
    "a: index": "`assert $ == 1`",
    "b": "`assert $ > 1`"
  },
  {
    "a: index": "`assert $ == 2`",
    "b: index": "`assert $ == 1`",
    "c": "assert $ > 3`"
  }
]
*/
func TestListSearch(t *testing.T) {
	src := "[ { \"`default`\": \"`json .b $ | assert $$ < 0`\" }, { \"a: index\": \"`assert $ == 1`\", \"b\": \"`assert $ > 1`\" }, { \"a: index\": \"`assert $ == 2`\", \"b: index\": \"`assert $ == 1`\", \"c\": \"`assert $ > 3`\" } ]"
	s1 := `
[
  {
    "a": 1,
    "b": 20
  },
  {
    "a": 2,
    "b": 1,
    "c": 30
  },
  {
    "a": 12,
    "b": -1
  }
]
`
	success(t, src, s1)

	f1 := `
[
  {
    "a": 3,
    "b": 20
  },
  {
    "a": 2,
    "b": 1,
    "c": 30
  },
  {
    "a": 12,
    "b": -1
  }
]
`
	fail(t, src, f1)

	f2 := `
[
  {
    "a": 1,
    "b": 20
  },
  {
    "a": 2,
    "b": 2,
    "c": 30
  },
  {
    "a": 12,
    "b": -1
  }
]
`
	fail(t, src, f2)

	f3 := `
[
  {
    "a": 1,
    "b": 20
  },
  {
    "a": 12,
    "b": -1
  }
]
`
	fail(t, src, f3)
	f4 := `
[
  {
    "a": 1,
    "b": 20
  },
  {
    "a": 2,
    "b": 1,
    "c": 30
  },
  {
    "a": 12,
    "b": 3
  }
]
`
	fail(t, src, f4)
}
func TestListSearchOptional(t *testing.T) {
	src := "[ { \"`default`\": \"`nop`\" }, { \"a: index, optional\": \"`assert $ == 1`\", \"b\": \"`assert $ > 1`\" } ]"
	s1 := `
[
  {
    "a": 1,
    "b": 20
  },
  {
    "a": 2,
    "b": 1,
    "c": 30
  },
  {
    "a": 12,
    "b": -1
  }
]
`
	success(t, src, s1)

	s2 := `
[
  {
    "a": 2,
    "b": 2,
    "c": 30
  },
  {
    "a": 12,
    "b": -1
  }
]
`
	success(t, src, s2)
}
func TestListCompareMember(t *testing.T) {
	src := "[ { \"`default`\": \"`json .b $ | assert $$ < 0`\" }, { \"a\": \"`assert $ >= 1`\", \"b\": \"`assert $ >= 10`\" }, { \"a\": \"`assert $ < 1`\", \"b\": \"`assert $ > 100`\", \"c\": \"`assert $ > 30`\" } ]"
	s1 := `
[
  {
    "a": 1,
    "b": 20
  },
  {
    "a": -2,
    "b": 101,
    "c": 40
  },
  {
    "a": 12,
    "b": -1
  }
]
`
	success(t, src, s1)

	f1 := `
[
  {
    "a": 0,
    "b": 20
  },
  {
    "a": -2,
    "b": 101,
    "c": 40
  }
]
`
	fail(t, src, f1)

	f2 := `
[
  {
    "a": 1,
    "b": 20
  },
  {
    "a": 2,
    "b": 101,
    "c": 40
  }
]
`
	fail(t, src, f2)

	f3 := `
[
  {
    "a": 1,
    "b": 20
  },
  {
    "a": -2,
    "b": 101
  }
]
`
	fail(t, src, f3)
	f4 := `
[
  {
    "a": 1,
    "b": 20
  },
  {
    "a": -2,
    "b": 100,
    "c": 40
  }
]
`
	fail(t, src, f4)
	f5 := `
[
  {
    "a": 1,
    "b": 20
  },
  {
    "a": -2,
    "b": 101,
    "c": 20
  }
]
`
	fail(t, src, f5)
}
func TestObjectCompare(t *testing.T) {
	src := "{ \"`default`\": [ \"`strlen $<key> | assert $$ > 4`\", \"`json .mc $<value> | assert $$ > 3`\" ], \"a\": 1, \"b\": \"hello\", \"c\": \"`assert $ > 10`\" }"
	s1 := `
  {
    "a": 1,
    "b": "hello",
    "c": 15,
    "hello": {
      "ma": 1,
      "mb": true,
      "mc": 20
    }
  }
`
	success(t, src, s1)
	f1 := `
  {
    "a": 1,
    "b": "hello",
    "c": 15,
    "hel": {
      "ma": 1,
      "mb": true,
      "mc": 20
    }
  }
`
	fail(t, src, f1)
}
func TestEmptyCommand(t *testing.T) {
	src := "{\"a\":\"`nop`\"}"
	s1 := ` { "a": 1 } `
	success(t, src, s1)
}
func TestObjectSubList(t *testing.T) {
	src := "{ \"a\": [ 1, 2, 3] }"
	s1 := ` { "a": [1, 2, 3] } `
	success(t, src, s1)
	f1 := ` { "a": [1, 2 ] } `
	fail(t, src, f1)
	f2 := ` { "a": [1, 2, 2 ] } `
	fail(t, src, f2)
	f3 := ` { "a": [1, 2, 3, 4 ] } `
	fail(t, src, f3)
}
func TestObjectChildObject(t *testing.T) {
	src := "{ \"a\": {\"b\":  1, \"c\":  \"`assert $ > 3`\"} }"
	s1 := ` { "a": {"b": 1, "c": 4} } `
	success(t, src, s1)
	f1 := ` { "a": {"b": 1, "c": 2} } `
	fail(t, src, f1)
}
func TestDemo(t *testing.T) {
	src := "{ \"`default`\": [ \"`print found key $<key>`\" ], \"a: optional\": 1, \"b\": \"`strlen $ | assert $$ > 10`\", \"c\": false, \"d\": [ { \"`list`\": [ \"`assert $<length> > 4`\" ], \"`item`\": [ \"`assert $ > 0`\" ], \"`default`\": [ \"`assert $ > 10`\" ] }, 1, 2, 3 ], \"e\": [ { \"`template`\": { \"name\": \"`strlen $<key> | assert $$ > 3`\", \"qty\": \"`assert $ > 10`\" } }, { \"name: index\": \"apple\", \"qty\": \"`assert $ > 1000`\" } ] }"
	s1 := `
  {
    "b": "abcdefg hijklmn opq",
    "c": false,

    "d": [
      1,
      2,
      3,
      12,
      13
    ],

    "e": [
      {
        "name": "apple",
        "qty": 1200
      }
    ]
  }
`
	success(t, src, s1)
	f1 := `
  {
    "b": "abcdefg hijklmn opq",
    "c": false,

    "d": [
      1,
      2,
      3,
      4,
      13
    ],

    "e": [
      {
        "name": "apple",
        "qty": 1200
      }
    ]
  }
`
	fail(t, src, f1)
}
