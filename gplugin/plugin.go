package gplugin

import (
	"plugin"

	"github.com/pkg/errors"
)

type MessageReceiver interface {
	Recv(msg string) error
}

type ReceiverCreator func(param string) (MessageReceiver, error)

func Load(path, sym, param string) (MessageReceiver, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return nil, errors.Wrapf(err, "load %s", path)
	}

	s, err := p.Lookup(sym)
	if err != nil {
		return nil, errors.Wrapf(err, "lookup %s from %s", sym, path)
	}

	f, ok := s.(func(string) (MessageReceiver, error))
	if !ok {
		return nil, errors.Errorf("%s.%s has type of %T, not a ReceiverCreator", path, sym, s)
	}
	return f(param)
}
