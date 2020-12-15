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
	segs     []segments
	decision failDecision
}

func (d *dynamicConsumer) processResponse(bg *background) next {
	bg.setError("")

	for _, s := range d.segs {
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
	return nextContinue
}

func (d *dynamicConsumer) processFailure(bg *background, err error) next {
	switch d.decision {
	case abortOnFail:
		return nextAbortPlan
	case ignoreOnFail:
		return nextContinue
	}
	return nextAbortAll
}

func makeDynamicConsumer(check []string, failAction failDecision) (*dynamicConsumer, error) {
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

		d.segs = append(d.segs, seg)
	}
	return d, nil
}
