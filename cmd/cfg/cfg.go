package main

import (
	"encoding/json"
	"os"

	"github.com/forrestjgq/gmeter/config"
)

func main() {
	req := &config.Request{
		Method: "POST",
		Path:   "/ai/detect/all",
		Headers: map[string]string{
			"content-type": "application/json",
		},
		Body: json.RawMessage(`
{
  "image": {
      "url": "/mnt/cephfs/vsec/vsecTestData/upload/sence.jpg",
      "url2": "/home/vse/depends/res/12imagesall/6_002696.jpg",
      "url3": "/home/vse/depends/res/12imagesall/1_005235.jpg",
      "url4": "/home/vse/depends/res/nonv.jpg"
  }
}
`),
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
				Host:    "vse",
				Request: "req",
				Response: &config.Response{Check: []string{
					"`assert $(STATUS) == 200`",
					"`json .Result.InnerStatus $(RESPONSE) | assert $(INPUT) == 200`",
					"`json -n .Result.Pedestrian $(RESPONSE) | assert $(INPUT) == 8`",
					"`json -n .Result.Faces $(RESPONSE) | assert $(INPUT) == 1`",
				}},
				Timeout: "10s",
			},
		},
		Mode: "",
		Schedules: []*config.Schedule{
			&config.Schedule{
				Name:        "recog-image",
				Tests:       "recognize",
				Count:       1000,
				Concurrency: 1,
				Env:         nil,
			},
		},
		Options: map[config.Option]string{
			config.OptionAbortIfFail: "true",
			//config.OptionDebug:       "true",
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
