package main

import (
	"flag"

	"github.com/forrestjgq/gmeter/gplugin"
	"github.com/golang/glog"
)

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		glog.Fatalf("invalid so: %s", args)
	}

	so := args[0]
	plugin, err := gplugin.Load(so, "Load", "")
	if err != nil {
		glog.Fatalf("load fail: %v", err)
	}
	_ = gplugin.Send(plugin, "hello world")
}
