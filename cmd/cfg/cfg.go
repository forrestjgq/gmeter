package main

import (
	"encoding/json"
	"os"

	"github.com/forrestjgq/gmeter/config"
)

var reqstr = "{ \"image\": { \"url\": \"$(IMAGE)\" }, \"roi\": { \"X\": \"`cvt -i $(X)`\", \"Y\": \"`cvt -i $(Y)`\", \"W\": \"`cvt -i $(W)`\", \"H\": \"`cvt -i $(H)`\"}}"

func main() {
	req := &config.Request{
		Method: "POST",
		Path:   "/debug/detect/face",
		Headers: map[string]string{
			"content-type": "application/json",
		},
		Body: json.RawMessage(reqstr),
	}
	cfg := config.Config{
		Name: "sample",
		Hosts: map[string]*config.Host{
			"vse": &config.Host{
				Host:  "http://127.0.0.1:8009",
				Proxy: "",
			},
		},
		Messages: map[string]*config.Request{
			"req": req,
		},
		Tests: map[string]*config.Test{
			"recognize": &config.Test{
				PreProcess: []string{
					"`list /home/forrest/project/gmeter/img.list | envw JSON`",
					"`json .image $(JSON) | envw IMAGE`",
					"`json .x $(JSON) | envw X`",
					"`json .y $(JSON) | envw Y`",
					"`json .w $(JSON) | envw W`",
					"`json .h $(JSON) | envw H`",
				},
				Host:    "vse",
				Request: "req",
				Response: &config.Response{
					Success: []string{
						"`report`",
					},
					Failure: []string{
						//"`assert $(STATUS) == 200`",
						//"`json .Result.InnerStatus $(RESPONSE) | assert $(INPUT) == 200`",
						//"`json -n .Result.Pedestrian $(RESPONSE) | assert $(INPUT) == 8`",
						//"`json -n .Result.Faces $(RESPONSE) | assert $(INPUT) == 1`",
						"`report`",
					},
				},
				Timeout: "10s",
			},
		},
		Mode: "",
		Schedules: []*config.Schedule{
			&config.Schedule{
				Name:        "recog-image",
				PreProcess:  []string{},
				Tests:       "recognize",
				Count:       0,
				Concurrency: 1,
				Env:         nil,
				Reporter: config.Report{
					Path: "report.log",
					//Format: "`json .Result.Faces.[0].Features $(RESPONSE)`\n",
					Format: "{ \"Error\": \"$(ERROR)\", \"Status\": $(STATUS), \"Response\": $(RESPONSE) }\n",
				},
			},
		},
		Options: map[config.Option]string{
			config.OptionAbortIfFail: "true",
			config.OptionDebug:       "true",
		},
	}

	b, e := json.MarshalIndent(cfg, "", "    ")
	if e != nil {
		panic(e)
	}

	f, e := os.Create("./sample.json")
	if e != nil {
		panic(e)
	}

	_, _ = f.Write(b)
	_ = f.Close()
}
