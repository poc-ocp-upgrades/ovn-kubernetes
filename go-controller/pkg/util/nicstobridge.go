package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"syscall"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
)

const (
	ubuntuDefaultFile	= "/etc/default/openvswitch-switch"
	rhelDefaultFile		= "/etc/default/openvswitch"
)

func getBridgeName(iface string) string {
	_logClusterCodePath()
	defer _logClusterCodePath()
	return fmt.Sprintf("br%s", iface)
}
func GetNicName(brName string) string {
	_logClusterCodePath()
	defer _logClusterCodePath()
	stdout, stderr, err := RunOVSVsctl("br-get-external-id", brName, "bridge-uplink")
	if err != nil {
		logrus.Errorf("Failed to get the bridge-uplink for the bridge %q:, stderr: %q, error: %v", brName, stderr, err)
		return ""
	}
	if stdout == "" && strings.HasPrefix(brName, "br") {
		return fmt.Sprintf("%s", brName[len("br"):])
	}
	return stdout
}
func saveIPAddress(oldLink, newLink netlink.Link, addrs []netlink.Addr) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	for i := range addrs {
		addr := addrs[i]
		if err := netlink.AddrDel(oldLink, &addr); err != nil {
			logrus.Errorf("Remove addr from %q failed: %v", oldLink.Attrs().Name, err)
			return err
		}
		addr.Label = newLink.Attrs().Name
		if err := netlink.AddrAdd(newLink, &addr); err != nil {
			logrus.Errorf("Add addr to newLink %q failed: %v", newLink.Attrs().Name, err)
			return err
		}
		logrus.Infof("Successfully saved addr %q to newLink %q", addr.String(), newLink.Attrs().Name)
	}
	return netlink.LinkSetUp(newLink)
}
func delAddRoute(oldLink, newLink netlink.Link, route netlink.Route) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	if err := netlink.RouteDel(&route); err != nil && !strings.Contains(err.Error(), "no such process") {
		logrus.Errorf("Remove route from %q failed: %v", oldLink.Attrs().Name, err)
		return err
	}
	route.LinkIndex = newLink.Attrs().Index
	if err := netlink.RouteAdd(&route); err != nil && !os.IsExist(err) {
		logrus.Errorf("Add route to newLink %q failed: %v", newLink.Attrs().Name, err)
		return err
	}
	logrus.Infof("Successfully saved route %q", route.String())
	return nil
}
func saveRoute(oldLink, newLink netlink.Link, routes []netlink.Route) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	for i := range routes {
		route := routes[i]
		if route.Dst == nil && route.Gw != nil && route.LinkIndex > 0 {
			continue
		}
		err := delAddRoute(oldLink, newLink, route)
		if err != nil {
			return err
		}
	}
	for i := range routes {
		route := routes[i]
		if route.Dst == nil && route.Gw != nil && route.LinkIndex > 0 {
			err := delAddRoute(oldLink, newLink, route)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
func setupDefaultFile() {
	_logClusterCodePath()
	defer _logClusterCodePath()
	platform, err := runningPlatform()
	if err != nil {
		logrus.Errorf("Failed to set OVS package default file (%v)", err)
		return
	}
	var defaultFile, text string
	if platform == ubuntu {
		defaultFile = ubuntuDefaultFile
		text = "OVS_CTL_OPTS=\"$OVS_CTL_OPTS --delete-transient-ports\""
	} else if platform == rhel {
		defaultFile = rhelDefaultFile
		text = "OPTIONS=--delete-transient-ports"
	} else {
		return
	}
	fileContents, err := ioutil.ReadFile(defaultFile)
	if err != nil {
		logrus.Errorf("failed to parse file %s (%v)", defaultFile, err)
		return
	}
	ss := strings.Split(string(fileContents), "\n")
	for _, line := range ss {
		if strings.Contains(line, "--delete-transient-ports") {
			return
		}
	}
	f, err := os.OpenFile(defaultFile, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		logrus.Errorf("failed to open %s to write (%v)", defaultFile, err)
		return
	}
	defer f.Close()
	if _, err = f.WriteString(text); err != nil {
		logrus.Errorf("failed to write to %s (%v)", defaultFile, err)
		return
	}
}
func NicToBridge(iface string) (string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	ifaceLink, err := netlink.LinkByName(iface)
	if err != nil {
		return "", err
	}
	bridge := getBridgeName(iface)
	stdout, stderr, err := RunOVSVsctl("--", "--may-exist", "add-br", bridge, "--", "br-set-external-id", bridge, "bridge-id", bridge, "--", "br-set-external-id", bridge, "bridge-uplink", iface, "--", "set", "bridge", bridge, "fail-mode=standalone", fmt.Sprintf("other_config:hwaddr=%s", ifaceLink.Attrs().HardwareAddr), "--", "--may-exist", "add-port", bridge, iface, "--", "set", "port", iface, "other-config:transient=true")
	if err != nil {
		logrus.Errorf("Failed to create OVS bridge, stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		return "", err
	}
	logrus.Infof("Successfully created OVS bridge %q", bridge)
	setupDefaultFile()
	addrs, err := netlink.AddrList(ifaceLink, syscall.AF_INET)
	if err != nil {
		return "", err
	}
	routes, err := netlink.RouteList(ifaceLink, syscall.AF_INET)
	if err != nil {
		return "", err
	}
	bridgeLink, err := netlink.LinkByName(bridge)
	if err != nil {
		return "", err
	}
	if err = saveIPAddress(ifaceLink, bridgeLink, addrs); err != nil {
		return "", err
	}
	if err = saveRoute(ifaceLink, bridgeLink, routes); err != nil {
		return "", err
	}
	return bridge, nil
}
func BridgeToNic(bridge string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	bridgeLink, err := netlink.LinkByName(bridge)
	if err != nil {
		return err
	}
	addrs, err := netlink.AddrList(bridgeLink, syscall.AF_INET)
	if err != nil {
		return err
	}
	routes, err := netlink.RouteList(bridgeLink, syscall.AF_INET)
	if err != nil {
		return err
	}
	ifaceLink, err := netlink.LinkByName(GetNicName(bridge))
	if err != nil {
		return err
	}
	if err = saveIPAddress(bridgeLink, ifaceLink, addrs); err != nil {
		return err
	}
	if err = saveRoute(bridgeLink, ifaceLink, routes); err != nil {
		return err
	}
	stdout, stderr, err := RunOVSVsctl("--", "--if-exists", "del-br", bridge)
	if err != nil {
		logrus.Errorf("Failed to delete OVS bridge, stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		return err
	}
	logrus.Infof("Successfully deleted OVS bridge %q", bridge)
	stdout, stderr, err = RunOVSVsctl("--", "--if-exists", "del-port", "br-int", fmt.Sprintf("k8s-patch-br-int-%s", bridge))
	if err != nil {
		logrus.Errorf("Failed to delete patch port on br-int, stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		return err
	}
	return nil
}
