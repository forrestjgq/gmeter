package config

import (
	"encoding/json"
	"reflect"
	"testing"
)

func vvv(v interface{}, t *testing.T) {

	if v == nil {
		t.Log("is nil")
	}
}
func TestName(t *testing.T) {
	var v interface{}
	v = nil

	if v == nil {
		t.Log("is nil")
	}
	vvv(v, t)

	if reflect.ValueOf(v).IsZero() {
		t.Log("is zero")
	}
	if reflect.ValueOf(v).IsNil() {
		t.Log("is nil")
	}
}

func TestConfig_AddHost(t *testing.T) {
	m := make(map[string]*float64)

	a := m["a"]
	t.Log("a = ", a)
	t.Log("len of m is ", len(m))

}

func TestConfig_AddMessage(t *testing.T) {
	a := []int{1, 2, 3, 4, 5}
	i := 0
	a = append(a[:i], a[i+1:]...)
	t.Log("i = 0: ", a)
	i = 1
	a = append(a[:i], a[i+1:]...)
	t.Log("i = 1: ", a)
	i = 2
	a = append(a[:i], a[i+1:]...)
	t.Log("i = 2: ", a)

	f1 := 1
	f2 := 1

	i1 := interface{}(f1)
	i2 := interface{}(f2)
	if i1 == i2 {
		t.Log("i equal")
	} else {
		t.Log("i not equal")
	}

	t.Log("i deep equal: ", reflect.DeepEqual(i1, i2))
	l := interface{}(string("111"))
	r := interface{}(json.Number("111"))
	if l == r {
		t.Log("equal")
	} else {
		t.Log("not equal")
	}
	t.Log("deep equal: ", reflect.DeepEqual(l, r))
}
