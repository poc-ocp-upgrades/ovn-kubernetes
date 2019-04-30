package util

import (
	"fmt"
	"strings"
	"github.com/sirupsen/logrus"
)

func RemoveNode(nodeName string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	clusterRouter, err := GetK8sClusterRouter()
	if err != nil {
		return fmt.Errorf("failed to get cluster router")
	}
	_, stderr, err := RunOVNNbctl("--if-exist", "ls-del", nodeName)
	if err != nil {
		return fmt.Errorf("Failed to delete logical switch %s, "+"stderr: %q, error: %v", nodeName, stderr, err)
	}
	gatewayRouter := fmt.Sprintf("GR_%s", nodeName)
	var routerIP string
	routerIPNetwork, stderr, err := RunOVNNbctl("--if-exist", "get", "logical_router_port", "rtoj-"+gatewayRouter, "networks")
	if err != nil {
		return fmt.Errorf("Failed to get logical router port, stderr: %q, "+"error: %v", stderr, err)
	}
	if routerIPNetwork != "" {
		routerIPNetwork = strings.Trim(routerIPNetwork, "[]\"")
		if routerIPNetwork != "" {
			routerIP = strings.Split(routerIPNetwork, "/")[0]
		}
	}
	if routerIP != "" {
		var uuids string
		uuids, stderr, err = RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "logical_router_static_route", "nexthop="+routerIP)
		if err != nil {
			return fmt.Errorf("Failed to fetch all routes with gateway "+"router %s as nexthop, stderr: %q, "+"error: %v", gatewayRouter, stderr, err)
		}
		routes := strings.Fields(uuids)
		for _, route := range routes {
			_, stderr, err = RunOVNNbctl("--if-exists", "remove", "logical_router", clusterRouter, "static_routes", route)
			if err != nil {
				logrus.Errorf("Failed to delete static route %s"+", stderr: %q, err = %v", route, stderr, err)
				continue
			}
		}
	}
	_, stderr, err = RunOVNNbctl("--if-exist", "lsp-del", "jtor-"+gatewayRouter)
	if err != nil {
		return fmt.Errorf("Failed to delete logical switch port jtor-%s, "+"stderr: %q, error: %v", gatewayRouter, stderr, err)
	}
	_, stderr, err = RunOVNNbctl("--if-exist", "lrp-del", "rtos-"+nodeName)
	if err != nil {
		return fmt.Errorf("Failed to delete logical router port rtos-%s, "+"stderr: %q, error: %v", nodeName, stderr, err)
	}
	_, stderr, err = RunOVNNbctl("--if-exist", "lr-del", gatewayRouter)
	if err != nil {
		return fmt.Errorf("Failed to delete gateway router %s, stderr: %q, "+"error: %v", gatewayRouter, stderr, err)
	}
	externalSwitch := "ext_" + nodeName
	_, stderr, err = RunOVNNbctl("--if-exist", "ls-del", externalSwitch)
	if err != nil {
		return fmt.Errorf("Failed to delete external switch %s, stderr: %q, "+"error: %v", externalSwitch, stderr, err)
	}
	return nil
}
