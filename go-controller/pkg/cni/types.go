package cni

import (
	"net/http"
	"github.com/containernetworking/cni/pkg/types"
)

const serverRunDir string = "/var/run/ovn-kubernetes/cni/"
const serverSocketName string = "ovn-cni-server.sock"
const serverSocketPath string = serverRunDir + "/" + serverSocketName
const serverTCPAddress string = "127.0.0.1:3996"

type command string

const CNIAdd command = "ADD"
const CNIUpdate command = "UPDATE"
const CNIDel command = "DEL"

type Request struct {
	Env	map[string]string	`json:"env,omitempty"`
	Config	[]byte			`json:"config,omitempty"`
}
type PodRequest struct {
	Command		command
	PodNamespace	string
	PodName		string
	SandboxID	string
	Netns		string
	IfName		string
	CNIConf		*types.NetConf
	Result		chan *PodResult
}
type PodResult struct {
	Response	[]byte
	Err		error
}
type cniRequestFunc func(request *PodRequest) ([]byte, error)
type Server struct {
	http.Server
	requestFunc	cniRequestFunc
	rundir		string
}
