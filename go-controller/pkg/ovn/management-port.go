package ovn

import (
	"fmt"
	"net"
	"runtime"
	"strconv"
	"strings"
	"github.com/openvswitch/ovn-kubernetes/go-controller/pkg/config"
	"github.com/openvswitch/ovn-kubernetes/go-controller/pkg/util"
	"github.com/sirupsen/logrus"
)

const (
	windowsOS = "windows"
)

func configureManagementPortWindows(clusterSubnet []string, clusterServicesSubnet, routerIP, interfaceName, interfaceIP string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_, _, err := util.RunPowershell("Enable-NetAdapter", "-IncludeHidden", interfaceName)
	if err != nil {
		return err
	}
	ifAlias := fmt.Sprintf("-InterfaceAlias %s", interfaceName)
	_, _, err = util.RunPowershell("Get-NetIPAddress", ifAlias)
	if err == nil {
		logrus.Debugf("Interface %s exists, removing.", interfaceName)
		_, _, err = util.RunPowershell("Remove-NetIPAddress", ifAlias, "-Confirm:$false")
		if err != nil {
			return err
		}
	}
	portIP, interfaceIPNet, err := net.ParseCIDR(interfaceIP)
	if err != nil {
		return fmt.Errorf("Failed to parse interfaceIP %v : %v", interfaceIP, err)
	}
	portPrefix, _ := interfaceIPNet.Mask.Size()
	_, _, err = util.RunPowershell("New-NetIPAddress", fmt.Sprintf("-IPAddress %s", portIP), fmt.Sprintf("-PrefixLength %d", portPrefix), ifAlias)
	if err != nil {
		return err
	}
	_, _, err = util.RunNetsh("interface", "ipv4", "set", "subinterface", interfaceName, fmt.Sprintf("mtu=%d", config.Default.MTU), "store=persistent")
	if err != nil {
		return err
	}
	stdout, stderr, err := util.RunPowershell("$(Get-NetAdapter", "-IncludeHidden", "|", "Where", "{", "$_.Name", "-Match", fmt.Sprintf("\"%s\"", interfaceName), "}).ifIndex")
	if err != nil {
		logrus.Errorf("Failed to fetch interface index, stderr: %q, error: %v", stderr, err)
		return err
	}
	if _, err := strconv.Atoi(stdout); err != nil {
		logrus.Errorf("Failed to parse interface index %q: %v", stdout, err)
		return err
	}
	interfaceIndex := stdout
	for _, subnet := range clusterSubnet {
		subnetIP, subnetIPNet, err := net.ParseCIDR(subnet)
		if err != nil {
			return fmt.Errorf("failed to parse clusterSubnet %v : %v", subnet, err)
		}
		stdout, stderr, err = util.RunRoute("print", "-4", subnetIP.String())
		if err != nil {
			logrus.Debugf("Failed to run route print, stderr: %q, error: %v", stderr, err)
		}
		if strings.Contains(stdout, subnetIP.String()) {
			logrus.Debugf("Route was found, skipping route add")
		} else {
			subnetMask := net.IP(subnetIPNet.Mask).String()
			_, stderr, err = util.RunRoute("-p", "add", subnetIP.String(), "mask", subnetMask, routerIP, "METRIC", "2", "IF", interfaceIndex)
			if err != nil {
				logrus.Errorf("failed to run route add, stderr: %q, error: %v", stderr, err)
				return err
			}
		}
	}
	if clusterServicesSubnet != "" {
		clusterServiceIP, clusterServiceIPNet, err := net.ParseCIDR(clusterServicesSubnet)
		if err != nil {
			return fmt.Errorf("Failed to parse clusterServicesSubnet %v : %v", clusterServicesSubnet, err)
		}
		stdout, stderr, err = util.RunRoute("print", "-4", clusterServiceIP.String())
		if err != nil {
			logrus.Debugf("Failed to run route print, stderr: %q, error: %v", stderr, err)
		}
		if strings.Contains(stdout, clusterServiceIP.String()) {
			logrus.Debugf("Route was found, skipping route add")
		} else {
			clusterServiceMask := net.IP(clusterServiceIPNet.Mask).String()
			_, stderr, err = util.RunRoute("-p", "add", clusterServiceIP.String(), "mask", clusterServiceMask, routerIP, "METRIC", "2", "IF", interfaceIndex)
			if err != nil {
				logrus.Errorf("failed to run route add, stderr: %q, error: %v", stderr, err)
				return err
			}
		}
	}
	return nil
}
func configureManagementPort(clusterSubnet []string, clusterServicesSubnet, routerIP, interfaceName, interfaceIP string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	if runtime.GOOS == windowsOS {
		return configureManagementPortWindows(clusterSubnet, clusterServicesSubnet, routerIP, interfaceName, interfaceIP)
	}
	_, _, err := util.RunIP("link", "set", interfaceName, "up")
	if err != nil {
		return err
	}
	_, _, err = util.RunIP("addr", "flush", "dev", interfaceName)
	if err != nil {
		return err
	}
	_, _, err = util.RunIP("addr", "add", interfaceIP, "dev", interfaceName)
	if err != nil {
		return err
	}
	for _, subnet := range clusterSubnet {
		_, _, err = util.RunIP("route", "flush", subnet)
		if err != nil {
			return err
		}
		_, _, err = util.RunIP("route", "add", subnet, "via", routerIP)
		if err != nil {
			return err
		}
	}
	if clusterServicesSubnet != "" {
		_, _, err = util.RunIP("route", "flush", clusterServicesSubnet)
		if err != nil {
			return err
		}
		_, _, err = util.RunIP("route", "add", clusterServicesSubnet, "via", routerIP)
		if err != nil {
			return err
		}
	}
	return nil
}
func CreateManagementPort(nodeName, localSubnet, clusterServicesSubnet string, clusterSubnet []string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	ip, localSubnetNet, err := net.ParseCIDR(localSubnet)
	if err != nil {
		return fmt.Errorf("Failed to parse local subnet %v : %v", localSubnetNet, err)
	}
	ip = util.NextIP(ip)
	n, _ := localSubnetNet.Mask.Size()
	routerIPMask := fmt.Sprintf("%s/%d", ip.String(), n)
	routerIP := ip.String()
	nodeName = strings.ToLower(nodeName)
	routerMac, stderr, err := util.RunOVNNbctl("--if-exist", "get", "logical_router_port", "rtos-"+nodeName, "mac")
	if err != nil {
		logrus.Errorf("Failed to get logical router port,stderr: %q, error: %v", stderr, err)
		return err
	}
	var clusterRouter string
	if routerMac == "" {
		routerMac = util.GenerateMac()
	}
	clusterRouter, err = util.GetK8sClusterRouter()
	if err != nil {
		return err
	}
	_, stderr, err = util.RunOVNNbctl("--may-exist", "lrp-add", clusterRouter, "rtos-"+nodeName, routerMac, routerIPMask)
	if err != nil {
		logrus.Errorf("Failed to add logical port to router, stderr: %q, error: %v", stderr, err)
		return err
	}
	stdout, stderr, err := util.RunOVNNbctl("--", "--may-exist", "ls-add", nodeName, "--", "set", "logical_switch", nodeName, "other-config:subnet="+localSubnet, "external-ids:gateway_ip="+routerIPMask)
	if err != nil {
		logrus.Errorf("Failed to create a logical switch %v, stdout: %q, stderr: %q, error: %v", nodeName, stdout, stderr, err)
		return err
	}
	stdout, stderr, err = util.RunOVNNbctl("--", "--may-exist", "lsp-add", nodeName, "stor-"+nodeName, "--", "set", "logical_switch_port", "stor-"+nodeName, "type=router", "options:router-port=rtos-"+nodeName, "addresses="+"\""+routerMac+"\"")
	if err != nil {
		logrus.Errorf("Failed to add logical port to switch, stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		return err
	}
	stdout, stderr, err = util.RunOVSVsctl("--", "--may-exist", "add-br", "br-int")
	if err != nil {
		logrus.Errorf("Failed to create br-int, stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		return err
	}
	var interfaceName string
	if len(nodeName) > 11 {
		interfaceName = "k8s-" + (nodeName[:11])
	} else {
		interfaceName = "k8s-" + nodeName
	}
	stdout, stderr, err = util.RunOVSVsctl("--", "--may-exist", "add-port", "br-int", interfaceName, "--", "set", "interface", interfaceName, "type=internal", "mtu_request="+fmt.Sprintf("%d", config.Default.MTU), "external-ids:iface-id=k8s-"+nodeName)
	if err != nil {
		logrus.Errorf("Failed to add port to br-int, stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		return err
	}
	macAddress, stderr, err := util.RunOVSVsctl("--if-exists", "get", "interface", interfaceName, "mac_in_use")
	if err != nil {
		logrus.Errorf("Failed to get mac address of %v, stderr: %q, error: %v", interfaceName, stderr, err)
		return err
	}
	if macAddress == "[]" {
		return fmt.Errorf("Failed to get mac address of %v", interfaceName)
	}
	if runtime.GOOS == windowsOS && macAddress == "00:00:00:00:00:00" {
		macAddress, err = util.FetchIfMacWindows(interfaceName)
		if err != nil {
			return err
		}
	}
	ip = util.NextIP(ip)
	portIP := ip.String()
	portIPMask := fmt.Sprintf("%s/%d", portIP, n)
	stdout, stderr, err = util.RunOVNNbctl("--", "--may-exist", "lsp-add", nodeName, "k8s-"+nodeName, "--", "lsp-set-addresses", "k8s-"+nodeName, macAddress+" "+portIP)
	if err != nil {
		logrus.Errorf("Failed to add logical port to switch, stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		return err
	}
	err = configureManagementPort(clusterSubnet, clusterServicesSubnet, routerIP, interfaceName, portIPMask)
	if err != nil {
		return err
	}
	k8sClusterLbTCP, stderr, err := util.RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "load_balancer", "external_ids:k8s-cluster-lb-tcp=yes")
	if err != nil {
		logrus.Errorf("Failed to get k8sClusterLbTCP, stderr: %q, error: %v", stderr, err)
		return err
	}
	if k8sClusterLbTCP == "" {
		return fmt.Errorf("Failed to get k8sClusterLbTCP")
	}
	stdout, stderr, err = util.RunOVNNbctl("set", "logical_switch", nodeName, "load_balancer="+k8sClusterLbTCP)
	if err != nil {
		logrus.Errorf("Failed to set logical switch %v's loadbalancer, stdout: %q, stderr: %q, error: %v", nodeName, stdout, stderr, err)
		return err
	}
	k8sClusterLbUDP, stderr, err := util.RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "load_balancer", "external_ids:k8s-cluster-lb-udp=yes")
	if err != nil {
		logrus.Errorf("Failed to get k8sClusterLbUDP, stderr: %q, error: %v", stderr, err)
		return err
	}
	if k8sClusterLbUDP == "" {
		return fmt.Errorf("Failed to get k8sClusterLbUDP")
	}
	stdout, stderr, err = util.RunOVNNbctl("add", "logical_switch", nodeName, "load_balancer", k8sClusterLbUDP)
	if err != nil {
		logrus.Errorf("Failed to add logical switch %v's loadbalancer, stdout: %q, stderr: %q, error: %v", nodeName, stdout, stderr, err)
		return err
	}
	return nil
}
