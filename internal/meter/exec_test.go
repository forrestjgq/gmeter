package meter

import (
	"testing"

	"github.com/forrestjgq/gmeter/config"
)

func TestExecute(t *testing.T) {

	opt := &config.GOptions{
		Vars: map[string]string{
			"IP":   "127.0.0.1",
			"PORT": "18009",
		},
		Template: "test/base/base.json",
		Configs: []string{
			"test/base/ping.json",
			"test/client",
		},
		HTTPServerCfg:  "test/server/server.json",
		ArceeServerCfg: "",
		Call:           "",
		Final:          "",
	}

	err := Execute(opt)
	if err != nil {
		t.Fatalf(err.Error())
	}
}
func TestExecuteImports(t *testing.T) {

	opt := &config.GOptions{
		Vars: map[string]string{
			"IP":   "127.0.0.1",
			"PORT": "18009",
		},
		Configs: []string{
			"test/standalone/import.json",
		},
		HTTPServerCfg:  "test/server/server.json",
		ArceeServerCfg: "",
		Call:           "",
		Final:          "",
	}

	err := Execute(opt)
	if err != nil {
		t.Fatalf(err.Error())
	}
}
