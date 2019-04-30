package ovn

import (
	"github.com/openvswitch/ovn-kubernetes/go-controller/pkg/factory"
	"github.com/openvswitch/ovn-kubernetes/go-controller/pkg/kube"
	util "github.com/openvswitch/ovn-kubernetes/go-controller/pkg/util"
	"github.com/sirupsen/logrus"
	kapi "k8s.io/api/core/v1"
	kapisnetworking "k8s.io/api/networking/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"reflect"
	"sync"
)

type Controller struct {
	kube				kube.Interface
	nodePortEnable			bool
	watchFactory			*factory.WatchFactory
	gatewayCache			map[string]string
	loadbalancerClusterCache	map[string]string
	loadbalancerGWCache		map[string]string
	logicalSwitchCache		map[string]bool
	logicalPortCache		map[string]string
	logicalPortUUIDCache		map[string]string
	namespaceAddressSet		map[string]map[string]bool
	namespaceMutex			map[string]*sync.Mutex
	namespacePolicies		map[string]map[string]*namespacePolicy
	portGroupIngressDeny		string
	portGroupEgressDeny		string
	lspIngressDenyCache		map[string]int
	lspEgressDenyCache		map[string]int
	lspMutex			*sync.Mutex
	lsMutex				*sync.Mutex
	portGroupSupport		bool
}

const (
	TCP	= "TCP"
	UDP	= "UDP"
)

func NewOvnController(kubeClient kubernetes.Interface, wf *factory.WatchFactory, nodePortEnable bool) *Controller {
	_logClusterCodePath()
	defer _logClusterCodePath()
	return &Controller{kube: &kube.Kube{KClient: kubeClient}, watchFactory: wf, logicalSwitchCache: make(map[string]bool), logicalPortCache: make(map[string]string), logicalPortUUIDCache: make(map[string]string), namespaceAddressSet: make(map[string]map[string]bool), namespacePolicies: make(map[string]map[string]*namespacePolicy), namespaceMutex: make(map[string]*sync.Mutex), lspIngressDenyCache: make(map[string]int), lspEgressDenyCache: make(map[string]int), lspMutex: &sync.Mutex{}, lsMutex: &sync.Mutex{}, gatewayCache: make(map[string]string), loadbalancerClusterCache: make(map[string]string), loadbalancerGWCache: make(map[string]string), nodePortEnable: nodePortEnable}
}
func (oc *Controller) Run() error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_, _, err := util.RunOVNNbctl("--columns=_uuid", "list", "port_group")
	if err == nil {
		oc.portGroupSupport = true
	}
	for _, f := range []func() error{oc.WatchPods, oc.WatchServices, oc.WatchEndpoints, oc.WatchNamespaces, oc.WatchNetworkPolicy, oc.WatchNodes} {
		if err := f(); err != nil {
			return err
		}
	}
	return nil
}
func (oc *Controller) WatchPods() error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_, err := oc.watchFactory.AddPodHandler(cache.ResourceEventHandlerFuncs{AddFunc: func(obj interface{}) {
		pod := obj.(*kapi.Pod)
		if pod.Spec.NodeName != "" {
			oc.addLogicalPort(pod)
		}
	}, UpdateFunc: func(old, newer interface{}) {
		podNew := newer.(*kapi.Pod)
		podOld := old.(*kapi.Pod)
		if podOld.Spec.NodeName == "" && podNew.Spec.NodeName != "" {
			oc.addLogicalPort(podNew)
		}
	}, DeleteFunc: func(obj interface{}) {
		pod := obj.(*kapi.Pod)
		oc.deleteLogicalPort(pod)
	}}, oc.syncPods)
	return err
}
func (oc *Controller) WatchServices() error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_, err := oc.watchFactory.AddServiceHandler(cache.ResourceEventHandlerFuncs{AddFunc: func(obj interface{}) {
	}, UpdateFunc: func(old, new interface{}) {
	}, DeleteFunc: func(obj interface{}) {
		service := obj.(*kapi.Service)
		oc.deleteService(service)
	}}, oc.syncServices)
	return err
}
func (oc *Controller) WatchEndpoints() error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_, err := oc.watchFactory.AddEndpointsHandler(cache.ResourceEventHandlerFuncs{AddFunc: func(obj interface{}) {
		ep := obj.(*kapi.Endpoints)
		err := oc.AddEndpoints(ep)
		if err != nil {
			logrus.Errorf("Error in adding load balancer: %v", err)
		}
	}, UpdateFunc: func(old, new interface{}) {
		epNew := new.(*kapi.Endpoints)
		epOld := old.(*kapi.Endpoints)
		if reflect.DeepEqual(epNew.Subsets, epOld.Subsets) {
			return
		}
		if len(epNew.Subsets) == 0 {
			err := oc.deleteEndpoints(epNew)
			if err != nil {
				logrus.Errorf("Error in deleting endpoints - %v", err)
			}
		} else {
			err := oc.AddEndpoints(epNew)
			if err != nil {
				logrus.Errorf("Error in modifying endpoints: %v", err)
			}
		}
	}, DeleteFunc: func(obj interface{}) {
		ep := obj.(*kapi.Endpoints)
		err := oc.deleteEndpoints(ep)
		if err != nil {
			logrus.Errorf("Error in deleting endpoints - %v", err)
		}
	}}, nil)
	return err
}
func (oc *Controller) WatchNetworkPolicy() error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_, err := oc.watchFactory.AddPolicyHandler(cache.ResourceEventHandlerFuncs{AddFunc: func(obj interface{}) {
		policy := obj.(*kapisnetworking.NetworkPolicy)
		oc.AddNetworkPolicy(policy)
		return
	}, UpdateFunc: func(old, newer interface{}) {
		oldPolicy := old.(*kapisnetworking.NetworkPolicy)
		newPolicy := newer.(*kapisnetworking.NetworkPolicy)
		if !reflect.DeepEqual(oldPolicy, newPolicy) {
			oc.deleteNetworkPolicy(oldPolicy)
			oc.AddNetworkPolicy(newPolicy)
		}
		return
	}, DeleteFunc: func(obj interface{}) {
		policy := obj.(*kapisnetworking.NetworkPolicy)
		oc.deleteNetworkPolicy(policy)
		return
	}}, oc.syncNetworkPolicies)
	return err
}
func (oc *Controller) WatchNamespaces() error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_, err := oc.watchFactory.AddNamespaceHandler(cache.ResourceEventHandlerFuncs{AddFunc: func(obj interface{}) {
		ns := obj.(*kapi.Namespace)
		oc.AddNamespace(ns)
		return
	}, UpdateFunc: func(old, newer interface{}) {
		return
	}, DeleteFunc: func(obj interface{}) {
		ns := obj.(*kapi.Namespace)
		oc.deleteNamespace(ns)
		return
	}}, oc.syncNamespaces)
	return err
}
func (oc *Controller) WatchNodes() error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_, err := oc.watchFactory.AddNodeHandler(cache.ResourceEventHandlerFuncs{AddFunc: func(obj interface{}) {
	}, UpdateFunc: func(old, new interface{}) {
	}, DeleteFunc: func(obj interface{}) {
		node := obj.(*kapi.Node)
		logrus.Debugf("Delete event for Node %q. Removing the node from "+"various caches", node.Name)
		oc.lsMutex.Lock()
		delete(oc.gatewayCache, node.Name)
		delete(oc.logicalSwitchCache, node.Name)
		oc.lsMutex.Unlock()
	}}, nil)
	return err
}
