package cluster

import (
	"fmt"
	"syscall"
	"github.com/openvswitch/ovn-kubernetes/go-controller/pkg/util"
	"github.com/vishvananda/netlink"
)

func getDefaultGatewayInterfaceDetails() (string, string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	routes, err := netlink.RouteList(nil, syscall.AF_INET)
	if err != nil {
		return "", "", fmt.Errorf("Failed to get routing table in node")
	}
	for i := range routes {
		route := routes[i]
		if route.Dst == nil && route.Gw != nil && route.LinkIndex > 0 {
			intfLink, err := netlink.LinkByIndex(route.LinkIndex)
			if err != nil {
				continue
			}
			intfName := intfLink.Attrs().Name
			if intfName != "" {
				return intfName, route.Gw.String(), nil
			}
		}
	}
	return "", "", fmt.Errorf("Failed to get default gateway interface")
}
func getIntfName(gatewayIntf string) (string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	intfName := util.GetNicName(gatewayIntf)
	_, stderr, err := util.RunOVSVsctl("--if-exists", "get", "interface", intfName, "ofport")
	if err != nil {
		return "", fmt.Errorf("failed to get ofport of %s, stderr: %q, error: %v", intfName, stderr, err)
	}
	return intfName, nil
}
