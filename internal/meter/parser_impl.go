package meter

import (
	"math"
	"strconv"

	"github.com/golang/glog"

	"github.com/pkg/errors"
)

//go:generate goyacc -o parser_rule.go parser.yy
type composer interface {
	composable
	getError() error
	String() string
}

var yyComposer composer

type varType int32

const (
	varLocal varType = iota
	varGlobal
	varJson
	varArgument
)

type varReader struct {
	err  error
	name string
	typ  varType
	desc string
}

func (v *varReader) String() string {
	return v.desc
}
func (v *varReader) getError() error {
	return v.err
}

func (v *varReader) compose(bg *background) (string, error) {
	switch v.typ {
	case varLocal:
		return bg.getLocalEnv(v.name), nil
	case varGlobal:
		return bg.getGlobalEnv(v.name), nil
	case varJson:
		return bg.getJsonEnv(v.name), nil
	case varArgument:
		idx, err := strconv.Atoi(v.name)
		if err != nil {
			return "", errors.Wrapf(err, "convert argument index %s", v.name)
		}
		return bg.getArgument(idx)
	default:
		return "", errors.Errorf("unknown var type %d", v.typ)
	}
}

func makeVarReader(typ varType, str string) composer {
	//fmt.Printf("var(%s)\n", str)
	ret := &varReader{
		name: str,
		typ:  typ,
	}
	if len(str) == 0 {
		ret.err = errors.Errorf("var %d without a name", typ)
	} else {
		switch typ {
		case varLocal:
			ret.desc = "$(" + str + ")"
		case varGlobal:
			ret.desc = "${" + str + "}"
		case varJson:
			ret.desc = "$<" + str + ">"
		case varArgument:
			ret.desc = "$" + str
		default:
			ret.err = errors.Errorf("unknown var type %d", typ)
		}
	}
	return ret
}

type staticReader struct {
	str string
}

func (s staticReader) compose(_ *background) (string, error) {
	return s.str, nil
}

func (s staticReader) getError() error {
	return nil
}
func (s staticReader) String() string {
	return s.str
}

func makeStaticReader(str string) composer {
	return &staticReader{str: str}
}

type commandComposer struct {
	cmd  command
	err  error
	desc string
}

func (c *commandComposer) compose(bg *background) (string, error) {
	if c.cmd == nil {
		return "", nil
	}
	input := bg.getLocalEnv(KeyInput)
	defer bg.setInput(input)
	return c.cmd.execute(bg)
}

func (c *commandComposer) getError() error {
	return c.err
}
func (c *commandComposer) String() string {
	return c.desc
}

func makeCommand(str string) composer {
	c, err := parseCmd(str)
	cc := &commandComposer{
		cmd:  c,
		err:  err,
		desc: "`" + str + "`",
	}
	return cc
}

type unaryComposer struct {
	tok  Token
	c    composer
	err  error
	desc string
}

func (u *unaryComposer) compose(bg *background) (string, error) {
	s, err := u.c.compose(bg)
	if err != nil {
		return "", err
	}

	if u.tok == INC || u.tok == DEC {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return "", errors.Wrapf(err, "parse float of %s", s)
		}
		if u.tok == INC {
			f += 1.0
		} else {
			f -= 1.0
		}
		i := int(f)
		return strconv.Itoa(i), nil
	} else if u.tok == NOT {
		r := ""
		if isFalse(s) {
			r = _true
		} else if isTrue(s) {
			r = _false
		} else {
			return "", errors.Errorf("! can not apply to (%s)", s)
		}
		return r, nil

	} else {
		panic("unknown token")
	}
}

func (u *unaryComposer) getError() error {
	return u.err
}
func (u *unaryComposer) String() string {
	return u.desc + u.c.String()
}

func makeUnary(lhs composer, str string) composer {
	uc := &unaryComposer{
		c:    lhs,
		desc: str,
	}
	switch str {
	case "++":
		uc.tok = INC
	case "--":
		uc.tok = DEC
	case "!":
		uc.tok = NOT
	default:
		uc.err = errors.New("unknown unary operator " + str)
	}
	return uc
}
func makePostUnary(lhs composer, str string) composer {
	return makeUnary(lhs, str)
}
func makePreUnary(lhs composer, str string) composer {
	return makeUnary(lhs, str)
}
func makeNotUnary(lhs composer) composer {
	return makeUnary(lhs, "!")
}

type binCalc func(lhs, rhs string) (string, error)
type binaryComposer struct {
	calc     binCalc
	lhs, rhs composer
	err      error
	desc     string
}

func (b *binaryComposer) String() string {
	return b.lhs.String() + " " + b.desc + " " + b.rhs.String()
}

func (b *binaryComposer) compose(bg *background) (string, error) {
	lhs, err := b.lhs.compose(bg)
	if err != nil {
		return "", err
	}
	rhs, err := b.rhs.compose(bg)
	if err != nil {
		return "", err
	}

	return b.calc(lhs, rhs)
}

func (b *binaryComposer) getError() error {
	return b.err
}

func makeCalc(lhs composer, op string, rhs composer) composer {
	toNum := func(s string) (float64, error) {
		return strconv.ParseFloat(s, 64)
	}
	toBool := func(s string) (bool, error) {
		if isTrue(s) {
			return true, nil
		}
		if isFalse(s) {
			return false, nil
		}
		return false, errors.Errorf("expect a bool value: %s", s)
	}
	fToStr := func(f float64) string {
		return strconv.FormatFloat(f, 'f', 8, 64)
	}
	bToStr := func(b bool) string {
		if b {
			return _true
		} else {
			return _false
		}
	}
	arith := func(lhs, rhs string, f func(left, right float64) float64) (string, error) {
		l, err := toNum(lhs)
		if err != nil {
			return "", err
		}
		r, err := toNum(rhs)
		if err != nil {
			return "", err
		}
		return fToStr(f(l, r)), nil
	}

	logic := func(lhs, rhs string, f func(left, right bool) bool) (string, error) {
		l, err := toBool(lhs)
		if err != nil {
			return "", err
		}
		r, err := toBool(rhs)
		if err != nil {
			return "", err
		}
		return bToStr(f(l, r)), nil
	}
	comp := func(lhs, rhs string, f func(left, right float64) bool) (string, error) {
		l, err := toNum(lhs)
		if err != nil {
			return "", err
		}
		r, err := toNum(rhs)
		if err != nil {
			return "", err
		}
		return bToStr(f(l, r)), nil
	}
	var err error
	var calc binCalc
	switch op {
	case "+":
		calc = func(lhs, rhs string) (string, error) {
			return arith(lhs, rhs, func(left, right float64) float64 {
				return left + right
			})
		}
	case "-":
		calc = func(lhs, rhs string) (string, error) {
			return arith(lhs, rhs, func(left, right float64) float64 {
				return left - right
			})
		}
	case "*":
		calc = func(lhs, rhs string) (string, error) {
			return arith(lhs, rhs, func(left, right float64) float64 {
				return left * right
			})
		}
	case "/":
		calc = func(lhs, rhs string) (string, error) {
			return arith(lhs, rhs, func(left, right float64) float64 {
				if right == 0 {
					panic("div 0 error")
				}
				return left / right
			})
		}
	case "%":
		calc = func(lhs, rhs string) (string, error) {
			return arith(lhs, rhs, func(left, right float64) float64 {
				if right == 0 {
					panic("div 0 error")
				}
				return float64(int(left) % int(right))
			})
		}
	case "&&":
		calc = func(lhs, rhs string) (string, error) {
			return logic(lhs, rhs, func(left, right bool) bool {
				return left && right
			})
		}
	case "||":
		calc = func(lhs, rhs string) (string, error) {
			return logic(lhs, rhs, func(left, right bool) bool {
				return left || right
			})
		}
	case ">":
		calc = func(lhs, rhs string) (string, error) {
			return comp(lhs, rhs, func(left, right float64) bool {
				return left > right
			})
		}
	case ">=":
		calc = func(lhs, rhs string) (string, error) {
			return comp(lhs, rhs, func(left, right float64) bool {
				return left >= right
			})
		}
	case "<":
		calc = func(lhs, rhs string) (string, error) {
			return comp(lhs, rhs, func(left, right float64) bool {
				return left < right
			})
		}
	case "<=":
		calc = func(lhs, rhs string) (string, error) {
			return comp(lhs, rhs, func(left, right float64) bool {
				return left <= right
			})
		}
	case "!=", "==":
		calc = func(lhs, rhs string) (string, error) {
			if op == "==" && lhs == rhs {
				return _true, nil
			}

			f, err := comp(lhs, rhs, func(left, right float64) bool {
				if op == "!=" {
					return math.Abs(left-right) >= eps
				}
				return math.Abs(left-right) < eps
			})
			if err != nil {
				f, err = logic(lhs, rhs, func(left, right bool) bool {
					if op == "!=" {
						return left != right
					}
					return left == right
				})
			}
			if err != nil {
				return _false, nil
			}
			return f, nil
		}
	default:
		err = errors.Errorf("unknown bianry operator %s", op)
	}
	b := &binaryComposer{
		calc: calc,
		lhs:  lhs,
		rhs:  rhs,
		err:  err,
		desc: op,
	}
	return b
}

type assigner struct {
	err  error
	lhs  string
	rhs  composer
	desc string
}

func (a *assigner) String() string {
	return a.lhs + " " + a.desc + " " + a.rhs.String()
}

func (a *assigner) compose(bg *background) (string, error) {
	res, err := a.rhs.compose(bg)
	if err != nil {
		return "", err
	}

	bg.setLocalEnv(a.lhs, res)
	return "", nil
}

func (a *assigner) getError() error {
	return a.err
}

func makeAssign(lhs string, op string, rhs composer) composer {
	c := rhs
	var err error
	switch op {
	case "+=":
		left := makeVarReader(varLocal, lhs)
		c = makeCalc(left, "+", rhs)
	case "-=":
		left := makeVarReader(varLocal, lhs)
		c = makeCalc(left, "-", rhs)
	case "*=":
		left := makeVarReader(varLocal, lhs)
		c = makeCalc(left, "*", rhs)
	case "/=":
		left := makeVarReader(varLocal, lhs)
		c = makeCalc(left, "/", rhs)
	case "%=":
		left := makeVarReader(varLocal, lhs)
		c = makeCalc(left, "%", rhs)
	case "=":
		c = rhs
	default:
		err = errors.Errorf("unknown assign operator %s", op)
	}
	return &assigner{
		err:  err,
		lhs:  lhs,
		rhs:  c,
		desc: op,
	}
}

type combiner struct {
	lhs, rhs composer
}

func (c combiner) String() string {
	return c.lhs.String() + "; " + c.rhs.String()
}

func (c combiner) compose(bg *background) (string, error) {
	_, err := c.lhs.compose(bg)
	if err != nil {
		return "", err
	}
	return c.rhs.compose(bg)
}

func (c combiner) getError() error {
	return nil
}

func makeCombiner(lhs, rhs composer) composer {
	return &combiner{lhs: lhs, rhs: rhs}
}

func makeExpression(str string) composable {
	//fmt.Printf("make expression %s\n", str)
	s := &Scanner{}
	s.Init([]byte(str), func(pos int, msg string) {
		glog.Fatalf("expression parse %s fail: %s", str, msg)
	})
	yyParse(s)
	return yyComposer
}
