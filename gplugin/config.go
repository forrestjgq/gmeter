package gplugin

import (
	"encoding/json"
	"io/ioutil"

	"github.com/golang/glog"
	"github.com/pkg/errors"
)

type Plugin struct {
	Path   string
	Symbol string
	Param  json.RawMessage
}

func (p Plugin) load() error {
	name, err := Load(p.Path, p.Symbol, string(p.Param))
	if err != nil {
		return errors.Wrapf(err, "Load plugin %+v", p)
	}
	glog.Infof("Load plugin %s path %s", name, p.Path)
	return nil
}

type PluginConfig struct {
	Plugins []*Plugin
}

func LoadPlugins(cfgPath string) error {
	b, err := ioutil.ReadFile(cfgPath)
	if err != nil {
		return errors.Wrapf(err, "read file %s", cfgPath)
	}

	var plugins PluginConfig
	err = json.Unmarshal(b, &plugins)
	if err != nil {
		return errors.Wrapf(err, "unmarshal plugin config %s", cfgPath)
	}

	for _, p := range plugins.Plugins {
		err = p.load()
		if err != nil {
			return errors.Wrapf(err, "load plugin")
		}
	}
	return nil
}
