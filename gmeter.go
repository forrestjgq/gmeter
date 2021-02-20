package main

import (
	"flag"
	"fmt"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strings"

	"github.com/forrestjgq/gmeter/internal/meter"

	"github.com/forrestjgq/gmeter/config"
	"github.com/pkg/errors"

	"github.com/golang/glog"
)

func parseGlobalVariables(s string) (map[string]string, error) {
	if len(s) == 0 {
		return nil, nil
	}
	m := make(map[string]string)
	envs := strings.Split(s, " ")
	for _, c := range envs {
		kvs := strings.Split(c, "=")
		if len(kvs) != 2 {
			return nil, errors.Errorf("invalid variable definition: %s", c)
		}

		m[kvs[0]] = kvs[1]
	}
	return m, nil
}
func run() error {
	cfg := ""
	httpsrv := ""
	arceeCfg := ""
	call := ""
	template := ""
	variables := ""
	final := ""
	gmport := 0
	flag.StringVar(&variables, "e", "", "predefined global variables k=v, seperated by space if define multiple variables")
	flag.StringVar(&template, "t", "", "template config file path")
	flag.StringVar(&template, "template", "", "template config file path")
	flag.StringVar(&cfg, "config", "", "config file path, could be a .json, or .list, or a directory")
	flag.StringVar(&httpsrv, "httpsrv", "", "config file path for http server")
	flag.StringVar(&arceeCfg, "arcee", "", "arcee configuration file path")
	flag.StringVar(&call, "call", "", "extra program command line")
	flag.StringVar(&final, "f", "", "final execute config")
	flag.IntVar(&gmport, "gm", 7777, "gomark HTTP server, default 7777")
	flag.Parse()

	opt := &config.GOptions{
		Vars:           map[string]string{},
		Template:       template,
		Configs:        []string{},
		HTTPServerCfg:  httpsrv,
		ArceeServerCfg: arceeCfg,
		Call:           call,
		Final:          final,
		GoMarkPort:     gmport,
	}

	var err error
	if len(variables) > 0 {
		opt.Vars, err = parseGlobalVariables(variables)
		if err != nil {
			return errors.Wrapf(err, "parse global variables")
		}
	}

	if len(cfg) > 0 {
		opt.Configs = append(opt.Configs, cfg)
	} else {
		opt.Configs = flag.Args()
	}

	return meter.Execute(opt)
}
func main() {
	err := run()
	if err != nil {
		glog.Errorf("run failure, error: %+v", err)
		os.Exit(1)
	}

	if false {
		buf := make([]byte, 500*1024)
		n := runtime.Stack(buf, true)
		buf = buf[0:n]
		fmt.Print(string(buf))
	}
}
