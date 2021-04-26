package meter

import (
	"encoding/json"

	"github.com/pkg/errors"
)

// consumer should be a component that process response and failure
type consumer interface {
	processResponse(bg *background) next
	processFailure(bg *background, err error) next
}

// defaultConsumerType will continue test no matter what
type defaultConsumerType struct{}

func (d defaultConsumerType) processResponse(bg *background) next {
	return nextContinue
}

func (d defaultConsumerType) processFailure(_ *background, _ error) next {
	return nextContinue
}

var defaultConsumer = &defaultConsumerType{}

type failDecision int

const (
	abortOnFail failDecision = iota
	ignoreOnFail
)

type dynamicConsumer struct {
	// set to true if you need process failure as response
	check    composable
	success  composable
	fail     composable
	template jsonRule
	decision failDecision
}

func (d *dynamicConsumer) processResponse(bg *background) next {
	return d.process(bg, KeyResponse)
}
func (d *dynamicConsumer) process(bg *background, key string) next {
	if d.template != nil {
		if err := compareTemplate(d.template, bg, bg.getLocalEnv(key)); err != nil {
			return d.processFailure(bg, err)
		}
	}

	if d.check != nil {
		_, err := d.check.compose(bg)
		if err != nil {
			return d.processFailure(bg, err)
		}
	}

	d.processSuccess(bg)
	return nextContinue
}

func (d *dynamicConsumer) decideFailure(bg *background, err error) next {
	//if bg.inDebug() {
	//	glog.Errorf("failed: %+v", err)
	//} else {
	//	glog.Errorf("failed: %v", err)
	//}
	switch d.decision {
	case abortOnFail:
		return nextAbortPlan
	case ignoreOnFail:
		return nextContinue
	}
	return nextAbortAll
}
func (d *dynamicConsumer) processSuccess(bg *background) {
	if d.success != nil {
		_, _ = d.success.compose(bg)
	}
}
func (d *dynamicConsumer) processFailure(bg *background, err error) next {
	err = errors.Wrap(err, "process failure")
	// move error to failure if any to make sure fail processing without any error
	bg.setLocalEnv(KeyFailure, err.Error())
	bg.setError(nil)

	if d.fail != nil {
		_, _ = d.fail.compose(bg)
	}

	n := d.decideFailure(bg, err)
	bg.setError(err)
	return n
}

func makeDynamicConsumer(check, success, fail interface{}, template json.RawMessage, failAction failDecision) (*dynamicConsumer, error) {
	d := &dynamicConsumer{}
	d.decision = failAction

	var err error
	d.check, _, err = makeComposable(check)
	if err != nil {
		return nil, err
	}
	d.success, _, err = makeComposable(success, optIgnoreError())
	if err != nil {
		return nil, err
	}
	d.fail, _, err = makeComposable(fail, optIgnoreError())
	if err != nil {
		return nil, err
	}

	d.template, err = makeJsonTemplate(template)
	if err != nil {
		return nil, err
	}

	return d, nil
}
