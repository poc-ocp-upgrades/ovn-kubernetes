package ovn

import (
	"fmt"
	"github.com/sirupsen/logrus"
	kapi "k8s.io/api/core/v1"
	"net"
)

func isServiceIPSet(service *kapi.Service) bool {
	_logClusterCodePath()
	defer _logClusterCodePath()
	return service.Spec.ClusterIP != kapi.ClusterIPNone && service.Spec.ClusterIP != ""
}
func (ovn *Controller) syncServices(services []interface{}) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	clusterServices := make(map[string][]string)
	nodeportServices := make(map[string][]string)
	lbServices := make(map[string][]string)
	for _, serviceInterface := range services {
		service, ok := serviceInterface.(*kapi.Service)
		if !ok {
			logrus.Errorf("Spurious object in syncServices: %v", serviceInterface)
			continue
		}
		if service.Spec.Type != kapi.ServiceTypeClusterIP && service.Spec.Type != kapi.ServiceTypeNodePort && service.Spec.Type != kapi.ServiceTypeLoadBalancer {
			continue
		}
		if !isServiceIPSet(service) {
			logrus.Debugf("Skipping service %s due to clusterIP = %q", service.Name, service.Spec.ClusterIP)
			continue
		}
		for _, svcPort := range service.Spec.Ports {
			protocol := svcPort.Protocol
			if protocol == "" || (protocol != TCP && protocol != UDP) {
				protocol = TCP
			}
			if service.Spec.Type == kapi.ServiceTypeNodePort {
				port := fmt.Sprintf("%d", svcPort.NodePort)
				if protocol == TCP {
					nodeportServices[TCP] = append(nodeportServices[TCP], port)
				} else {
					nodeportServices[UDP] = append(nodeportServices[UDP], port)
				}
			}
			if svcPort.Port == 0 {
				continue
			}
			key := fmt.Sprintf("%s:%d", service.Spec.ClusterIP, svcPort.Port)
			if protocol == TCP {
				clusterServices[TCP] = append(clusterServices[TCP], key)
			} else {
				clusterServices[UDP] = append(clusterServices[UDP], key)
			}
			if len(service.Spec.ExternalIPs) == 0 {
				continue
			}
			for _, extIP := range service.Spec.ExternalIPs {
				key := fmt.Sprintf("%s:%d", extIP, svcPort.Port)
				if protocol == TCP {
					lbServices[TCP] = append(lbServices[TCP], key)
				} else {
					lbServices[UDP] = append(lbServices[UDP], key)
				}
			}
		}
	}
	for _, protocol := range []string{TCP, UDP} {
		loadBalancer, err := ovn.getLoadBalancer(kapi.Protocol(protocol))
		if err != nil {
			logrus.Errorf("Failed to get load-balancer for %s (%v)", kapi.Protocol(protocol), err)
			continue
		}
		loadBalancerVIPS, err := ovn.getLoadBalancerVIPS(loadBalancer)
		if err != nil {
			logrus.Errorf("failed to get load-balancer vips for %s (%v)", loadBalancer, err)
			continue
		}
		if loadBalancerVIPS == nil {
			continue
		}
		for vip := range loadBalancerVIPS {
			if !stringSliceMembership(clusterServices[protocol], vip) {
				logrus.Debugf("Deleting stale cluster vip %s in "+"loadbalancer %s", vip, loadBalancer)
				ovn.deleteLoadBalancerVIP(loadBalancer, vip)
			}
		}
	}
	gateways, stderr, err := ovn.getOvnGateways()
	if err != nil {
		logrus.Errorf("failed to get ovn gateways. Not syncing nodeport"+"stdout: %q, stderr: %q (%v)", gateways, stderr, err)
		return
	}
	for _, gateway := range gateways {
		for _, protocol := range []string{TCP, UDP} {
			loadBalancer, err := ovn.getGatewayLoadBalancer(gateway, protocol)
			if err != nil {
				logrus.Errorf("physical gateway %s does not have "+"load_balancer (%v)", gateway, err)
				continue
			}
			if loadBalancer == "" {
				continue
			}
			loadBalancerVIPS, err := ovn.getLoadBalancerVIPS(loadBalancer)
			if err != nil {
				logrus.Errorf("failed to get load-balancer vips for %s (%v)", loadBalancer, err)
				continue
			}
			if loadBalancerVIPS == nil {
				continue
			}
			for vip := range loadBalancerVIPS {
				_, port, err := net.SplitHostPort(vip)
				if err != nil {
					logrus.Errorf("failed to split %s to vip and port (%v)", vip, err)
					continue
				}
				if !stringSliceMembership(nodeportServices[protocol], port) && !stringSliceMembership(lbServices[protocol], vip) {
					logrus.Debugf("Deleting stale nodeport vip %s in "+"loadbalancer %s", vip, loadBalancer)
					ovn.deleteLoadBalancerVIP(loadBalancer, vip)
				}
			}
		}
	}
}
func (ovn *Controller) deleteService(service *kapi.Service) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	if !isServiceIPSet(service) || len(service.Spec.Ports) == 0 {
		return
	}
	ips := make([]string, 0)
	for _, svcPort := range service.Spec.Ports {
		var port int32
		if service.Spec.Type == kapi.ServiceTypeNodePort {
			port = svcPort.NodePort
		} else {
			port = svcPort.Port
		}
		if port == 0 {
			continue
		}
		protocol := svcPort.Protocol
		if protocol == "" || (protocol != TCP && protocol != UDP) {
			protocol = TCP
		}
		var targetPort int32
		if service.Spec.Type == kapi.ServiceTypeNodePort && ovn.nodePortEnable {
			err := ovn.createGatewaysVIP(string(protocol), port, targetPort, ips)
			if err != nil {
				logrus.Errorf("Error in deleting NodePort gateway entry for service "+"%s:%d %+v", service.Name, port, err)
			}
		}
		if service.Spec.Type == kapi.ServiceTypeNodePort || service.Spec.Type == kapi.ServiceTypeClusterIP {
			loadBalancer, err := ovn.getLoadBalancer(protocol)
			if err != nil {
				logrus.Errorf("Failed to get load-balancer for %s (%v)", protocol, err)
				break
			}
			err = ovn.createLoadBalancerVIP(loadBalancer, service.Spec.ClusterIP, svcPort.Port, ips, targetPort)
			if err != nil {
				logrus.Errorf("Error in deleting load balancer for service "+"%s:%d %+v", service.Name, port, err)
			}
		}
		ovn.handleExternalIPs(service, svcPort, ips, targetPort)
	}
}
