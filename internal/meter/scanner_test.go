package meter

import (
	"fmt"
	"testing"
)

func TestScanner(t *testing.T) {
	src := []string{
		"vara=1; vara == 2.0",
		"vara != 3 && !bool",
		"(vara >= 3 && !bool) || (hello > 3)",
		"(vara <= 3 && v < 222) || (hello == 'hello')",
		"hello != ''",
	}

	for _, v := range src {
		t.Logf("test: %s", v)
		s := &Scanner{}
		s.Init([]byte(v), func(pos int, msg string) {
			t.Fatalf("error: %s", msg)
		})
		for {
			pos, tok, lit := s.Scan()
			if tok == Eof {
				break
			}
			fmt.Printf("%d\t%s\t%q\n", pos, tok, lit)
		}
	}
}
