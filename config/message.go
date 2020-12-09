package gmeter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
)

type Request struct {
	Method  string // default to be GET
	Path    string // /path/to/target
	Headers map[string]string
	Body    json.RawMessage
}

func (m *Request) compile(input *Request) error {
	return nil
}
func (m *Request) check() error {
	if matched, matchErr := regexp.Match("^(/[^/]+)*$", []byte(m.Path)); matchErr != nil {
		panic("message match regexp invalid")
	} else if !matched {
		return fmt.Errorf("invalid path: %s", m.Path)
	}

	switch m.Method {
	case http.MethodGet:
		if m.Body != nil {
			return fmt.Errorf("message %s GET with message body", m.Path)
		}
	case http.MethodPut:
	case http.MethodDelete:
		if m.Body != nil {
			return fmt.Errorf("message %s DELETE with message body", m.Path)
		}
	case http.MethodPost:
	case http.MethodPatch:
	default:
		return fmt.Errorf("invalid method: %s for host %s", m.Method, m.Path)
	}

	if m.Headers == nil {
		m.Headers = make(map[string]string)
	}
	return nil
}

type Response struct {
	// Examples:
	// Response body could be:
	// {
	//    "a": {
	//        "b": "b-value",
	//        "c": 10
	//    },
	//    "d": [
	//        {
	//            "e": false,
	//            "f": 10,
	//        }, {
	//            "e": true,
	//            "f": 0,
	//        }
	//    ]
	// }
	//
	// $(Status) == 200, $(Status) != 400, $(Status) < 300, here ${Status} is current status code
	// .a.b == "b-value", compare field, here returns true
	// .d[] > 0, xx[] indicates that xx is an array and xx[] is number of this array
	// select .d[?].e == false | .f > 1, select from ".d" array where ".e" is "false", then pass element to next pipe
	//        which is ".f > 1" to judge value
	// select .d[*] | .
	Check []string
}
