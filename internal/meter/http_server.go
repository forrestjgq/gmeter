package meter

import (
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"github.com/forrestjgq/glog"
	"github.com/gorilla/mux"

	"github.com/forrestjgq/gmeter/config"
	"github.com/pkg/errors"
)

type httpsrv struct {
	cfg  *config.HttpServer
	port int
	l    net.Listener
	s    *http.Server
	r    *mux.Router
}

func (s *httpsrv) start(name string, cfg *config.HttpServer) error {
	s.cfg = cfg

	s.r = mux.NewRouter()
	bg := &background{
		name:   name,
		db:     createDB(),
		local:  makeSimpEnv(),
		global: makeSimpEnv(),
	}
	for k, v := range cfg.Env {
		bg.setGlobalEnv(k, v)
	}
	// report
	var err error
	if len(cfg.Report.Path) > 0 {
		cfg.Report.Path, err = loadFilePath(bg.getGlobalEnv(KeyTPath), cfg.Report.Path)
		if err != nil {
			return err
		}
	}
	bg.rpt, err = makeReporter(&cfg.Report)
	if err != nil {
		return err
	}

	for i, rc := range cfg.Routes {
		method := rc.Method
		if len(method) == 0 {
			method = "GET"
		}
		if len(rc.Path) == 0 {
			return errors.Errorf("HTTP server %s route[%d]: empty path", name, i)
		}
		f, err := makeRoute(bg, rc)
		if err != nil {
			return errors.Wrapf(err, "make route %d", i)
		}
		s.r.Methods(method).Path(rc.Path).Handler(f)
	}
	seg, err := makeSegments(cfg.Address)
	if err != nil {
		return err
	}

	addr, err := seg.compose(bg)
	if err != nil {
		return err
	}

	l, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.l = l
	s.port = l.Addr().(*net.TCPAddr).Port

	AddGlobalVariable("HTTP.PORT", strconv.Itoa(s.port))

	s.s = &http.Server{
		Handler: s.r,
	}
	go func() {
		_ = s.s.Serve(l)
		bg.globalClose()
	}()
	return nil
}

var servers = map[string]*httpsrv{}

// Start a test, path is the configure json file path, which must be able to be
// unmarshal to config.Config
func StartHTTPServer(path string) error {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return errors.Wrap(err, "read config file")
	}

	var s config.HttpServers
	err = json.Unmarshal(b, &s)
	if err != nil {
		return errors.Wrap(err, "unmarshal json")
	}

	path, err = filepath.Abs(path)
	if err != nil {
		return errors.Wrapf(err, "absolute path of %s", path)
	}
	tpath := filepath.Dir(path)
	for _, srv := range s.Servers {
		if srv.Env == nil {
			srv.Env = make(map[string]string)
		}
		srv.Env[KeyConfig] = path
		srv.Env[KeyTPath] = tpath
	}
	return StartHTTPServerConfig(&s)
}

func StartHTTPServerConfig(c *config.HttpServers) error {
	for k, v := range c.Servers {
		s := &httpsrv{}
		err := s.start(k, v)
		if err != nil {
			StopAll()
			return errors.Wrapf(err, "start server %s", k)
		}

		glog.Infof("Start HTTP server %s", k)
		servers[k] = s
	}

	time.Sleep(1 * time.Second)
	return nil
}

func StopAll() {
	for _, s := range servers {
		_ = s.s.Close()
	}
	servers = map[string]*httpsrv{}
}
