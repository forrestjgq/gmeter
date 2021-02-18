// Code generated by goyacc -o parser_rule.go parser.yy. DO NOT EDIT.

//line parser.yy:2
package meter

import __yyfmt__ "fmt"

//line parser.yy:2

//line parser.yy:7
type yySymType struct {
	yys  int
	str  string
	tok  Token
	err  error
	comp composer
}

const IDENTITY = 57346
const LITERAL = 57347
const UNARY_ARITH_OP = 57348
const COMP_OP = 57349
const EQUAL_OP = 57350
const LOGIC_OP = 57351
const AND_OP = 57352
const OR_OP = 57353
const ASSIGN_OP = 57354
const V_LOCAL = 57355
const V_GLOBAL = 57356
const V_JSON = 57357
const CMD_EXEC = 57358
const V_ARGUMENT = 57359

var yyToknames = [...]string{
	"$end",
	"error",
	"$unk",
	"IDENTITY",
	"LITERAL",
	"UNARY_ARITH_OP",
	"COMP_OP",
	"EQUAL_OP",
	"LOGIC_OP",
	"AND_OP",
	"OR_OP",
	"ASSIGN_OP",
	"V_LOCAL",
	"V_GLOBAL",
	"V_JSON",
	"CMD_EXEC",
	"V_ARGUMENT",
	"';'",
	"'('",
	"')'",
	"'!'",
	"'*'",
	"'/'",
	"'%'",
	"'+'",
	"'-'",
}

var yyStatenames = [...]string{}

const yyEofCode = 1
const yyErrCode = 2
const yyInitialStackSize = 16

//line parser.yy:163

//line yacctab:1
var yyExca = [...]int{
	-1, 1,
	1, -1,
	-2, 0,
}

const yyPrivate = 57344

const yyLast = 60

var yyAct = [...]int{
	12, 7, 10, 30, 31, 11, 51, 25, 37, 17,
	14, 32, 33, 34, 9, 36, 38, 18, 19, 20,
	22, 21, 8, 23, 2, 15, 6, 4, 42, 5,
	17, 14, 45, 48, 49, 50, 46, 47, 18, 19,
	20, 22, 21, 44, 23, 24, 15, 26, 27, 40,
	43, 39, 41, 28, 29, 35, 1, 3, 13, 16,
}

var yyPact = [...]int{
	25, 27, -1000, -1000, -1000, -5, 36, 38, 45, 47,
	-22, -11, -1000, 49, 4, 4, -1000, -1000, -1000, -1000,
	-1000, -1000, -1000, 4, 25, 4, 4, 4, 4, 4,
	4, 4, 4, 4, 4, -1000, -1000, -1000, -1000, -14,
	-1000, 36, 38, 45, 47, -22, -11, -11, -1000, -1000,
	-1000, -1000,
}

var yyPgo = [...]int{
	0, 59, 58, 0, 5, 2, 14, 22, 1, 26,
	27, 57, 56, 24,
}

var yyR1 = [...]int{
	0, 12, 12, 13, 13, 10, 11, 1, 1, 1,
	1, 1, 1, 1, 1, 2, 2, 3, 3, 3,
	4, 4, 4, 4, 5, 5, 5, 6, 6, 7,
	7, 8, 8, 9, 9,
}

var yyR2 = [...]int{
	0, 1, 3, 1, 1, 1, 3, 1, 1, 1,
	1, 1, 1, 1, 3, 1, 2, 1, 2, 2,
	1, 3, 3, 3, 1, 3, 3, 1, 3, 1,
	3, 1, 3, 1, 3,
}

var yyChk = [...]int{
	-1000, -12, -13, -11, -10, 4, -9, -8, -7, -6,
	-5, -4, -3, -2, 6, 21, -1, 5, 13, 14,
	15, 17, 16, 19, 18, 12, 11, 10, 8, 7,
	25, 26, 22, 23, 24, 6, -3, 4, -3, -10,
	-13, -9, -8, -7, -6, -5, -4, -4, -3, -3,
	-3, 20,
}

var yyDef = [...]int{
	0, -2, 1, 3, 4, 8, 5, 33, 31, 29,
	27, 24, 20, 17, 0, 0, 15, 7, 9, 10,
	11, 12, 13, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 16, 18, 8, 19, 0,
	2, 6, 34, 32, 30, 28, 25, 26, 21, 22,
	23, 14,
}

var yyTok1 = [...]int{
	1, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 3,
	3, 3, 3, 21, 3, 3, 3, 24, 3, 3,
	19, 20, 22, 25, 3, 26, 3, 23, 3, 3,
	3, 3, 3, 3, 3, 3, 3, 3, 3, 18,
}

var yyTok2 = [...]int{
	2, 3, 4, 5, 6, 7, 8, 9, 10, 11,
	12, 13, 14, 15, 16, 17,
}

var yyTok3 = [...]int{
	0,
}

var yyErrorMessages = [...]struct {
	state int
	token int
	msg   string
}{}

//line yaccpar:1

/*	parser for yacc output	*/

var (
	yyDebug        = 0
	yyErrorVerbose = false
)

type yyLexer interface {
	Lex(lval *yySymType) int
	Error(s string)
}

type yyParser interface {
	Parse(yyLexer) int
	Lookahead() int
}

type yyParserImpl struct {
	lval  yySymType
	stack [yyInitialStackSize]yySymType
	char  int
}

func (p *yyParserImpl) Lookahead() int {
	return p.char
}

func yyNewParser() yyParser {
	return &yyParserImpl{}
}

const yyFlag = -1000

func yyTokname(c int) string {
	if c >= 1 && c-1 < len(yyToknames) {
		if yyToknames[c-1] != "" {
			return yyToknames[c-1]
		}
	}
	return __yyfmt__.Sprintf("tok-%v", c)
}

func yyStatname(s int) string {
	if s >= 0 && s < len(yyStatenames) {
		if yyStatenames[s] != "" {
			return yyStatenames[s]
		}
	}
	return __yyfmt__.Sprintf("state-%v", s)
}

func yyErrorMessage(state, lookAhead int) string {
	const TOKSTART = 4

	if !yyErrorVerbose {
		return "syntax error"
	}

	for _, e := range yyErrorMessages {
		if e.state == state && e.token == lookAhead {
			return "syntax error: " + e.msg
		}
	}

	res := "syntax error: unexpected " + yyTokname(lookAhead)

	// To match Bison, suggest at most four expected tokens.
	expected := make([]int, 0, 4)

	// Look for shiftable tokens.
	base := yyPact[state]
	for tok := TOKSTART; tok-1 < len(yyToknames); tok++ {
		if n := base + tok; n >= 0 && n < yyLast && yyChk[yyAct[n]] == tok {
			if len(expected) == cap(expected) {
				return res
			}
			expected = append(expected, tok)
		}
	}

	if yyDef[state] == -2 {
		i := 0
		for yyExca[i] != -1 || yyExca[i+1] != state {
			i += 2
		}

		// Look for tokens that we accept or reduce.
		for i += 2; yyExca[i] >= 0; i += 2 {
			tok := yyExca[i]
			if tok < TOKSTART || yyExca[i+1] == 0 {
				continue
			}
			if len(expected) == cap(expected) {
				return res
			}
			expected = append(expected, tok)
		}

		// If the default action is to accept or reduce, give up.
		if yyExca[i+1] != 0 {
			return res
		}
	}

	for i, tok := range expected {
		if i == 0 {
			res += ", expecting "
		} else {
			res += " or "
		}
		res += yyTokname(tok)
	}
	return res
}

func yylex1(lex yyLexer, lval *yySymType) (char, token int) {
	token = 0
	char = lex.Lex(lval)
	if char <= 0 {
		token = yyTok1[0]
		goto out
	}
	if char < len(yyTok1) {
		token = yyTok1[char]
		goto out
	}
	if char >= yyPrivate {
		if char < yyPrivate+len(yyTok2) {
			token = yyTok2[char-yyPrivate]
			goto out
		}
	}
	for i := 0; i < len(yyTok3); i += 2 {
		token = yyTok3[i+0]
		if token == char {
			token = yyTok3[i+1]
			goto out
		}
	}

out:
	if token == 0 {
		token = yyTok2[1] /* unknown char */
	}
	if yyDebug >= 3 {
		__yyfmt__.Printf("lex %s(%d)\n", yyTokname(token), uint(char))
	}
	return char, token
}

func yyParse(yylex yyLexer) int {
	return yyNewParser().Parse(yylex)
}

func (yyrcvr *yyParserImpl) Parse(yylex yyLexer) int {
	var yyn int
	var yyVAL yySymType
	var yyDollar []yySymType
	_ = yyDollar // silence set and not used
	yyS := yyrcvr.stack[:]

	Nerrs := 0   /* number of errors */
	Errflag := 0 /* error recovery flag */
	yystate := 0
	yyrcvr.char = -1
	yytoken := -1 // yyrcvr.char translated into internal numbering
	defer func() {
		// Make sure we report no lookahead when not parsing.
		yystate = -1
		yyrcvr.char = -1
		yytoken = -1
	}()
	yyp := -1
	goto yystack

ret0:
	return 0

ret1:
	return 1

yystack:
	/* put a state and value onto the stack */
	if yyDebug >= 4 {
		__yyfmt__.Printf("char %v in %v\n", yyTokname(yytoken), yyStatname(yystate))
	}

	yyp++
	if yyp >= len(yyS) {
		nyys := make([]yySymType, len(yyS)*2)
		copy(nyys, yyS)
		yyS = nyys
	}
	yyS[yyp] = yyVAL
	yyS[yyp].yys = yystate

yynewstate:
	yyn = yyPact[yystate]
	if yyn <= yyFlag {
		goto yydefault /* simple state */
	}
	if yyrcvr.char < 0 {
		yyrcvr.char, yytoken = yylex1(yylex, &yyrcvr.lval)
	}
	yyn += yytoken
	if yyn < 0 || yyn >= yyLast {
		goto yydefault
	}
	yyn = yyAct[yyn]
	if yyChk[yyn] == yytoken { /* valid shift */
		yyrcvr.char = -1
		yytoken = -1
		yyVAL = yyrcvr.lval
		yystate = yyn
		if Errflag > 0 {
			Errflag--
		}
		goto yystack
	}

yydefault:
	/* default state action */
	yyn = yyDef[yystate]
	if yyn == -2 {
		if yyrcvr.char < 0 {
			yyrcvr.char, yytoken = yylex1(yylex, &yyrcvr.lval)
		}

		/* look through exception table */
		xi := 0
		for {
			if yyExca[xi+0] == -1 && yyExca[xi+1] == yystate {
				break
			}
			xi += 2
		}
		for xi += 2; ; xi += 2 {
			yyn = yyExca[xi+0]
			if yyn < 0 || yyn == yytoken {
				break
			}
		}
		yyn = yyExca[xi+1]
		if yyn < 0 {
			goto ret0
		}
	}
	if yyn == 0 {
		/* error ... attempt to resume parsing */
		switch Errflag {
		case 0: /* brand new error */
			yylex.Error(yyErrorMessage(yystate, yytoken))
			Nerrs++
			if yyDebug >= 1 {
				__yyfmt__.Printf("%s", yyStatname(yystate))
				__yyfmt__.Printf(" saw %s\n", yyTokname(yytoken))
			}
			fallthrough

		case 1, 2: /* incompletely recovered error ... try again */
			Errflag = 3

			/* find a state where "error" is a legal shift action */
			for yyp >= 0 {
				yyn = yyPact[yyS[yyp].yys] + yyErrCode
				if yyn >= 0 && yyn < yyLast {
					yystate = yyAct[yyn] /* simulate a shift of "error" */
					if yyChk[yystate] == yyErrCode {
						goto yystack
					}
				}

				/* the current p has no shift on "error", pop stack */
				if yyDebug >= 2 {
					__yyfmt__.Printf("error recovery pops state %d\n", yyS[yyp].yys)
				}
				yyp--
			}
			/* there is no state on the stack with an error shift ... abort */
			goto ret1

		case 3: /* no shift yet; clobber input char */
			if yyDebug >= 2 {
				__yyfmt__.Printf("error recovery discards %s\n", yyTokname(yytoken))
			}
			if yytoken == yyEofCode {
				goto ret1
			}
			yyrcvr.char = -1
			yytoken = -1
			goto yynewstate /* try again in the same state */
		}
	}

	/* reduction by production yyn */
	if yyDebug >= 2 {
		__yyfmt__.Printf("reduce %v in:\n\t%v\n", yyn, yyStatname(yystate))
	}

	yynt := yyn
	yypt := yyp
	_ = yypt // guard against "declared and not used"

	yyp -= yyR2[yyn]
	// yyp is now the index of $0. Perform the default action. Iff the
	// reduced production is ε, $1 is possibly out of range.
	if yyp+1 >= len(yyS) {
		nyys := make([]yySymType, len(yyS)*2)
		copy(nyys, yyS)
		yyS = nyys
	}
	yyVAL = yyS[yyp+1]

	/* consult goto table to find next state */
	yyn = yyR1[yyn]
	yyg := yyPgo[yyn]
	yyj := yyg + yyS[yyp].yys + 1

	if yyj >= yyLast {
		yystate = yyAct[yyg]
	} else {
		yystate = yyAct[yyj]
		if yyChk[yystate] != -yyn {
			yystate = yyAct[yyg]
		}
	}
	// dummy call; replaced with literal code
	switch yynt {

	case 1:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.yy:21
		{
			yyComposer = yyDollar[1].comp
			yyVAL.comp = yyComposer
		}
	case 2:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser.yy:25
		{
			yyComposer = makeCombiner(yyDollar[1].comp, yyDollar[3].comp)
			yyVAL.comp = yyComposer
		}
	case 3:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.yy:31
		{
			yyVAL.comp = yyDollar[1].comp
		}
	case 4:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.yy:34
		{
			yyVAL.comp = yyDollar[1].comp
		}
	case 5:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.yy:40
		{
			yyVAL.comp = yyDollar[1].comp
		}
	case 6:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser.yy:45
		{
			yyVAL.comp = makeAssign(yyDollar[1].str, yyDollar[2].str, yyDollar[3].comp)
		}
	case 7:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.yy:50
		{
			yyVAL.comp = makeStaticReader(yyDollar[1].str)
		}
	case 8:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.yy:53
		{
			yyVAL.comp = makeStaticReader(yyDollar[1].str)
		}
	case 9:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.yy:56
		{
			yyVAL.comp = makeVarReader(varLocal, yyDollar[1].str)
		}
	case 10:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.yy:59
		{
			yyVAL.comp = makeVarReader(varGlobal, yyDollar[1].str)
		}
	case 11:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.yy:62
		{
			yyVAL.comp = makeVarReader(varJson, yyDollar[1].str)
		}
	case 12:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.yy:65
		{
			yyVAL.comp = makeVarReader(varArgument, yyDollar[1].str)
		}
	case 13:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.yy:68
		{
			yyVAL.comp = makeCommand(yyDollar[1].str)
		}
	case 14:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser.yy:71
		{
			yyVAL.comp = yyDollar[2].comp
		}
	case 15:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.yy:77
		{
			yyVAL.comp = yyDollar[1].comp
		}
	case 16:
		yyDollar = yyS[yypt-2 : yypt+1]
//line parser.yy:80
		{
			yyVAL.comp = makePostUnary(yyDollar[1].comp, yyDollar[2].str)
		}
	case 17:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.yy:86
		{
			yyVAL.comp = yyDollar[1].comp
		}
	case 18:
		yyDollar = yyS[yypt-2 : yypt+1]
//line parser.yy:89
		{
			yyVAL.comp = makePreUnary(yyDollar[2].comp, yyDollar[1].str)
		}
	case 19:
		yyDollar = yyS[yypt-2 : yypt+1]
//line parser.yy:92
		{
			yyVAL.comp = makeNotUnary(yyDollar[2].comp)
		}
	case 20:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.yy:99
		{
			yyVAL.comp = yyDollar[1].comp
		}
	case 21:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser.yy:102
		{
			yyVAL.comp = makeCalc(yyDollar[1].comp, "*", yyDollar[3].comp)
		}
	case 22:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser.yy:105
		{
			yyVAL.comp = makeCalc(yyDollar[1].comp, "/", yyDollar[3].comp)
		}
	case 23:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser.yy:108
		{
			yyVAL.comp = makeCalc(yyDollar[1].comp, "%", yyDollar[3].comp)
		}
	case 24:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.yy:114
		{
			yyVAL.comp = yyDollar[1].comp
		}
	case 25:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser.yy:117
		{
			yyVAL.comp = makeCalc(yyDollar[1].comp, "+", yyDollar[3].comp)
		}
	case 26:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser.yy:120
		{
			yyVAL.comp = makeCalc(yyDollar[1].comp, "-", yyDollar[3].comp)
		}
	case 27:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.yy:127
		{
			yyVAL.comp = yyDollar[1].comp
		}
	case 28:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser.yy:130
		{
			yyVAL.comp = makeCalc(yyDollar[1].comp, yyDollar[2].str, yyDollar[3].comp)
		}
	case 29:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.yy:136
		{
			yyVAL.comp = yyDollar[1].comp
		}
	case 30:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser.yy:139
		{
			yyVAL.comp = makeCalc(yyDollar[1].comp, yyDollar[2].str, yyDollar[3].comp)
		}
	case 31:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.yy:146
		{
			yyVAL.comp = yyDollar[1].comp
		}
	case 32:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser.yy:149
		{
			yyVAL.comp = makeCalc(yyDollar[1].comp, yyDollar[2].str, yyDollar[3].comp)
		}
	case 33:
		yyDollar = yyS[yypt-1 : yypt+1]
//line parser.yy:155
		{
			yyVAL.comp = yyDollar[1].comp
		}
	case 34:
		yyDollar = yyS[yypt-3 : yypt+1]
//line parser.yy:158
		{
			yyVAL.comp = makeCalc(yyDollar[1].comp, yyDollar[2].str, yyDollar[3].comp)
		}
	}
	goto yystack /* stack new state and value */
}
