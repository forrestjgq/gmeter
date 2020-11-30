package gmeter

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
)

type Message struct {
	Name    string
	Path    string // /path/to/target
	Method  string
	Headers map[string]string
	Params  map[string]string // Path?key=value&key=value..
	Body    json.RawMessage
}

func (m *Message) compile(input *Message) error {
	return nil
}
func (m *Message) check() error {
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
	if m.Params == nil {
		m.Params = make(map[string]string)
	}
	return nil
}
