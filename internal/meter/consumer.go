package meter

import (
	"encoding/json"

	"github.com/golang/glog"
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
	check    *group
	success  *group
	fail     *group
	template jsonRule
	decision failDecision
}

func (d *dynamicConsumer) processResponse(bg *background) next {
	if d.template != nil {
		if err := compareTemplate(d.template, bg, bg.getLocalEnv(KeyResponse)); err != nil {
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
	glog.Errorf("%s|%s|%s failed: %v",
		bg.getGlobalEnv(KeyConfig), bg.getGlobalEnv(KeySchedule), bg.getLocalEnv(KeyTest), err)
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
	// move error to failure if any to make sure fail processing without any error
	bg.setLocalEnv(KeyFailure, err.Error())
	bg.setError("")

	if d.fail != nil {
		_, _ = d.fail.compose(bg)
	}

	return d.decideFailure(bg, err)
}

func makeDynamicConsumer(check, success, fail []string, template json.RawMessage, failAction failDecision) (*dynamicConsumer, error) {
	d := &dynamicConsumer{}
	d.decision = failAction

	var err error
	if len(check) > 0 {
		d.check, err = makeGroup(check, false)
		if err != nil {
			return nil, err
		}
	}
	if len(success) > 0 {
		d.success, err = makeGroup(success, true)
		if err != nil {
			return nil, err
		}
	}
	if len(fail) > 0 {
		d.fail, err = makeGroup(fail, true)
		if err != nil {
			return nil, err
		}
	}

	if len(template) > 0 {
		d.template, err = makeJsonTemplate(template)
		if err != nil {
			return nil, err
		}
	}
	return d, nil
}
