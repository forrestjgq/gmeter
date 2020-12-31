package config

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
)

// Request defines parameters to generate an HTTP request.
// Note that any part of any member can contain embedded commands.
//
// Note that Body is accepted only when it's empty, or it's a valid json.
type Request struct {
	Method  string            // default to be GET, could be GET/POST/PUT/DELETE
	Path    string            // [dynamic] /path/to/target, parameter is supported like /path?param1=1&param2=hello...
	Headers map[string]string // extra headers like "Content-Type: application/json"
	Body    json.RawMessage   // [dynamic] Json body to send, or "" if no body is required.
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

// Response defines how to process successful request and failed request.
//
// HTTP Response processing:
//     While HTTP server responds, even with non-2xx status code, Template will
//     be called for json comparing with HTTP response if it's defined.
//     If Template succeeds or it's not defined , Check will be called.
//     If any error is reported in Check processing, Check will be aborted.
//
// If any error is reported in HTTP and HTTP response processing, Failure will be called.
// Fail reason is recorded in $(ERROR). Note that $(URL), $(REQUEST), $(STATUS)
// and $(RESPONSE) may be empty. Any other variables generated before HTTP sending
// (if any) may also be empty.
//
// If no error is reported in HTTP and HTTP response processing, Success will be called.
// Any error reported in Success will NOT trigger Failure.
//
type Response struct {
	Check    []string        // [dynamic] segments called after server responds.
	Success  []string        // [dynamic] segments called if error is reported during http request and Check
	Failure  []string        // [dynamic] segments called if any error occurs.
	Template json.RawMessage // [dynamic] Template is a json compare template to compare with response.
}
