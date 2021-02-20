package meter

import (
	"testing"

	"github.com/forrestjgq/gmeter/config"
)

func TestExecute(t *testing.T) {

	opt := &config.GOptions{
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
func TestExecuteList(t *testing.T) {

	opt := &config.GOptions{
		Template: "test/base/base.json",
		Configs: []string{
			"test/base/ping.json",
			"test/client",
			"test/client/all.list",
		},
		HTTPServerCfg:  "test/server/server.json",
		ArceeServerCfg: "",
		Call:           "",
		Final:          "test/base/ping.json",
	}

	err := Execute(opt)
	if err != nil {
		t.Fatalf(err.Error())
	}
}
func TestExecuteImports(t *testing.T) {

	opt := &config.GOptions{
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
