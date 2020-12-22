package meter

import (
	"errors"

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
	check    []segments
	success  []segments
	fail     []segments
	decision failDecision
}

func (d *dynamicConsumer) processResponse(bg *background) next {

	for _, s := range d.check {
		_, err := s.compose(bg)
		if err == nil {
			errstr := bg.getError()
			if len(errstr) > 0 {
				err = errors.New(errstr)
			}
		}

		// if error occurs, stops response processing
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
	for _, s := range d.success {
		_, _ = s.compose(bg)
	}
}
func (d *dynamicConsumer) processFailure(bg *background, err error) next {
	bg.setError(err.Error())
	for _, s := range d.fail {
		_, _ = s.compose(bg)
	}
	return d.decideFailure(bg, err)
}

func makeDynamicConsumer(check, success, fail []string, failAction failDecision) (*dynamicConsumer, error) {
	d := &dynamicConsumer{}
	d.decision = failAction

	for _, c := range check {
		if len(c) == 0 {
			continue
		}
		seg, err := makeSegments(c)
		if err != nil {
			return nil, err
		}

		d.check = append(d.check, seg)
	}
	for _, c := range success {
		if len(c) == 0 {
			continue
		}
		seg, err := makeSegments(c)
		if err != nil {
			return nil, err
		}

		d.success = append(d.success, seg)
	}
	for _, c := range fail {
		if len(c) == 0 {
			continue
		}
		seg, err := makeSegments(c)
		if err != nil {
			return nil, err
		}

		d.fail = append(d.fail, seg)
	}
	return d, nil
}
