package meter

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/pkg/errors"
)

//
// {
//     "name": "steven",
//     "age: optional": "`assert $$ > 20`",
//
// }

type jsonProp int

const (
	jsonPropOptional jsonProp = iota
	jsonPropAbsent            // item must not be present
	jsonPropIndex             // item behaves as index
)

const (
	jsonEnvKey    = "key"
	jsonEnvValue  = "value"
	jsonEnvLength = "length"
)
const (
	indDefault = "default"
)

type jsonEnv struct {
	value interface{}
	simp  env
}

func (j *jsonEnv) get(key string) string {
	v := j.simp.get(key)

	// dynamic marshal value
	if len(v) == 0 && !j.simp.has(key) {
		if key == jsonEnvValue {
			s, err := getStringOfValue(j.value)
			if err == nil {
				j.simp.put(key, s)
				v = s
			}
		} else if key == jsonEnvLength {
			if j.value != nil {
				if list, ok := j.value.([]interface{}); ok {
					return strconv.Itoa(len(list))
				} else if str, ok := j.value.(string); ok {
					return strconv.Itoa(utf8.RuneCountInString(str))
				}
			}
			v = "0"
		}
	}

	return v
}

func (j *jsonEnv) put(key string, value string) {
	j.simp.put(key, value)
}

func (j *jsonEnv) delete(key string) {
	j.simp.delete(key)
}

func (j *jsonEnv) has(key string) bool {
	return j.simp.has(key)
}

func (j *jsonEnv) dup() env {
	panic("dup should not be called")
}
func (j *jsonEnv) pop(bg *background) {
	bg.popJsonEnv()
}

func makeJsonEnv(bg *background, key string, value interface{}) *jsonEnv {
	je := &jsonEnv{
		value: value,
		simp:  makeSimpEnv(),
	}
	je.simp.put(jsonEnvKey, key)
	bg.pushJsonEnv(je)

	return je
}

// support value that is:
// "`xxxxx`", or
// ["`xxxxx`", "`yyyyy`",...]
func makeDynamic(value interface{}) (composable, error) {
	if v, ok := value.([]interface{}); ok {
		var slist []string
		fail := false
		for _, s := range v {
			if str, yes := s.(string); yes {
				slist = append(slist, str)
			} else {
				fail = true
				break
			}
		}
		if !fail {
			value = slist
		}
	}
	switch v := value.(type) {
	case []string:
		var list []string
		for _, s := range v {
			if len(s) < 2 || s[0] != '`' || s[len(s)-1] != '`' {
				return nil, errors.New("default with a list should be `string`")
			}
			if len(s) > 2 {
				list = append(list, s)
			}
		}
		c, err := makeGroup(list, false)
		if err != nil {
			return nil, err
		}
		return c, nil
	case string:
		if len(v) > 2 && v[0] == '`' && v[len(v)-1] == '`' {
			c, err := makeSegments(v)
			if err != nil {
				return nil, err
			}
			return c, nil
		}
		return nil, errors.New("default object accept only string or string list")
	default:
		return nil, fmt.Errorf("default object accept only string or string list, %T is not accepted", value)
	}
}

////////////////////////////////////////////////////////////////////////////////
// jsonDynamicRule:
//     default processing for object members not explicitly defined
////////////////////////////////////////////////////////////////////////////////

// process for members of json object that is not explicitly defined in jsonObject
type jsonDynamicRule struct {
	comp composable
}

func (j *jsonDynamicRule) getKey() *jsonKey {
	return nil
}

func (j *jsonDynamicRule) compare(bg *background, key string, src interface{}) error {
	defer makeJsonEnv(bg, key, src).pop(bg)
	_, err := j.comp.compose(bg)
	return err
}

func makeDynamicRule(value interface{}) (jsonRule, error) {
	jod := &jsonDynamicRule{}

	c, err := makeDynamic(value)
	if err != nil {
		return nil, err
	}
	jod.comp = c
	return jod, nil
}
func getStringOfValue(src interface{}) (string, error) {
	e := ""
	switch v := src.(type) {
	case bool:
		if v {
			e = "true"
		} else {
			e = "false"
		}
	case string:
		e = v
	case float64:
		e = strconv.FormatFloat(v, 'f', 8, 64)
	case json.Number:
		e = string(v)
	case []interface{}, map[string]interface{}:
		b, err := json.Marshal(src)
		if err != nil {
			return "", err
		}
		e = string(b)
	}

	return e, nil
}

type jsonKey struct {
	key  string
	prop []jsonProp
}

func (k *jsonKey) verify(bg *background, key string, value interface{}) error {
	if key != k.key {
		return fmt.Errorf("key not match: %s vs %s", key, k.key)
	}

	if value == nil {
		if !k.acceptNil() {
			return fmt.Errorf("json obj %s not accept nil", k.key)
		}
		return nil
	}

	if k.has(jsonPropAbsent) {
		return fmt.Errorf("json obj %s must be nil", k.key)
	}
	return nil
}
func (k *jsonKey) acceptNil() bool {
	return k.has(jsonPropOptional) || k.has(jsonPropAbsent)
}
func (k *jsonKey) has(prop jsonProp) bool {
	for _, p := range k.prop {
		if p == prop {
			return true
		}
	}

	return false
}

func makeJsonKey(s string) (*jsonKey, error) {
	key := &jsonKey{}
	s = strings.TrimSpace(s)
	arr := strings.Split(s, ":")
	if len(arr) == 0 {
		return nil, errors.New("key must not be empty")
	}
	if len(arr) > 2 {
		return nil, fmt.Errorf("invalid key definition: %s", s)
	}

	key.key = arr[0]
	if len(arr) == 2 {
		ts := strings.TrimSpace(arr[1])
		if len(ts) != 0 {
			opts := strings.Split(ts, ",")
			for _, opt := range opts {
				opt = strings.TrimSpace(opt)
				switch opt {
				case "optional":
					key.prop = append(key.prop, jsonPropOptional)
				case "absent":
					key.prop = append(key.prop, jsonPropAbsent)
				case "index":
					key.prop = append(key.prop, jsonPropIndex)
				default:
					return nil, fmt.Errorf("unknown property %s in %s", opt, s)
				}
			}
		}
	}

	return key, nil
}

type jsonRule interface {
	compare(bg *background, key string, src interface{}) error
	getKey() *jsonKey
}

// jsonStaticValue is a value defined by static int/float/bool/string
type jsonStaticValue struct {
	key   *jsonKey
	value interface{}
	str   string
}

func (jsv *jsonStaticValue) getKey() *jsonKey {
	return jsv.key
}

func (jsv *jsonStaticValue) compare(bg *background, key string, src interface{}) error {
	if key != jsv.key.key {
		return fmt.Errorf("static value: key not match: %s -> %s", key, jsv.key.key)
	}

	switch dv := jsv.value.(type) {
	case bool, string:
		if src != jsv.value {
			return fmt.Errorf("static compare fail: %v != %v", jsv.value, src)
		}
	case float64:
		sf := float64(0)
		switch sv := src.(type) {
		case float64:
			sf = sv
		case json.Number:
			v, err := sv.Float64()
			if err != nil {
				return fmt.Errorf("compare fail: %s != %v, convert %v to float fail", jsv.str, src, src)
			}
			sf = v
		default:
			return fmt.Errorf("compare fail: %s != %v, not support type of src: %v", jsv.str, src, src)
		}

		if math.Abs(sf-dv) > 0.0000001 {
			return fmt.Errorf("compare fail: %s != %v", jsv.str, src)
		}
	case json.Number:
		switch sv := src.(type) {
		case float64:
			// src: float, dst: number
			if df, err := dv.Float64(); err != nil {
				return fmt.Errorf("compare fail: %s != %v, convert %v to float fail", jsv.str, src, jsv.value)
			} else {
				if math.Abs(df-sv) > 0.0000001 {
					return fmt.Errorf("compare fail: %s != %v", jsv.str, src)
				}
			}
		case json.Number:
			s, err1 := sv.Float64()
			d, err2 := dv.Float64()
			fail := false
			if err1 == nil && err2 == nil {
				if math.Abs(s-d) > 0.0000001 {
					fail = true
				}
			} else {
				fail = true
			}

			if fail {
				i1, err3 := sv.Int64()
				i2, err4 := dv.Int64()
				if err3 == nil && err4 == nil {
					if i1 != i2 {
						fail = true
					}
				}
			}

			if fail {
				return fmt.Errorf("compare fail: %s != %v", jsv.str, src)
			}
		default:
			return fmt.Errorf("compare fail: %s != %v, not support type of src: %v", jsv.str, src, src)
		}
	default:
		return fmt.Errorf("unsupported json static type %T, value %v", src, src)
	}
	return nil
}

func makeJsonStaticValue(key *jsonKey, value interface{}) (jsonRule, error) {
	jsv := &jsonStaticValue{key: key}
	jsv.value = value

	switch v := value.(type) {
	case bool:
		if v {
			jsv.str = "1"
		} else {
			jsv.str = "0"
		}
	case float64:
		jsv.str = strconv.FormatFloat(v, 'f', 8, 64)
	case string:
		if len(v) >= 2 && (v[0] == '`' || v[len(v)-1] == '`') {
			return nil, fmt.Errorf("not a static value: %s", v)
		}
		jsv.str = v
	case json.Number:
		jsv.str = v.String()
	default:
		return nil, fmt.Errorf("invalid static json value type: %T, value %v", value, value)
	}
	return jsv, nil
}

////////////////////////////////////////////////////////////////////////////////
// jsonDynamicValue:
//     json dynamic value is used for a key-value in which value is a embedded
//     command or a command list
//
//     only basic value can be applied on jsonDynamicValue like boo, numbers,
//     string, nil...
////////////////////////////////////////////////////////////////////////////////
type jsonDynamicValue struct {
	key  *jsonKey
	comp composable
}

func (jdv *jsonDynamicValue) getKey() *jsonKey {
	return jdv.key
}

func (jdv *jsonDynamicValue) compare(bg *background, key string, src interface{}) error {
	defer makeJsonEnv(bg, key, src).pop(bg)
	if err := jdv.key.verify(bg, key, src); err != nil {
		return err
	}

	_, err := jdv.comp.compose(bg)
	if err != nil {
		return err
	}

	return nil
}

// like: "a": "`assert $ > 1 && assert $ < 3`"
// like: "a": [
//           "`assert $ > 1 && assert $ < 3`",
//           "`envw -c $ VAR`"
//        ]
//
func makeJsonDynamicValue(key *jsonKey, v interface{}) (jsonRule, error) {
	c, err := makeDynamic(v)
	if err != nil {
		return nil, err
	}

	jdv := &jsonDynamicValue{key: key, comp: c}
	return jdv, nil
}

////////////////////////////////////////////////////////////////////////////////
// jsonObject:
//     json object processing
////////////////////////////////////////////////////////////////////////////////

type jsonObject struct {
	key     *jsonKey
	rules   map[string]jsonRule // rules applied on the whole object, like `default`
	members map[string]jsonRule
	index   map[string]jsonRule
}

func (j *jsonObject) getKey() *jsonKey {
	return j.key
}

func (j *jsonObject) hasIndex() bool {
	return len(j.index) > 0
}
func (j *jsonObject) isIndexOptional() bool {
	for _, v := range j.index {
		if !v.getKey().has(jsonPropOptional) {
			return false
		}
	}
	return true
}
func (j *jsonObject) match(bg *background, key string, src interface{}) error {
	// this is a try matching, any error should be abandon
	errstr := bg.getError()
	defer bg.setError(errstr)

	defer makeJsonEnv(bg, key, src).pop(bg)

	if src == nil {
		src = make(map[string]interface{})
	}

	m, ok := src.(map[string]interface{})
	if !ok {
		return fmt.Errorf("json obj %s expect an object, but got %v", j.key.key, src)
	}

	for k, r := range j.index {
		// has rules for this member
		if err := r.compare(bg, k, m[k]); err != nil {
			return err
		}
	}

	return nil
}
func (j *jsonObject) compare(bg *background, key string, src interface{}) error {
	defer makeJsonEnv(bg, key, src).pop(bg)
	if err := j.key.verify(bg, key, src); err != nil {
		return err
	}

	m, ok := src.(map[string]interface{})
	if !ok {
		return fmt.Errorf("json obj %s expect an object, but got %v", j.key.key, src)
	}

	// rules before member comparing

	// member comparing
	keys := make(map[string]int)
	for k := range j.members {
		keys[k] = 1
	}

	for k, v := range m {
		if rule, ok := j.members[k]; ok {
			// has rules for this member
			if err := rule.compare(bg, k, v); err != nil {
				return err
			}
			if _, exist := keys[k]; !exist {
				return fmt.Errorf("duplicate member %s", k)
			}
			delete(keys, k)
		} else {
			// no rules for this, check default
			if def, exist := j.rules[indDefault]; exist {
				if err := def.compare(bg, k, v); err != nil {
					return err
				}
			}
		}
	}
	// those not compared
	for k := range keys {
		mkey := j.members[k].getKey()
		if mkey != nil {
			if !mkey.acceptNil() {
				return fmt.Errorf("%s.%s must exist", key, mkey.key)
			}
		}
	}

	// rules after member comparing
	return nil
}

func makeJsonObject(key *jsonKey, value map[string]interface{}) (*jsonObject, error) {
	if key == nil {
		key = &jsonKey{}
	}
	obj := &jsonObject{
		key:     key,
		rules:   make(map[string]jsonRule),
		members: make(map[string]jsonRule),
		index:   make(map[string]jsonRule),
	}
	for k, v := range value {
		if len(k) >= 2 && k[0] == '`' && k[len(k)-1] == '`' {
			// dynamic members should be parsed and saved in rules
			ind := k[1 : len(k)-1]
			switch ind {
			case indDefault:
				if r, err := makeDynamicRule(v); err != nil {
					return nil, err
				} else {
					obj.rules[ind] = r
				}

			default:
				return nil, errors.New("invalid json object ind: " + ind)
			}
		} else {
			// member processing
			key, err := makeJsonKey(k)
			if err != nil {
				return nil, err
			}
			r, err := makeJsonRule(key, v)
			if err != nil {
				return nil, err
			}
			if r != nil {
				obj.members[key.key] = r
				if key.has(jsonPropIndex) {
					obj.index[key.key] = r
				}
			}
		}
	}

	return obj, nil
}

////////////////////////////////////////////////////////////////////////////////
// jsonList:
//     json list processing
////////////////////////////////////////////////////////////////////////////////
type jsonList struct {
	key      *jsonKey
	rules    map[string]jsonRule // rules applied on the whole object, like `default`
	members  []jsonRule
	searcher []*jsonObject
}

func (j *jsonList) getKey() *jsonKey {
	return j.key
}

func (j *jsonList) compare(bg *background, key string, src interface{}) error {
	defer makeJsonEnv(bg, key, src).pop(bg)

	value, ok := src.([]interface{})
	if !ok {
		return fmt.Errorf("not a list to compare: %T(%v)", src, src)
	}

	// compare list
	if r, ok := j.rules["list"]; ok {
		if err := r.compare(bg, key, src); err != nil {
			return err
		}
	}

	hasItem := false
	if r, ok := j.rules["item"]; ok {
		hasItem = true
		for i := range value {
			if err := r.compare(bg, "", value[i]); err != nil {
				return err
			}
		}
	}
	if r, ok := j.rules["template"]; ok {
		hasItem = true
		for i := range value {
			if err := r.compare(bg, "", value[i]); err != nil {
				return err
			}
		}
	}

	if len(j.members) > 0 {
		if len(value) < len(j.members) {
			return fmt.Errorf("data length %d < %d", len(value), len(j.members))
		}

		for i := range j.members {
			if err := j.members[i].compare(bg, "", value[i]); err != nil {
				return err
			}
		}

		value = value[len(j.members):]
	} else if len(j.searcher) > 0 {
		// for each searcher, find an item that matches, and compare it.
		// if no matching is found or matched comparing failed, fail the list compare
		for k, srch := range j.searcher {
			found := false
			for i, dst := range value {
				if srch.match(bg, "", dst) == nil {
					if err := srch.compare(bg, "", dst); err != nil {
						return err
					}
					value = append(value[:i], value[i+1:]...)
					found = true
					break
				}
			}
			if !found && !srch.isIndexOptional() {
				return fmt.Errorf("searcher %d fails", k)
			}
		}
	}

	if len(value) > 0 {
		// items not processed by member and searcher should be processed by default if any
		if r, ok := j.rules["default"]; ok {
			for i := range value {
				if err := r.compare(bg, "", value[i]); err != nil {
					return err
				}
			}
		} else if !hasItem {
			return fmt.Errorf("no default process for the rest of list items")
		}
	}
	return nil
}

func tryMakeDynamicList(key *jsonKey, value []interface{}) jsonRule {
	var l []string
	for _, v := range value {
		if s, ok := v.(string); ok {
			l = append(l, s)
		} else {
			return nil
		}
	}
	if r, err := makeJsonDynamicValue(key, l); err != nil {
		return nil
	} else {
		return r
	}
}

// [
//   {
//        "`list`": "`xxxxx`" or ["`xxxxx`", "`yyyyy`"],
//        "`default`": "`xxxxx`" or ["`xxxxx`", "`yyyyy`"]
//   },
//   ...
// ]
//
func makeJsonList(key *jsonKey, value []interface{}) (jsonRule, error) {
	// "xxx": ["`yyy`", "`zzz`"]
	if key != nil {
		r := tryMakeDynamicList(key, value)
		if r != nil {
			return r, nil
		}
	} else {
		key = &jsonKey{}
	}

	list := &jsonList{
		key:   key,
		rules: make(map[string]jsonRule),
	}

	if len(value) > 0 {
		// check if first item is a json object and all its keys are `xxx`
		if m, ok := value[0].(map[string]interface{}); ok {
			valid := true
			for k := range m {
				if len(k) <= 2 || k[0] != '`' || k[len(k)-1] != '`' {
					valid = false
					break
				}
			}
			if valid {
				for k, v := range m {
					var r jsonRule
					var err error
					typ := k[1 : len(k)-1]

					switch k {
					case "`list`", "`item`", "`default`":
						r, err = makeDynamicRule(v)
					case "`template`":
						r, err = makeJsonTemplateFromValue(v)
					default:
						err = fmt.Errorf("unknown list operation: %s", k)
					}

					if err != nil {
						return nil, err
					}
					list.rules[typ] = r
				}
				value = value[1:]
			}
		}
	}

	if len(value) > 0 {
		// plays as default key
		key = &jsonKey{
			key:  "",
			prop: nil,
		}

		switch v := value[0].(type) {
		case bool, json.Number, float64:
			for _, item := range value {
				r, err := makeJsonStaticValue(key, item)
				if err != nil {
					return nil, err
				}
				list.members = append(list.members, r)
			}
		case nil: // skip
		case string:
			// string that is "`xxx`" is a dynamic value, or "xxx" is a static value
			for _, item := range value {
				r, err := makeJsonDynamicValue(key, item)
				if err != nil {
					r, err = makeJsonStaticValue(key, item)
					if err != nil {
						return nil, err
					}
				}
				list.members = append(list.members, r)
			}
		case []interface{}:
			for i, item := range value {
				if sub, ok := item.([]interface{}); ok {
					r, err := makeJsonList(key, sub)
					if err != nil {
						return nil, err
					}
					list.members = append(list.members, r)
				} else {
					return nil, fmt.Errorf("expect %T but got %T(%v) in list[%d]", v, item, item, i)
				}
			}
		case map[string]interface{}:
			for i, item := range value {
				if mv, ok := item.(map[string]interface{}); !ok {
					return nil, fmt.Errorf("expect %T but got %T(%v) in list[%d]", v, mv, mv, i)
				} else {
					r, err := makeJsonObject(key, mv)
					if err != nil {
						return nil, err
					}
					if r.hasIndex() {
						list.searcher = append(list.searcher, r)
					} else {
						list.members = append(list.members, r)
					}
				}
			}
		default:
			return nil, fmt.Errorf("unsupported json value type %T, value: %v", value, value)
		}
	}

	if len(list.searcher) > 0 && len(list.members) > 0 {
		return nil, fmt.Errorf("you can not partially search in the list %s", key.key)
	}
	return list, nil
}

func makeJsonRule(key *jsonKey, value interface{}) (jsonRule, error) {
	switch v := value.(type) {
	case string:
		r, err := makeJsonDynamicValue(key, value)
		if err != nil {
			r, err = makeJsonStaticValue(key, value)
		}
		return r, err
	case json.Number, bool, float64:
		return makeJsonStaticValue(key, value)
	case nil: // do nothing
		return nil, nil
	case []interface{}:
		return makeJsonList(key, v)
	case map[string]interface{}:
		return makeJsonObject(key, v)
	default:
		return nil, fmt.Errorf("unsupported json value type %T, value: %v", value, value)
	}
}
func makeJsonTemplateFromValue(value interface{}) (jsonRule, error) {
	switch v := value.(type) {
	case []interface{}:
		return makeJsonList(nil, v)
	case map[string]interface{}:
		return makeJsonObject(nil, v)
	default:
		return nil, fmt.Errorf("json type %T is not supported", v)
	}

}
func makeJsonTemplate(raw json.RawMessage) (jsonRule, error) {
	if string(raw) == "null" {
		return nil, nil
	}
	var value interface{}
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}

	return makeJsonTemplateFromValue(value)
}

func compareTemplate(template jsonRule, bg *background, msg string) error {

	var v interface{}
	err := json.Unmarshal([]byte(msg), &v)
	if err != nil {
		return err
	}

	return template.compare(bg, "", v)
}
