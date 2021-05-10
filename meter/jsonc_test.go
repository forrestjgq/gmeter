package meter

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func TestMakeJSONC(t *testing.T) {
	template := `
{
	"a": "'assert $ > 1'",
	"b": "'print b = $'",
	"c": "'assert $ == $(C)'",
	"d": "'env -w D $'"
}
`
	tpl := strings.ReplaceAll(template, "'", "`")
	j, err := MakeJSONC(json.RawMessage(tpl))
	if err != nil {
		t.Fatalf(err.Error())
	}
	j.Set("C", "10")

	msg := `
{
	"a": 2,
	"b": "hello world",
	"c": 10,
	"d": 101
}

`
	err = j.Compare(json.RawMessage(msg))
	if err != nil {
		t.Fatalf(err.Error())
	}
	v := j.Get("D")
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		t.Fatalf(err.Error())
	}
	i := int(f)
	if i != 101 {
		t.Fatalf("expect v 101, got %s", v)
	}
}
