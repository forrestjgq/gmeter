package gplugin

import (
	"plugin"

	"github.com/golang/glog"

	"github.com/pkg/errors"
)

var plugins = make(map[string]MessageReceiver)

type MessageReceiver interface {
	Name() string
	Recv(msg string) error
}

type ReceiverCreator func(param string) (MessageReceiver, error)

func Load(path, sym, param string) (string, error) {
	p, err := plugin.Open(path)
	if err != nil {
		return "", errors.Wrapf(err, "load %s", path)
	}

	s, err := p.Lookup(sym)
	if err != nil {
		return "", errors.Wrapf(err, "lookup %s from %s", sym, path)
	}

	f, ok := s.(func(string) (MessageReceiver, error))
	if !ok {
		return "", errors.Errorf("%s.%s has type of %T, not a ReceiverCreator", path, sym, s)
	}
	r, err := f(param)
	if err != nil {
		return "", errors.Wrapf(err, "create receiver for plugin %s", path)
	}
	name := r.Name()
	if _, ok := plugins[name]; ok {
		return "", errors.Wrapf(err, "duplciate load plugin %s", name)
	}

	plugins[name] = r
	glog.Infof("load plugin %s from %s", name, path)
	return name, nil
}

func Send(plugin, message string) error {
	if p, ok := plugins[plugin]; ok {
		return p.Recv(message)
	}
	return errors.Errorf("plugin %s not found", plugin)
}
