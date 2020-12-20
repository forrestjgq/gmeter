package meter

import "errors"

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
	success  []segments
	fail     []segments
	decision failDecision
}

func (d *dynamicConsumer) processResponse(bg *background) next {

	for _, s := range d.success {
		_, err := s.compose(bg)
		if err == nil {
			errstr := bg.getError()
			if len(errstr) > 0 {
				err = errors.New(errstr)
			}
		}

		// if error occurs, stops response processing
		if err != nil {
			return d.decideFailure(bg, err)
		}
	}
	return nextContinue
}

func (d *dynamicConsumer) decideFailure(bg *background, err error) next {
	switch d.decision {
	case abortOnFail:
		return nextAbortPlan
	case ignoreOnFail:
		return nextContinue
	}
	return nextAbortAll
}
func (d *dynamicConsumer) processFailure(bg *background, err error) next {
	bg.setError(err.Error())

	for _, s := range d.fail {
		_, _ = s.compose(bg)
	}
	return d.decideFailure(bg, err)
}

func makeDynamicConsumer(success, fail []string, failAction failDecision) (*dynamicConsumer, error) {
	d := &dynamicConsumer{}
	d.decision = failAction

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
