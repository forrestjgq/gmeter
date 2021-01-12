package config

import "encoding/json"

// RequestProcess defines how to process received request.
//
// HTTP request processing:
//     While HTTP server receives a request, $(URL) and $(REQUEST) will be
//     written with request URL and request body if any.
//     Template will be called for json comparing with HTTP request if it's
//     defined. If Template succeeds or it's not defined , Check will be called.
//     If any error is reported in Check processing, Check will be aborted.
//
// If any error is reported in  HTTP request processing, Failure will be called.
// Fail reason is recorded in $(FAILURE).
//
// If no error is reported in HTTP and HTTP response processing, Success will be called.
// Any error reported in Success will NOT trigger Failure.
//
// During processing, you should write response status code to $(STATUS), and if you
// need respond this request with a body, set $(RESPONSE) to the key of Route.Response.
// If $(STATUS) is empty, it will be default value 200. If $(RESPONSE) is empty, no
// response body will be written.
type RequestProcess struct {
	Check    []string        // [dynamic] segments called after server responds.
	Success  []string        // [dynamic] segments called if error is reported during http request and Check
	Failure  []string        // [dynamic] segments called if any error occurs.
	Template json.RawMessage // [dynamic] Template is a json compare template to compare with response.
}

// Route is an entity for HTTP server to process incoming request. gmeter will use Method
// and Path to guide request to route. When a request is received, its URL and request body
// will be written to $(URL) and $(REQUEST) if any. The following values will be written
// to local variables:
//   - headers defined in Headers, with environment variable of header name
//   - path variables, with key of path variable name and value of segment inside URL
//   - request parameters, with key of parameter key, value of parameter value(s)
type Route struct {
	// HTTP request method this route will process, default for "GET"
	Method string
	// [dynamic] router path definition, it could take path variables like:
	//     "/var/js/{script}
	// and by accessing `$(script)` you'll get the path segment value, for example,
	// if request path is:
	//     "/var/js/query"
	// now `$(script)` will get value `query`
	//
	// And you can also specify request parameters taking by request path, for example:
	//     "/var/js/{script}?name=hello
	// and by accessing `$(name)` you'll get `hello`. Multiple parameter with same name will
	// be joined together separated by a space:
	//     "/var/js/{script}?name=hello&name=world
	// and by accessing `$(name)` you'll get `hello world`.
	Path string
	// [dynamic] required headers definition, if its value is a raw string like "application/json", it
	// requires request takes this header with value of "application/json"; or if it is a dynamic
	// segment(with embedded commands) like "`assert $$ == hello`", instead of string comparing, it
	// will be called.
	Headers  map[string]string
	Request  *RequestProcess            // HTTP request processing
	Response map[string]json.RawMessage // [dynamic] multiple responses template identified by key of map
	Env      map[string]string          // predefined local variables
}

// HttpServer defines an HTTP server
type HttpServer struct {
	Address string            // ":0" or ":port" or "ip:port"
	Routes  []*Route          // HTTP server routers
	Report  Report            // Optional reporter, may used in router processing
	Env     map[string]string // predefined global variables
}

// HttpServers defines one or more HTTP servers
type HttpServers struct {
	// Servers represented by a name
	Servers map[string]*HttpServer
}
