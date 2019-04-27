package cni

import (
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"github.com/sirupsen/logrus"
	"github.com/Microsoft/hcsshim"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/openvswitch/ovn-kubernetes/go-controller/pkg/config"
)

func getHNSIdFromConfigOrByGatewayIP(gatewayIP string) (string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	if config.CNI.WinHNSNetworkID != "" {
		logrus.Infof("Using HNS Network Id from config: %v", config.CNI.WinHNSNetworkID)
		return config.CNI.WinHNSNetworkID, nil
	}
	hnsNetworkId := ""
	hnsNetworks, err := hcsshim.HNSListNetworkRequest("GET", "", "")
	if err != nil {
		return "", err
	}
	for _, hnsNW := range hnsNetworks {
		for _, hnsNWSubnet := range hnsNW.Subnets {
			if strings.Compare(gatewayIP, hnsNWSubnet.GatewayAddress) == 0 {
				if len(hnsNetworkId) == 0 {
					hnsNetworkId = hnsNW.Id
				} else {
					return "", fmt.Errorf("Found more than one network suitable for containers, " + "please specify win-hnsnetwork-id in config")
				}
			}
		}
	}
	if len(hnsNetworkId) != 0 {
		logrus.Infof("HNS Network Id found: %v", hnsNetworkId)
		return hnsNetworkId, nil
	}
	return "", fmt.Errorf("Could not find any suitable network to attach the container")
}
func createHNSEndpoint(hnsConfiguration *hcsshim.HNSEndpoint) (*hcsshim.HNSEndpoint, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	logrus.Infof("Creating HNS endpoint")
	hnsConfigBytes, err := json.Marshal(hnsConfiguration)
	if err != nil {
		return nil, err
	}
	logrus.Infof("hnsConfigBytes: %v", string(hnsConfigBytes))
	createdHNSEndpoint, err := hcsshim.HNSEndpointRequest("POST", "", string(hnsConfigBytes))
	if err != nil {
		logrus.Errorf("Could not create the HNSEndpoint, error: %v", err)
		return nil, err
	}
	logrus.Infof("Created HNS endpoint with ID: %v", createdHNSEndpoint.Id)
	return createdHNSEndpoint, nil
}
func containerHotAttachEndpoint(existingHNSEndpoint *hcsshim.HNSEndpoint, containerID string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	logrus.Infof("Attaching endpoint %v to container %v", existingHNSEndpoint.Id, containerID)
	if err := hcsshim.HotAttachEndpoint(containerID, existingHNSEndpoint.Id); err != nil {
		logrus.Infof("Error attaching the endpoint to the container, error: %v", err)
		return err
	}
	logrus.Infof("Endpoint attached successfully to the container")
	return nil
}
func deleteHNSEndpoint(endpointName string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	logrus.Infof("Deleting HNS endpoint: %v", endpointName)
	hnsEndpoint, err := hcsshim.GetHNSEndpointByName(endpointName)
	if err == nil {
		logrus.Infof("Fetched endpoint: %v", endpointName)
		_, err = hnsEndpoint.Delete()
		if err != nil {
			logrus.Warningf("Failed to delete HNS endpoint: %q", err)
		} else {
			logrus.Infof("HNS endpoint successfully deleted: %q", endpointName)
		}
		return err
	}
	logrus.Infof("No endpoint with name %v was found, error %v", endpointName, err)
	return nil
}
func (pr *PodRequest) ConfigureInterface(namespace string, podName string, macAddress string, ipAddress string, gatewayIP string, mtu int, ingress, egress int64) ([]*current.Interface, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	conf := pr.CNIConf
	ipAddr, ipNet, err := net.ParseCIDR(ipAddress)
	if err != nil {
		return nil, err
	}
	ipMaskSize, _ := ipNet.Mask.Size()
	endpointName := fmt.Sprintf("%s_%s", namespace, podName)
	defer func() {
		if err != nil {
			errHNSDelete := deleteHNSEndpoint(endpointName)
			if errHNSDelete != nil {
				logrus.Warningf("Failed to delete the HNS Endpoint, reason: %q", errHNSDelete)
			}
		}
	}()
	var hnsNetworkId string
	hnsNetworkId, err = getHNSIdFromConfigOrByGatewayIP(gatewayIP)
	if err != nil {
		logrus.Infof("Error when detecting the HNS Network Id: %q", err)
		return nil, err
	}
	var createdEndpoint *hcsshim.HNSEndpoint
	createdEndpoint, err = hcsshim.GetHNSEndpointByName(endpointName)
	if err != nil {
		logrus.Infof("HNS endpoint %q does not exist", endpointName)
		macAddressIpFormat := strings.Replace(macAddress, ":", "-", -1)
		hnsEndpoint := &hcsshim.HNSEndpoint{Name: endpointName, VirtualNetwork: hnsNetworkId, IPAddress: ipAddr, MacAddress: macAddressIpFormat, PrefixLength: uint8(ipMaskSize), DNSServerList: strings.Join(conf.DNS.Nameservers, ","), DNSSuffix: strings.Join(conf.DNS.Search, ",")}
		createdEndpoint, err = createHNSEndpoint(hnsEndpoint)
		if err != nil {
			return nil, err
		}
	} else {
		logrus.Infof("HNS endpoint already exists with name: %q", endpointName)
	}
	err = containerHotAttachEndpoint(createdEndpoint, pr.SandboxID)
	if err != nil {
		logrus.Warningf("Failed to hot attach HNS Endpoint %q to container %q, reason: %q", endpointName, pr.SandboxID, err)
		return nil, err
	}
	ifaceID := fmt.Sprintf("%s_%s", namespace, podName)
	ifaceName, errFind := ovsFind("interface", "name", "external-ids:iface-id="+ifaceID)
	if errFind == nil && len(ifaceName) > 0 && ifaceName[0] != "" {
		logrus.Infof("HNS endpoint %q already set up for container %q", endpointName, pr.SandboxID)
		return []*current.Interface{}, nil
	}
	ovsArgs := []string{"--may-exist", "add-port", "br-int", endpointName, "--", "set", "interface", endpointName, "type=internal", "--", "set", "interface", endpointName, fmt.Sprintf("external_ids:attached_mac=%s", macAddress), fmt.Sprintf("external_ids:iface-id=%s", ifaceID), fmt.Sprintf("external_ids:ip_address=%s", ipAddress)}
	var out []byte
	out, err = exec.Command("ovs-vsctl", ovsArgs...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failure in plugging pod interface: %v  %q", err, string(out))
	}
	mtuArgs := []string{"Set-NetIPInterface", "-IncludeAllCompartments", fmt.Sprintf("-InterfaceAlias \"vEthernet (%s)\"", ifaceID), fmt.Sprintf("-NlMtuBytes %d", mtu)}
	out, err = exec.Command("powershell", mtuArgs...).CombinedOutput()
	if err != nil {
		logrus.Warningf("Failed to set MTU on endpoint %q, with: %q", endpointName, string(out))
		return nil, fmt.Errorf("failed to set MTU on endpoint, reason: %q", err)
	}
	return []*current.Interface{}, nil
}
func (pr *PodRequest) PlatformSpecificCleanup() error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	namespace := pr.PodNamespace
	podName := pr.PodName
	if namespace == "" || podName == "" {
		logrus.Warningf("cleanup failed, required CNI variable missing from args: %v", pr)
		return nil
	}
	endpointName := fmt.Sprintf("%s_%s", namespace, podName)
	ovsArgs := []string{"del-port", "br-int", endpointName}
	out, err := exec.Command("ovs-vsctl", ovsArgs...).CombinedOutput()
	if err != nil && !strings.Contains(string(out), "no port named") {
		logrus.Warningf("failed to delete OVS port %s: %v  %q", endpointName, err, string(out))
	}
	if err = deleteHNSEndpoint(endpointName); err != nil {
		logrus.Warningf("failed to delete HNSEndpoint %v: %v", endpointName, err)
	}
	return nil
}
