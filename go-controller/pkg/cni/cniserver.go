package cni

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/openvswitch/ovn-kubernetes/go-controller/pkg/config"
)

func NewCNIServer(rundir string) *Server {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	if len(rundir) == 0 {
		rundir = serverRunDir
	}
	router := mux.NewRouter()
	s := &Server{Server: http.Server{Handler: router}, rundir: rundir}
	router.NotFoundHandler = http.HandlerFunc(http.NotFound)
	router.HandleFunc("/", s.handleCNIRequest).Methods("POST")
	return s
}
func gatherCNIArgs(env map[string]string) (map[string]string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	cniArgs, ok := env["CNI_ARGS"]
	if !ok {
		return nil, fmt.Errorf("missing CNI_ARGS: '%s'", env)
	}
	mapArgs := make(map[string]string)
	for _, arg := range strings.Split(cniArgs, ";") {
		parts := strings.Split(arg, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid CNI_ARG '%s'", arg)
		}
		mapArgs[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return mapArgs, nil
}
func cniRequestToPodRequest(r *http.Request) (*PodRequest, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	var cr Request
	b, _ := ioutil.ReadAll(r.Body)
	if err := json.Unmarshal(b, &cr); err != nil {
		return nil, fmt.Errorf("JSON unmarshal error: %v", err)
	}
	cmd, ok := cr.Env["CNI_COMMAND"]
	if !ok {
		return nil, fmt.Errorf("unexpected or missing CNI_COMMAND")
	}
	req := &PodRequest{Command: command(cmd), Result: make(chan *PodResult)}
	req.SandboxID, ok = cr.Env["CNI_CONTAINERID"]
	if !ok {
		return nil, fmt.Errorf("missing CNI_CONTAINERID")
	}
	req.Netns, ok = cr.Env["CNI_NETNS"]
	if !ok {
		return nil, fmt.Errorf("missing CNI_NETNS")
	}
	req.IfName, ok = cr.Env["CNI_IFNAME"]
	if !ok {
		req.IfName = "eth0"
	}
	cniArgs, err := gatherCNIArgs(cr.Env)
	if err != nil {
		return nil, err
	}
	req.PodNamespace, ok = cniArgs["K8S_POD_NAMESPACE"]
	if !ok {
		return nil, fmt.Errorf("missing K8S_POD_NAMESPACE")
	}
	req.PodName, ok = cniArgs["K8S_POD_NAME"]
	if !ok {
		return nil, fmt.Errorf("missing K8S_POD_NAME")
	}
	conf, err := config.ReadCNIConfig(cr.Config)
	if err != nil {
		return nil, fmt.Errorf("broken stdin args")
	}
	req.CNIConf = conf
	return req, nil
}
func (s *Server) handleCNIRequest(w http.ResponseWriter, r *http.Request) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	req, err := cniRequestToPodRequest(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("%v", err), http.StatusBadRequest)
		return
	}
	logrus.Infof("Waiting for %s result for pod %s/%s", req.Command, req.PodNamespace, req.PodName)
	result, err := s.requestFunc(req)
	if err != nil {
		http.Error(w, fmt.Sprintf("%v", err), http.StatusBadRequest)
	} else {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write(result); err != nil {
			logrus.Warningf("Error writing %s HTTP response: %v", req.Command, err)
		}
	}
}
