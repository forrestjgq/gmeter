package config

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

func (m *Request) Check() error {
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
	Success  []string
	Failure  []string
	Template json.RawMessage
}
