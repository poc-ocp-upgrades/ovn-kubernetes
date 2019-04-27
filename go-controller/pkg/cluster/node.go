package cluster

import (
	"net"
	"time"
	"github.com/sirupsen/logrus"
	"github.com/openvswitch/ovn-kubernetes/go-controller/pkg/cni"
	"github.com/openvswitch/ovn-kubernetes/go-controller/pkg/config"
	"github.com/openvswitch/ovn-kubernetes/go-controller/pkg/ovn"
	kapi "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

func (cluster *OvnClusterController) StartClusterNode(name string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	count := 300
	var err error
	var node *kapi.Node
	var subnet *net.IPNet
	var clusterSubnets []string
	for _, clusterSubnet := range cluster.ClusterIPNet {
		clusterSubnets = append(clusterSubnets, clusterSubnet.CIDR.String())
	}
	for count > 0 {
		if count != 300 {
			time.Sleep(time.Second)
		}
		count--
		node, err = cluster.Kube.GetNode(name)
		if err != nil {
			logrus.Errorf("Error starting node %s, no node found - %v", name, err)
			continue
		}
		sub, ok := node.Annotations[OvnHostSubnet]
		if !ok {
			logrus.Errorf("Error starting node %s, no annotation found on node for subnet - %v", name, err)
			continue
		}
		_, subnet, err = net.ParseCIDR(sub)
		if err != nil {
			logrus.Errorf("Invalid hostsubnet found for node %s - %v", node.Name, err)
			return err
		}
		break
	}
	if count == 0 {
		logrus.Errorf("Failed to get node/node-annotation for %s - %v", name, err)
		return err
	}
	logrus.Infof("Node %s ready for ovn initialization with subnet %s", node.Name, subnet.String())
	err = setupOVNNode(name)
	if err != nil {
		return err
	}
	err = ovn.CreateManagementPort(node.Name, subnet.String(), cluster.ClusterServicesSubnet, clusterSubnets)
	if err != nil {
		return err
	}
	if cluster.GatewayInit {
		err = cluster.initGateway(node.Name, clusterSubnets, subnet.String())
		if err != nil {
			return err
		}
	}
	if err = config.WriteCNIConfig(); err != nil {
		return err
	}
	if cluster.OvnHA {
		err = cluster.watchNamespaceUpdate(node, subnet.String())
		return err
	}
	cniServer := cni.NewCNIServer("")
	err = cniServer.Start(cni.HandleCNIRequest)
	return err
}
func (cluster *OvnClusterController) updateOvnNode(masterIP string, node *kapi.Node, subnet string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	err := config.UpdateOvnNodeAuth(masterIP)
	if err != nil {
		return err
	}
	err = setupOVNNode(node.Name)
	if err != nil {
		logrus.Errorf("Failed to setup OVN node (%v)", err)
		return err
	}
	var clusterSubnets []string
	for _, clusterSubnet := range cluster.ClusterIPNet {
		clusterSubnets = append(clusterSubnets, clusterSubnet.CIDR.String())
	}
	err = ovn.CreateManagementPort(node.Name, subnet, cluster.ClusterServicesSubnet, clusterSubnets)
	if err != nil {
		return err
	}
	if cluster.GatewayInit {
		err = cluster.initGateway(node.Name, clusterSubnets, subnet)
		if err != nil {
			return err
		}
	}
	return nil
}
func (cluster *OvnClusterController) watchNamespaceUpdate(node *kapi.Node, subnet string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_, err := cluster.watchFactory.AddNamespaceHandler(cache.ResourceEventHandlerFuncs{UpdateFunc: func(old, newer interface{}) {
		oldNs := old.(*kapi.Namespace)
		oldMasterIP := oldNs.Annotations[MasterOverlayIP]
		newNs := newer.(*kapi.Namespace)
		newMasterIP := newNs.Annotations[MasterOverlayIP]
		if newMasterIP != oldMasterIP {
			err := cluster.updateOvnNode(newMasterIP, node, subnet)
			if err != nil {
				logrus.Errorf("Failed to update OVN node with new "+"masterIP %s: %v", newMasterIP, err)
			}
		}
	}}, nil)
	return err
}
