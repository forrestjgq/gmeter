package arcee

import (
	"encoding/json"
	fmt "fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"

	"github.com/forrestjgq/gmeter/config"

	"github.com/pkg/errors"
	context "golang.org/x/net/context"

	"github.com/golang/glog"
	uuid "github.com/satori/go.uuid"
	"google.golang.org/grpc"
)

type arceeServer struct {
	server *grpc.Server
	lis    net.Listener
	path   string
}

func (a *arceeServer) GetFileContent(context context.Context, message *WeedFileArrayMessage) (*WeedFileMapMessage, error) {
	rsp := &WeedFileMapMessage{
		MapData: make(map[string][]byte),
	}
	for _, name := range message.ArrayData {
		path := a.path + "/" + name
		b, err := ioutil.ReadFile(path)
		if err != nil {
			rsp.MapData[name] = nil
		} else {
			rsp.MapData[name] = b
		}
	}
	return rsp, nil
}

func (a *arceeServer) PostFileContent(context context.Context, message *WeedFileMapMessage) (*WeedFileMapId, error) {
	rsp := &WeedFileMapId{MapIds: make(map[string]string)}
	for k, v := range message.MapData {
		id := uuid.NewV4().String()
		err := ioutil.WriteFile(a.path+"/"+id, v, os.ModePerm)
		if err != nil {
			rsp.MapIds[k] = ""
		} else {
			rsp.MapIds[k] = id
		}
	}
	return rsp, nil
}

func (a *arceeServer) DeleteFile(context context.Context, message *WeedFileArrayMessage) (*WeedFileArrayMessage, error) {
	rsp := &WeedFileArrayMessage{}
	for _, name := range message.ArrayData {
		path := a.path + "/" + name
		err := os.Remove(path)
		if err != nil {
			glog.Errorf("remove %s fail: %v", path, err)
		} else {
			rsp.ArrayData = append(rsp.ArrayData, name)
		}
	}
	return rsp, nil
}

var arcee *arceeServer

func StartArceeConfig(cfg *config.Arcee) (int, error) {
	path := cfg.Path
	port := cfg.Port

	absPath, err := filepath.Abs(path)
	if err != nil {
		return 0, errors.Wrapf(err, "get absolute path %s", path)
	}
	arcee = &arceeServer{path: absPath}
	if port < 0 {
		return 0, errors.Errorf("Invalid arcee server port: %d", port)
	}

	// make directory
	f, err := os.Stat(path)
	if err == nil {
		// exist, check dir
		if !f.IsDir() {
			return 0, errors.Errorf("%s is not a directory", path)
		}
	} else if os.IsNotExist(err) {
		err = os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return 0, errors.Wrapf(err, "mkdir %s", path)
		}
	} else {
		return 0, errors.Wrapf(err, "%s stat", path)
	}

	arcee.server = grpc.NewServer()
	RegisterGrpcServiceServer(arcee.server, arcee)
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return 0, errors.Wrapf(err, "failed to listen")
	}

	port = lis.Addr().(*net.TCPAddr).Port
	arcee.lis = lis

	go func() {
		glog.Errorf("Arcee server started on port %d", port)
		e := arcee.server.Serve(lis)
		if e != nil {
			glog.Error("gprc serve err: ", e)
		}
	}()

	return port, nil
}
func StartArcee(path string) (int, error) {

	b, err := ioutil.ReadFile(path)
	if err != nil {
		return 0, errors.Wrap(err, "read config file")
	}

	var s config.Arcee
	err = json.Unmarshal(b, &s)
	if err != nil {
		return 0, errors.Wrap(err, "unmarshal json")
	}

	return StartArceeConfig(&s)
}
