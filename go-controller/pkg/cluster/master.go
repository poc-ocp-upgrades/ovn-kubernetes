package cluster

import (
	"fmt"
	"net"
	kapi "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
	"github.com/openvswitch/ovn-kubernetes/go-controller/pkg/ovn"
	"github.com/openvswitch/ovn-kubernetes/go-controller/pkg/util"
	"github.com/openshift/origin/pkg/util/netutils"
	"github.com/sirupsen/logrus"
)

func (cluster *OvnClusterController) RebuildOVNDatabase(masterNodeName string, oc *ovn.Controller) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	logrus.Debugf("Rebuild OVN database for cluster nodes")
	var err error
	ipChange, err := cluster.checkMasterIPChange(masterNodeName)
	if err != nil {
		logrus.Errorf("Error when checking Master Node IP Change: %v", err)
		return err
	}
	logrus.Debugf("cluster.OvnHA: %t", cluster.OvnHA)
	if cluster.OvnHA && ipChange {
		logrus.Debugf("HA is enabled and DB doesn't exist!")
		err = cluster.UpdateDBForKubeNodes(masterNodeName)
		if err != nil {
			return err
		}
		err = cluster.UpdateKubeNsObjects(oc)
		if err != nil {
			return err
		}
		err = cluster.UpdateMasterNodeIP(masterNodeName)
		if err != nil {
			return err
		}
	}
	return nil
}
func (cluster *OvnClusterController) UpdateDBForKubeNodes(masterNodeName string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	nodes, err := cluster.Kube.GetNodes()
	if err != nil {
		logrus.Errorf("Failed to obtain k8s nodes: %v", err)
		return err
	}
	for _, node := range nodes.Items {
		subnet, ok := node.Annotations[OvnHostSubnet]
		if ok {
			logrus.Debugf("ovn_host_subnet: %s", subnet)
			ip, localNet, err := net.ParseCIDR(subnet)
			if err != nil {
				return fmt.Errorf("Failed to parse subnet %v: %v", subnet, err)
			}
			ip = util.NextIP(ip)
			n, _ := localNet.Mask.Size()
			routerIPMask := fmt.Sprintf("%s/%d", ip.String(), n)
			stdout, stderr, err := util.RunOVNNbctl("--may-exist", "ls-add", node.Name, "--", "set", "logical_switch", node.Name, fmt.Sprintf("other-config:subnet=%s", subnet), fmt.Sprintf("external-ids:gateway_ip=%s", routerIPMask))
			if err != nil {
				logrus.Errorf("Failed to create logical switch for "+"node %s, stdout: %q, stderr: %q, error: %v", node.Name, stdout, stderr, err)
				return err
			}
		}
	}
	return nil
}
func (cluster *OvnClusterController) UpdateKubeNsObjects(oc *ovn.Controller) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	namespaces, err := cluster.Kube.GetNamespaces()
	if err != nil {
		logrus.Errorf("Failed to get k8s namespaces: %v", err)
		return err
	}
	for _, ns := range namespaces.Items {
		oc.AddNamespace(&ns)
		pods, err := cluster.Kube.GetPods(ns.Name)
		if err != nil {
			logrus.Errorf("Failed to get k8s pods: %v", err)
			return err
		}
		for _, pod := range pods.Items {
			oc.AddLogicalPortWithIP(&pod)
		}
		endpoints, err := cluster.Kube.GetEndpoints(ns.Name)
		if err != nil {
			logrus.Errorf("Failed to get k8s endpoints: %v", err)
			return err
		}
		for _, ep := range endpoints.Items {
			er := oc.AddEndpoints(&ep)
			if er != nil {
				logrus.Errorf("Error adding endpoints: %v", er)
				return er
			}
		}
		policies, err := cluster.Kube.GetNetworkPolicies(ns.Name)
		if err != nil {
			logrus.Errorf("Failed to get k8s network policies: %v", err)
			return err
		}
		for _, policy := range policies.Items {
			oc.AddNetworkPolicy(&policy)
		}
	}
	return nil
}
func (cluster *OvnClusterController) UpdateMasterNodeIP(masterNodeName string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	masterNodeIP, err := netutils.GetNodeIP(masterNodeName)
	if err != nil {
		return fmt.Errorf("Failed to obtain local IP from master node "+"%q: %v", masterNodeName, err)
	}
	defaultNs, err := cluster.Kube.GetNamespace(DefaultNamespace)
	if err != nil {
		return fmt.Errorf("Failed to get default namespace: %v", err)
	}
	masterIP, ok := defaultNs.Annotations[MasterOverlayIP]
	if !ok || masterIP != masterNodeIP {
		err := cluster.Kube.SetAnnotationOnNamespace(defaultNs, MasterOverlayIP, masterNodeIP)
		if err != nil {
			return fmt.Errorf("Failed to set %s=%s on namespace %s: %v", MasterOverlayIP, masterNodeIP, defaultNs.Name, err)
		}
	}
	return nil
}
func (cluster *OvnClusterController) checkMasterIPChange(masterNodeName string) (bool, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	masterNodeIP, err := netutils.GetNodeIP(masterNodeName)
	if err != nil {
		return false, fmt.Errorf("Failed to obtain local IP from master "+"node %q: %v", masterNodeName, err)
	}
	defaultNs, err := cluster.Kube.GetNamespace(DefaultNamespace)
	if err != nil {
		return false, fmt.Errorf("Failed to get default namespace: %v", err)
	}
	masterIP := defaultNs.Annotations[MasterOverlayIP]
	logrus.Debugf("Master IP: %s, Annotated IP: %s", masterNodeIP, masterIP)
	if masterIP != masterNodeIP {
		logrus.Debugf("Detected Master node IP is different than default " + "namespae annotated IP.")
		return true, nil
	}
	return false, nil
}
func (cluster *OvnClusterController) StartClusterMaster(masterNodeName string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	alreadyAllocated := make([]string, 0)
	existingNodes, err := cluster.Kube.GetNodes()
	if err != nil {
		logrus.Errorf("Error in initializing/fetching subnets: %v", err)
		return err
	}
	for _, node := range existingNodes.Items {
		hostsubnet, ok := node.Annotations[OvnHostSubnet]
		if ok {
			alreadyAllocated = append(alreadyAllocated, hostsubnet)
		}
	}
	masterSubnetAllocatorList := make([]*netutils.SubnetAllocator, 0)
	for _, clusterEntry := range cluster.ClusterIPNet {
		subrange := make([]string, 0)
		for _, allocatedRange := range alreadyAllocated {
			firstAddress, _, err := net.ParseCIDR(allocatedRange)
			if err != nil {
				return err
			}
			if clusterEntry.CIDR.Contains(firstAddress) {
				subrange = append(subrange, allocatedRange)
			}
		}
		subnetAllocator, err := netutils.NewSubnetAllocator(clusterEntry.CIDR.String(), 32-clusterEntry.HostSubnetLength, subrange)
		if err != nil {
			return err
		}
		masterSubnetAllocatorList = append(masterSubnetAllocatorList, subnetAllocator)
	}
	cluster.masterSubnetAllocatorList = masterSubnetAllocatorList
	for _, node := range existingNodes.Items {
		_, ok := node.Annotations[OvnHostSubnet]
		if !ok {
			err := cluster.addNode(&node)
			if err != nil {
				logrus.Errorf("error creating subnet for node %s: %v", node.Name, err)
			}
		}
	}
	if err := cluster.SetupMaster(masterNodeName); err != nil {
		logrus.Errorf("Failed to setup master (%v)", err)
		return err
	}
	return cluster.watchNodes()
}
func (cluster *OvnClusterController) SetupMaster(masterNodeName string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	if err := setupOVNMaster(masterNodeName); err != nil {
		return err
	}
	stdout, stderr, err := util.RunOVNNbctl("--", "--may-exist", "lr-add", OvnClusterRouter, "--", "set", "logical_router", OvnClusterRouter, "external_ids:k8s-cluster-router=yes")
	if err != nil {
		logrus.Errorf("Failed to create a single common distributed router for the cluster, "+"stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		return err
	}
	k8sClusterLbTCP, stderr, err := util.RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "load_balancer", "external_ids:k8s-cluster-lb-tcp=yes")
	if err != nil {
		logrus.Errorf("Failed to get tcp load-balancer, stderr: %q, error: %v", stderr, err)
		return err
	}
	if k8sClusterLbTCP == "" {
		stdout, stderr, err = util.RunOVNNbctl("--", "create", "load_balancer", "external_ids:k8s-cluster-lb-tcp=yes", "protocol=tcp")
		if err != nil {
			logrus.Errorf("Failed to create tcp load-balancer, stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
			return err
		}
	}
	k8sClusterLbUDP, stderr, err := util.RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "load_balancer", "external_ids:k8s-cluster-lb-udp=yes")
	if err != nil {
		logrus.Errorf("Failed to get udp load-balancer, stderr: %q, error: %v", stderr, err)
		return err
	}
	if k8sClusterLbUDP == "" {
		stdout, stderr, err = util.RunOVNNbctl("--", "create", "load_balancer", "external_ids:k8s-cluster-lb-udp=yes", "protocol=udp")
		if err != nil {
			logrus.Errorf("Failed to create udp load-balancer, stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
			return err
		}
	}
	stdout, stderr, err = util.RunOVNNbctl("--may-exist", "ls-add", "join")
	if err != nil {
		logrus.Errorf("Failed to create logical switch called \"join\", stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		return err
	}
	routerMac, stderr, err := util.RunOVNNbctl("--if-exist", "get", "logical_router_port", "rtoj-"+OvnClusterRouter, "mac")
	if err != nil {
		logrus.Errorf("Failed to get logical router port rtoj-%v, stderr: %q, error: %v", OvnClusterRouter, stderr, err)
		return err
	}
	if routerMac == "" {
		routerMac = util.GenerateMac()
		stdout, stderr, err = util.RunOVNNbctl("--", "--may-exist", "lrp-add", OvnClusterRouter, "rtoj-"+OvnClusterRouter, routerMac, "100.64.1.1/24", "--", "set", "logical_router_port", "rtoj-"+OvnClusterRouter, "external_ids:connect_to_join=yes")
		if err != nil {
			logrus.Errorf("Failed to add logical router port rtoj-%v, stdout: %q, stderr: %q, error: %v", OvnClusterRouter, stdout, stderr, err)
			return err
		}
	}
	stdout, stderr, err = util.RunOVNNbctl("--", "--may-exist", "lsp-add", "join", "jtor-"+OvnClusterRouter, "--", "set", "logical_switch_port", "jtor-"+OvnClusterRouter, "type=router", "options:router-port=rtoj-"+OvnClusterRouter, "addresses="+"\""+routerMac+"\"")
	if err != nil {
		logrus.Errorf("Failed to add router-type logical switch port to join, stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		return err
	}
	stdout, stderr, err = util.RunOVNNbctl("--", "set", "nb_global", ".", "external-ids:gateway-lock=\"\"")
	if err != nil {
		logrus.Errorf("Failed to create lock for gateways, "+"stdout: %q, stderr: %q, error: %v", stdout, stderr, err)
		return err
	}
	return nil
}
func (cluster *OvnClusterController) addNode(node *kapi.Node) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	hostsubnet, ok := node.Annotations[OvnHostSubnet]
	if ok {
		_, _, err := net.ParseCIDR(hostsubnet)
		if err == nil {
			return nil
		}
	}
	for _, possibleSubnet := range cluster.masterSubnetAllocatorList {
		sn, err := possibleSubnet.GetNetwork()
		if err == netutils.ErrSubnetAllocatorFull {
			continue
		} else if err != nil {
			return fmt.Errorf("Error allocating network for node %s: %v", node.Name, err)
		} else {
			err = cluster.Kube.SetAnnotationOnNode(node, OvnHostSubnet, sn.String())
			if err != nil {
				_ = possibleSubnet.ReleaseNetwork(sn)
				return fmt.Errorf("Error creating subnet %s for node %s: %v", sn.String(), node.Name, err)
			}
			logrus.Infof("Created HostSubnet %s", sn.String())
			return nil
		}
	}
	return fmt.Errorf("error allocating netork for node %s: No more allocatable ranges", node.Name)
}
func (cluster *OvnClusterController) deleteNode(node *kapi.Node) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	sub, ok := node.Annotations[OvnHostSubnet]
	if !ok {
		return fmt.Errorf("Error in obtaining host subnet for node %q for deletion", node.Name)
	}
	_, subnet, err := net.ParseCIDR(sub)
	if err != nil {
		return fmt.Errorf("Error in parsing hostsubnet - %v", err)
	}
	for _, possibleSubnet := range cluster.masterSubnetAllocatorList {
		err = possibleSubnet.ReleaseNetwork(subnet)
		if err == nil {
			logrus.Infof("Deleted HostSubnet %s for node %s", sub, node.Name)
			return nil
		}
	}
	return fmt.Errorf("Error deleting subnet %v for node %q: subnet not found in any CIDR range or already available", sub, node.Name)
}
func (cluster *OvnClusterController) watchNodes() error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_, err := cluster.watchFactory.AddNodeHandler(cache.ResourceEventHandlerFuncs{AddFunc: func(obj interface{}) {
		node := obj.(*kapi.Node)
		logrus.Debugf("Added event for Node %q", node.Name)
		err := cluster.addNode(node)
		if err != nil {
			logrus.Errorf("error creating subnet for node %s: %v", node.Name, err)
		}
	}, UpdateFunc: func(old, new interface{}) {
	}, DeleteFunc: func(obj interface{}) {
		node := obj.(*kapi.Node)
		logrus.Debugf("Delete event for Node %q", node.Name)
		err := cluster.deleteNode(node)
		if err != nil {
			logrus.Errorf("Error deleting node %s: %v", node.Name, err)
		}
		err = util.RemoveNode(node.Name)
		if err != nil {
			logrus.Errorf("Failed to remove node %s (%v)", node.Name, err)
		}
	}}, nil)
	return err
}
