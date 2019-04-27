package ovn

import (
	"fmt"
	"github.com/sirupsen/logrus"
	kapi "k8s.io/api/core/v1"
	"sync"
	"time"
)

func (oc *Controller) syncNamespaces(namespaces []interface{}) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	expectedNs := make(map[string]bool)
	for _, nsInterface := range namespaces {
		ns, ok := nsInterface.(*kapi.Namespace)
		if !ok {
			logrus.Errorf("Spurious object in syncNamespaces: %v", nsInterface)
			continue
		}
		expectedNs[ns.Name] = true
	}
	err := oc.forEachAddressSetUnhashedName(func(addrSetName, namespaceName, nameSuffix string) {
		if nameSuffix == "" && !expectedNs[namespaceName] {
			oc.deleteAddressSet(hashedAddressSet(addrSetName))
		}
	})
	if err != nil {
		logrus.Errorf("Error in syncing namespaces: %v", err)
	}
}
func (oc *Controller) waitForNamespaceEvent(namespace string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	count := 100
	for {
		if oc.namespacePolicies[namespace] != nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
		count--
		if count == 0 {
			return fmt.Errorf("timeout waiting for namespace event")
		}
	}
	return nil
}
func (oc *Controller) addPodToNamespaceAddressSet(ns, address string) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	if oc.namespacePolicies[ns] == nil {
		return
	}
	oc.namespaceMutex[ns].Lock()
	defer oc.namespaceMutex[ns].Unlock()
	if oc.namespaceAddressSet[ns][address] {
		return
	}
	oc.namespaceAddressSet[ns][address] = true
	addresses := make([]string, 0)
	for address := range oc.namespaceAddressSet[ns] {
		addresses = append(addresses, address)
	}
	oc.setAddressSet(hashedAddressSet(ns), addresses)
}
func (oc *Controller) deletePodFromNamespaceAddressSet(ns, address string) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	if address == "" || oc.namespacePolicies[ns] == nil {
		return
	}
	oc.namespaceMutex[ns].Lock()
	defer oc.namespaceMutex[ns].Unlock()
	if !oc.namespaceAddressSet[ns][address] {
		return
	}
	delete(oc.namespaceAddressSet[ns], address)
	addresses := make([]string, 0)
	for address := range oc.namespaceAddressSet[ns] {
		addresses = append(addresses, address)
	}
	oc.setAddressSet(hashedAddressSet(ns), addresses)
}
func (oc *Controller) AddNamespace(ns *kapi.Namespace) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	logrus.Debugf("Adding namespace: %s", ns.Name)
	if oc.namespaceMutex[ns.Name] == nil {
		oc.namespaceMutex[ns.Name] = &sync.Mutex{}
	}
	oc.namespaceMutex[ns.Name].Lock()
	defer oc.namespaceMutex[ns.Name].Unlock()
	oc.namespaceAddressSet[ns.Name] = make(map[string]bool)
	existingPods, err := oc.kube.GetPods(ns.Name)
	if err != nil {
		logrus.Errorf("Failed to get all the pods (%v)", err)
	} else {
		for _, pod := range existingPods.Items {
			if pod.Status.PodIP != "" {
				oc.namespaceAddressSet[ns.Name][pod.Status.PodIP] = true
			}
		}
	}
	addresses := make([]string, 0)
	for address := range oc.namespaceAddressSet[ns.Name] {
		addresses = append(addresses, address)
	}
	oc.createAddressSet(ns.Name, hashedAddressSet(ns.Name), addresses)
	oc.namespacePolicies[ns.Name] = make(map[string]*namespacePolicy)
}
func (oc *Controller) deleteNamespace(ns *kapi.Namespace) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	logrus.Debugf("Deleting namespace: %+v", ns.Name)
	if oc.namespacePolicies[ns.Name] == nil {
		return
	}
	oc.namespaceMutex[ns.Name].Lock()
	oc.deleteAddressSet(hashedAddressSet(ns.Name))
	oc.namespacePolicies[ns.Name] = nil
	oc.namespaceAddressSet[ns.Name] = nil
	oc.namespaceMutex[ns.Name].Unlock()
	oc.namespaceMutex[ns.Name] = nil
}
