%{
package meter

import "fmt"

%}
%union {
 str string
 tok Token
 err error
 comp composer
}

%token <str> IDENTITY LITERAL
%token <str> UNARY_ARITH_OP COMP_OP EQUAL_OP LOGIC_OP AND_OP OR_OP ASSIGN_OP
%token <str> V_LOCAL V_GLOBAL V_JSON CMD_EXEC
%type <comp> primary postfix unary multiplicative additive relational equality logical_and logical_or expression assign stats stat

%%
stats:
    stat {
        yyComposer = $1
        $$ = yyComposer
    }
    | stats ';' stat {
        yyComposer = makeCombiner($1, $3)
        $$ = yyComposer
    }
    ;
stat:
    assign {
        $$ = $1
    }
    | expression {
        $$ = $1
    }
    ;

expression:
	logical_or {
	    if yyDebug > 5 {
            fmt.Println(" RULE: expression -> logical_or")
	    }
        $$ = $1
	}
	;
assign:
    IDENTITY ASSIGN_OP logical_or  {
        $$ = makeAssign($1, $2, $3)
    }
    ;
primary:
	LITERAL {
	    if yyDebug > 5 {
	        fmt.Println(" RULE: primary -> LITERAL")
	    }
	    $$ = makeStaticReader($1)
	}
    | V_LOCAL {
        $$ = makeVarReader(varLocal, $1)
    }
    | V_GLOBAL {
        $$ = makeVarReader(varGlobal, $1)
    }
    | V_JSON {
        $$ = makeVarReader(varJson, $1)
    }
    | CMD_EXEC {
        $$ = makeCommand($1)
    }
	| '(' expression ')'{
        $$ = $2
	}
	;

postfix:
	primary {
	    if yyDebug > 5 {
	        fmt.Println(" RULE: postfix -> primary")
	    }
        $$ = $1
	}
	| 	postfix UNARY_ARITH_OP {
	    $$ = makePostUnary($1, $2)
	}
	;

unary:
	postfix{
	    if yyDebug > 5 {
	        fmt.Println(" RULE: unary -> postfix")
	    }
        $$ = $1
	}
	| 	UNARY_ARITH_OP unary{
        $$ = makePreUnary($2, $1)
	}
	| 	'!' unary{
        $$ = makeNotUnary($2)
	}
	;


multiplicative:
	unary {
	    if yyDebug > 5 {
            fmt.Println(" RULE: multiplicative -> unary")
	    }
        $$ = $1
	}
	| multiplicative '*' unary {
        $$ = makeCalc($1, "*", $3)
	}
	| multiplicative '/' unary {
        $$ = makeCalc($1, "/", $3)
	}
	| multiplicative '%' unary {
        $$ = makeCalc($1, "%", $3)
	}
	;

additive:
	multiplicative  {
	    if yyDebug > 5 {
            fmt.Println(" RULE: additive -> multiplicative")
	    }
        $$ = $1
	}
	| additive '+' multiplicative {
        $$ = makeCalc($1, "+", $3)

	}
	| additive '-' multiplicative {
        $$ = makeCalc($1, "-", $3)

	}
	;


relational:
	additive {
	    if yyDebug > 5 {
            fmt.Println(" RULE: relational -> additive")
	    }
        $$ = $1

	}
	| relational COMP_OP additive {
        $$ = makeCalc($1, $2, $3)

	}
	;

equality:
	relational {
	    if yyDebug > 5 {
            fmt.Println(" RULE: equality -> relational")
	    }
        $$ = $1

	}
	| equality EQUAL_OP relational {
	    if yyDebug > 5 {
            fmt.Println(" RULE: equality EQUAL_OP relational ")
	    }
        $$ = makeCalc($1, $2, $3)

	}
	;


logical_and:
	equality {
	    if yyDebug > 5 {
            fmt.Println(" RULE: logical_and -> equality")
	    }
        $$ = $1

	}
	| logical_and AND_OP equality {
        $$ = makeCalc($1, $2, $3)

	}
	;

logical_or:
	logical_and {
	    if yyDebug > 5 {
            fmt.Println(" RULE: logical_or -> logical_and")
	    }
        $$ = $1

	}
	| logical_or OR_OP logical_and {
        $$ = makeCalc($1, $2, $3)
	}
	;

%%
