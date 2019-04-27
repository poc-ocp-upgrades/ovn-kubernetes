package cluster

import (
	"fmt"
	"github.com/openvswitch/ovn-kubernetes/go-controller/pkg/util"
)

func getDefaultGatewayInterfaceDetails() (string, string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return "", "", fmt.Errorf("Not implemented yet on Windows")
}
func getIntfName(gatewayIntf string) (string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	intfName, err := util.GetNicName(gatewayIntf)
	if err != nil {
		return "", err
	}
	_, stderr, err := util.RunOVSVsctl("--if-exists", "get", "interface", intfName, "ofport")
	if err != nil {
		return "", fmt.Errorf("failed to get ofport of %s, stderr: %q, error: %v", intfName, stderr, err)
	}
	return intfName, nil
}
