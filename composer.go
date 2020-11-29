package gmeter

import (
	"encoding/json"
)

type request struct {
	url     string
	method  string
	headers map[string]string
	body    string
}
type response struct {
	status int
	body   json.RawMessage
}

func compose(config *Config, prevReq *request, prevRsp *response, t *Test) *request {
	url := t.host
	msg := t.reqMsg
	url += msg.Path

	if len(msg.Params) > 0 {
		url += "?"
		for k, v := range msg.Params {
			if v != "" {
				url += k + "=" + v
			} else {
				url += k
			}
		}
	}

	// todo: compile url

	req := &request{
		url:     url,
		method:  msg.Method,
		headers: msg.Headers,
		body:    string(msg.Body),
	}

	if len(req.body) > 0 {
		if _, ok := req.headers["Content-Type"]; !ok {
			req.headers["Content-Type"] = "application/json"
		}
	}

	return req
}
