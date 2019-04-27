package cni

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/openvswitch/ovn-kubernetes/go-controller/pkg/config"
)

type Plugin struct{ socketPath string }

func NewCNIPlugin(socketPath string) *Plugin {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	if len(socketPath) == 0 {
		socketPath = serverSocketPath
	}
	return &Plugin{socketPath: socketPath}
}
func newCNIRequest(args *skel.CmdArgs) *Request {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	envMap := make(map[string]string)
	for _, item := range os.Environ() {
		idx := strings.Index(item, "=")
		if idx > 0 {
			envMap[strings.TrimSpace(item[:idx])] = item[idx+1:]
		}
	}
	return &Request{Env: envMap, Config: args.StdinData}
}
func (p *Plugin) doCNI(url string, req *Request) ([]byte, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal CNI request %v: %v", req, err)
	}
	client := &http.Client{Transport: &http.Transport{Dial: func(proto, addr string) (net.Conn, error) {
		var conn net.Conn
		if runtime.GOOS != "windows" {
			conn, err = net.Dial("unix", p.socketPath)
		} else {
			conn, err = net.Dial("tcp", serverTCPAddress)
		}
		return conn, err
	}}}
	resp, err := client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to send CNI request: %v", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read CNI result: %v", err)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("CNI request failed with status %v: '%s'", resp.StatusCode, string(body))
	}
	return body, nil
}
func (p *Plugin) CmdAdd(args *skel.CmdArgs) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	conf, err := config.ReadCNIConfig(args.StdinData)
	if err != nil {
		return fmt.Errorf("invalid stdin args")
	}
	req := newCNIRequest(args)
	body, err := p.doCNI("http://dummy/", req)
	if err != nil {
		return err
	}
	result, err := current.NewResult(body)
	if err != nil {
		return fmt.Errorf("failed to unmarshal response '%s': %v", string(body), err)
	}
	return types.PrintResult(result, conf.CNIVersion)
}
func (p *Plugin) CmdDel(args *skel.CmdArgs) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	_, err := p.doCNI("http://dummy/", newCNIRequest(args))
	return err
}
