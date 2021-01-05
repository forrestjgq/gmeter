package meter

import (
	"errors"
	"strings"
	"testing"
)

func TestDefaultConsumer(t *testing.T) {
	res := defaultConsumer.processResponse(nil)
	if res != nextContinue {
		t.Fatalf("fail")
	}
	res = defaultConsumer.processFailure(nil, errors.New("unknown"))
	if res != nextContinue {
		t.Fatalf("fail")
	}
}

func TestDynamicConsumerSuccess(t *testing.T) {
	bg, _ := createDefaultBackground()

	c, err := makeDynamicConsumer(
		[]string{ // check
			"`envw -c check RESULT`",
		},
		[]string{ // success
			"`echo $(RESULT) success | envw RESULT`",
		},
		[]string{ // fail
			"`echo $(RESULT) fail | envw RESULT`",
		},
		[]byte(""), // template
		abortOnFail,
	)
	if err != nil {
		t.Fatalf(err.Error())
	}

	res := c.processResponse(bg)
	if res != nextContinue {
		t.Fatalf("res %d", res)
	}

	s := bg.getLocalEnv("RESULT")
	if s != "check success" {
		t.Fatalf("unexpected %s", s)
	}
}
func TestDynamicConsumerCheckFail(t *testing.T) {
	bg, _ := createDefaultBackground()

	c, err := makeDynamicConsumer(
		[]string{ // check
			"`envw -c check RESULT`",
			"`assert 0 != 0`",
		},
		[]string{ // success
			"`echo $(RESULT) success | envw RESULT`",
		},
		[]string{ // fail
			"`echo $(RESULT) fail | envw RESULT`",
		},
		[]byte(""), // template
		abortOnFail,
	)
	if err != nil {
		t.Fatalf(err.Error())
	}

	res := c.processResponse(bg)
	if res != nextAbortPlan {
		t.Fatalf("res %d", res)
	}

	s := bg.getLocalEnv("RESULT")
	if s != "check fail" {
		t.Fatalf("unexpected %s", s)
	}
}
func TestDynamicConsumerFail(t *testing.T) {
	bg, _ := createDefaultBackground()

	c, err := makeDynamicConsumer(
		[]string{ // check
			"`envw -c check RESULT`",
			"`assert 0 != 0`",
		},
		[]string{ // success
			"`echo $(RESULT) success | envw RESULT`",
		},
		[]string{ // fail
			"`envw -c fail RESULT`",
		},
		[]byte(""), // template
		abortOnFail,
	)
	if err != nil {
		t.Fatalf(err.Error())
	}

	res := c.processResponse(bg)
	if res != nextAbortPlan {
		t.Fatalf("res %d", res)
	}

	s := bg.getLocalEnv("RESULT")
	if s != "fail" {
		t.Fatalf("unexpected %s", s)
	}
}
func TestDynamicConsumerTemplate(t *testing.T) {
	bg, _ := createDefaultBackground()

	c, err := makeDynamicConsumer(
		[]string{ // check
			"`echo $(RESULT) check | envw RESULT`",
		},
		[]string{ // success
			"`echo $(RESULT) success | envw RESULT`",
		},
		[]string{ // fail
			"`echo $(RESULT) fail | envw RESULT`",
		},
		[]byte("{ \"seq\": \"`assert $ > 1`\" }"), // template
		ignoreOnFail,
	)
	if err != nil {
		t.Fatalf(err.Error())
	}

	bg.setLocalEnv(KeyResponse, `{"seq": 1}`)
	res := c.processResponse(bg)
	if res != nextContinue {
		t.Fatalf("res %d", res)
	}

	s := bg.getLocalEnv("RESULT")
	s = strings.TrimSpace(s)
	if s != "fail" { // template first, then check, then success, any error will redirect to fail
		t.Fatalf("unexpected %s", s)
	}
}
func TestDynamicConsumerTemplateNull(t *testing.T) {
	bg, _ := createDefaultBackground()

	c, err := makeDynamicConsumer(
		[]string{ // check
			"`echo $(RESULT) check | envw RESULT`",
			"`assert 0 != 0`",
		},
		[]string{ // success
			"`echo $(RESULT) success | envw RESULT`",
		},
		[]string{ // fail
			"`echo $(RESULT) fail | envw RESULT`",
		},
		[]byte("null"), // template
		ignoreOnFail,
	)
	if err != nil {
		t.Fatalf(err.Error())
	}

	bg.setLocalEnv(KeyResponse, `{"seq": 1}`)
	res := c.processResponse(bg)
	if res != nextContinue {
		t.Fatalf("res %d", res)
	}

	s := bg.getLocalEnv("RESULT")
	if s != " check fail" { // template first, then check, then success, any error will redirect to fail
		t.Fatalf("unexpected %s", s)
	}
}
