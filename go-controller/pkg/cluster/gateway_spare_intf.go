package cluster

import (
	"fmt"
	"github.com/openvswitch/ovn-kubernetes/go-controller/pkg/util"
)

func initSpareGateway(nodeName string, clusterIPSubnet []string, subnet, gwNextHop, gwIntf string, nodeportEnable bool) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	ipAddress, err := getIPv4Address(gwIntf)
	if err != nil {
		return fmt.Errorf("Failed to get interface details for %s (%v)", gwIntf, err)
	}
	if ipAddress == "" {
		return fmt.Errorf("%s does not have a ipv4 address", gwIntf)
	}
	err = util.GatewayInit(clusterIPSubnet, nodeName, ipAddress, gwIntf, "", gwNextHop, subnet, nodeportEnable)
	if err != nil {
		return fmt.Errorf("failed to init spare interface gateway: %v", err)
	}
	return nil
}
