package meter

import "io"

// consumer should be a component that process response and failure
type consumer interface {
	processResponse(bg *background, status int, body io.Reader) next
	processFailure(bg *background, err error) next
}

// defaultConsumerType will continue test no matter what
type defaultConsumerType struct{}

func (d defaultConsumerType) processResponse(bg *background, status int, body io.Reader) next {
	return nextContinue
}

func (d defaultConsumerType) processFailure(_ *background, _ error) next {
	return nextContinue
}

var defaultConsumer = &defaultConsumerType{}
