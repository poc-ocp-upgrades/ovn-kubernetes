package ovn

import (
	"fmt"
	"github.com/openvswitch/ovn-kubernetes/go-controller/pkg/factory"
	"github.com/openvswitch/ovn-kubernetes/go-controller/pkg/util"
	"github.com/sirupsen/logrus"
	kapi "k8s.io/api/core/v1"
	knet "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"strings"
)

func (oc *Controller) syncNetworkPoliciesPortGroup(networkPolicies []interface{}) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	expectedPolicies := make(map[string]map[string]bool)
	for _, npInterface := range networkPolicies {
		policy, ok := npInterface.(*knet.NetworkPolicy)
		if !ok {
			logrus.Errorf("Spurious object in syncNetworkPolicies: %v", npInterface)
			continue
		}
		expectedPolicies[policy.Namespace] = map[string]bool{policy.Name: true}
	}
	err := oc.forEachAddressSetUnhashedName(func(addrSetName, namespaceName, policyName string) {
		if policyName != "" && !expectedPolicies[namespaceName][policyName] {
			portGroupName := fmt.Sprintf("%s_%s", namespaceName, policyName)
			hashedLocalPortGroup := hashedPortGroup(portGroupName)
			oc.deletePortGroup(hashedLocalPortGroup)
			oc.deleteAddressSet(hashedAddressSet(addrSetName))
		}
	})
	if err != nil {
		logrus.Errorf("Error in syncing network policies: %v", err)
	}
}
func (oc *Controller) addACLAllow(np *namespacePolicy, match, l4Match string, ipBlockCidr bool, gressNum int, policyType knet.PolicyType) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	var direction, action string
	direction = toLport
	if policyType == knet.PolicyTypeIngress {
		action = "allow-related"
	} else {
		action = "allow"
	}
	uuid, stderr, err := util.RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "ACL", fmt.Sprintf("external-ids:l4Match=\"%s\"", l4Match), fmt.Sprintf("external-ids:ipblock_cidr=%t", ipBlockCidr), fmt.Sprintf("external-ids:namespace=%s", np.namespace), fmt.Sprintf("external-ids:policy=%s", np.name), fmt.Sprintf("external-ids:%s_num=%d", policyType, gressNum), fmt.Sprintf("external-ids:policy_type=%s", policyType))
	if err != nil {
		logrus.Errorf("find failed to get the allow rule for "+"namespace=%s, policy=%s, stderr: %q (%v)", np.namespace, np.name, stderr, err)
		return
	}
	if uuid != "" {
		return
	}
	_, stderr, err = util.RunOVNNbctl("--id=@acl", "create", "acl", fmt.Sprintf("priority=%s", defaultAllowPriority), fmt.Sprintf("direction=%s", direction), match, fmt.Sprintf("action=%s", action), fmt.Sprintf("external-ids:l4Match=\"%s\"", l4Match), fmt.Sprintf("external-ids:ipblock_cidr=%t", ipBlockCidr), fmt.Sprintf("external-ids:namespace=%s", np.namespace), fmt.Sprintf("external-ids:policy=%s", np.name), fmt.Sprintf("external-ids:%s_num=%d", policyType, gressNum), fmt.Sprintf("external-ids:policy_type=%s", policyType), "--", "add", "port_group", np.portGroupUUID, "acls", "@acl")
	if err != nil {
		logrus.Errorf("failed to create the acl allow rule for "+"namespace=%s, policy=%s, stderr: %q (%v)", np.namespace, np.name, stderr, err)
		return
	}
}
func (oc *Controller) modifyACLAllow(namespace, policy, oldMatch string, newMatch string, gressNum int, policyType knet.PolicyType) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	uuid, stderr, err := util.RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "ACL", oldMatch, fmt.Sprintf("external-ids:namespace=%s", namespace), fmt.Sprintf("external-ids:policy=%s", policy), fmt.Sprintf("external-ids:%s_num=%d", policyType, gressNum), fmt.Sprintf("external-ids:policy_type=%s", policyType))
	if err != nil {
		logrus.Errorf("find failed to get the allow rule for "+"namespace=%s, policy=%s, stderr: %q (%v)", namespace, policy, stderr, err)
		return
	}
	if uuid != "" {
		_, stderr, err = util.RunOVNNbctl("set", "acl", uuid, fmt.Sprintf("%s", newMatch))
		if err != nil {
			logrus.Errorf("failed to modify the allow-from rule for "+"namespace=%s, policy=%s, stderr: %q (%v)", namespace, policy, stderr, err)
		}
		return
	}
}
func (oc *Controller) addIPBlockACLDeny(np *namespacePolicy, except, priority string, gressNum int, policyType knet.PolicyType) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	var match, l3Match, direction, lportMatch string
	direction = toLport
	if policyType == knet.PolicyTypeIngress {
		lportMatch = fmt.Sprintf("outport == @%s", np.portGroupName)
		l3Match = fmt.Sprintf("ip4.src == %s", except)
		match = fmt.Sprintf("match=\"%s && %s\"", lportMatch, l3Match)
	} else {
		lportMatch = fmt.Sprintf("inport == @%s", np.portGroupName)
		l3Match = fmt.Sprintf("ip4.dst == %s", except)
		match = fmt.Sprintf("match=\"%s && %s\"", lportMatch, l3Match)
	}
	uuid, stderr, err := util.RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "ACL", match, "action=drop", fmt.Sprintf("external-ids:ipblock-deny-policy-type=%s", policyType), fmt.Sprintf("external-ids:namespace=%s", np.namespace), fmt.Sprintf("external-ids:%s_num=%d", policyType, gressNum), fmt.Sprintf("external-ids:policy=%s", np.name))
	if err != nil {
		logrus.Errorf("find failed to get the ipblock default deny rule for "+"namespace=%s, policy=%s stderr: %q, (%v)", np.namespace, np.name, stderr, err)
		return
	}
	if uuid != "" {
		return
	}
	_, stderr, err = util.RunOVNNbctl("--id=@acl", "create", "acl", fmt.Sprintf("priority=%s", priority), fmt.Sprintf("direction=%s", direction), match, "action=drop", fmt.Sprintf("external-ids:ipblock-deny-policy-type=%s", policyType), fmt.Sprintf("external-ids:%s_num=%d", policyType, gressNum), fmt.Sprintf("external-ids:namespace=%s", np.namespace), fmt.Sprintf("external-ids:policy=%s", np.name), "--", "add", "port_group", np.portGroupUUID, "acls", "@acl")
	if err != nil {
		logrus.Errorf("error executing create ACL command, stderr: %q, %+v", stderr, err)
	}
	return
}
func (oc *Controller) addACLDenyPortGroup(portGroupUUID, portGroupName, priority string, policyType knet.PolicyType) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	var match, direction string
	direction = toLport
	if policyType == knet.PolicyTypeIngress {
		match = fmt.Sprintf("match=\"outport == @%s\"", portGroupName)
	} else {
		match = fmt.Sprintf("match=\"inport == @%s\"", portGroupName)
	}
	uuid, stderr, err := util.RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "ACL", match, "action=drop", fmt.Sprintf("external-ids:default-deny-policy-type=%s", policyType))
	if err != nil {
		return fmt.Errorf("find failed to get the default deny rule for "+"policy type %s stderr: %q (%v)", policyType, stderr, err)
	}
	if uuid != "" {
		return nil
	}
	_, stderr, err = util.RunOVNNbctl("--id=@acl", "create", "acl", fmt.Sprintf("priority=%s", priority), fmt.Sprintf("direction=%s", direction), match, "action=drop", fmt.Sprintf("external-ids:default-deny-policy-type=%s", policyType), "--", "add", "port_group", portGroupUUID, "acls", "@acl")
	if err != nil {
		return fmt.Errorf("error executing create ACL command for "+"policy type %s stderr: %q (%v)", policyType, stderr, err)
	}
	return nil
}
func (oc *Controller) addToACLDeny(portGroup, logicalPort string) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	logicalPortUUID := oc.getLogicalPortUUID(logicalPort)
	if logicalPortUUID == "" {
		return
	}
	_, stderr, err := util.RunOVNNbctl("--if-exists", "remove", "port_group", portGroup, "ports", logicalPortUUID, "--", "add", "port_group", portGroup, "ports", logicalPortUUID)
	if err != nil {
		logrus.Errorf("Failed to add logicalPort %s to portGroup %s "+"stderr: %q (%v)", logicalPort, portGroup, stderr, err)
	}
}
func (oc *Controller) deleteFromACLDeny(portGroup, logicalPort string) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	logicalPortUUID := oc.getLogicalPortUUID(logicalPort)
	if logicalPortUUID == "" {
		return
	}
	_, stderr, err := util.RunOVNNbctl("--if-exists", "remove", "port_group", portGroup, "ports", logicalPortUUID)
	if err != nil {
		logrus.Errorf("Failed to delete logicalPort %s to portGroup %s "+"stderr: %q (%v)", logicalPort, portGroup, stderr, err)
	}
}
func (oc *Controller) localPodAddACL(np *namespacePolicy, gress *gressPolicy) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	l3Match := gress.getL3MatchFromAddressSet()
	var lportMatch, cidrMatch string
	if gress.policyType == knet.PolicyTypeIngress {
		lportMatch = fmt.Sprintf("outport == @%s", np.portGroupName)
	} else {
		lportMatch = fmt.Sprintf("inport == @%s", np.portGroupName)
	}
	if len(gress.ipBlockCidr) > 0 && len(gress.ipBlockExcept) > 0 {
		except := fmt.Sprintf("{%s}", strings.Join(gress.ipBlockExcept, ", "))
		oc.addIPBlockACLDeny(np, except, ipBlockDenyPriority, gress.idx, gress.policyType)
	}
	if len(gress.portPolicies) == 0 {
		match := fmt.Sprintf("match=\"%s && %s\"", l3Match, lportMatch)
		l4Match := noneMatch
		if len(gress.ipBlockCidr) > 0 {
			cidrMatch = gress.getMatchFromIPBlock(lportMatch, l4Match)
			oc.addACLAllow(np, cidrMatch, l4Match, true, gress.idx, gress.policyType)
		}
		oc.addACLAllow(np, match, l4Match, false, gress.idx, gress.policyType)
	}
	for _, port := range gress.portPolicies {
		l4Match, err := port.getL4Match()
		if err != nil {
			continue
		}
		match := fmt.Sprintf("match=\"%s && %s && %s\"", l3Match, l4Match, lportMatch)
		if len(gress.ipBlockCidr) > 0 {
			cidrMatch = gress.getMatchFromIPBlock(lportMatch, l4Match)
			oc.addACLAllow(np, cidrMatch, l4Match, true, gress.idx, gress.policyType)
		}
		oc.addACLAllow(np, match, l4Match, false, gress.idx, gress.policyType)
	}
}
func (oc *Controller) createDefaultDenyPortGroup(policyType knet.PolicyType) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	var portGroupName string
	if policyType == knet.PolicyTypeIngress {
		if oc.portGroupIngressDeny != "" {
			return
		}
		portGroupName = "ingressDefaultDeny"
	} else if policyType == knet.PolicyTypeEgress {
		if oc.portGroupEgressDeny != "" {
			return
		}
		portGroupName = "egressDefaultDeny"
	}
	portGroupUUID, err := oc.createPortGroup(portGroupName, portGroupName)
	if err != nil {
		logrus.Errorf("Failed to create port_group for %s (%v)", portGroupName, err)
		return
	}
	err = oc.addACLDenyPortGroup(portGroupUUID, portGroupName, defaultDenyPriority, policyType)
	if err != nil {
		logrus.Errorf("Failed to create default deny port group %v", err)
		return
	}
	if policyType == knet.PolicyTypeIngress {
		oc.portGroupIngressDeny = portGroupUUID
	} else if policyType == knet.PolicyTypeEgress {
		oc.portGroupEgressDeny = portGroupUUID
	}
}
func (oc *Controller) localPodAddDefaultDeny(policy *knet.NetworkPolicy, logicalPort string) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	oc.lspMutex.Lock()
	defer oc.lspMutex.Unlock()
	oc.createDefaultDenyPortGroup(knet.PolicyTypeIngress)
	oc.createDefaultDenyPortGroup(knet.PolicyTypeEgress)
	if !(len(policy.Spec.PolicyTypes) == 1 && policy.Spec.PolicyTypes[0] == knet.PolicyTypeEgress) {
		if oc.lspIngressDenyCache[logicalPort] == 0 {
			oc.addToACLDeny(oc.portGroupIngressDeny, logicalPort)
		}
		oc.lspIngressDenyCache[logicalPort]++
	}
	if (len(policy.Spec.PolicyTypes) == 1 && policy.Spec.PolicyTypes[0] == knet.PolicyTypeEgress) || len(policy.Spec.Egress) > 0 || len(policy.Spec.PolicyTypes) == 2 {
		if oc.lspEgressDenyCache[logicalPort] == 0 {
			oc.addToACLDeny(oc.portGroupEgressDeny, logicalPort)
		}
		oc.lspEgressDenyCache[logicalPort]++
	}
}
func (oc *Controller) localPodDelDefaultDeny(policy *knet.NetworkPolicy, logicalPort string) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	oc.lspMutex.Lock()
	defer oc.lspMutex.Unlock()
	if !(len(policy.Spec.PolicyTypes) == 1 && policy.Spec.PolicyTypes[0] == knet.PolicyTypeEgress) {
		if oc.lspIngressDenyCache[logicalPort] > 0 {
			oc.lspIngressDenyCache[logicalPort]--
			if oc.lspIngressDenyCache[logicalPort] == 0 {
				oc.deleteFromACLDeny(oc.portGroupIngressDeny, logicalPort)
			}
		}
	}
	if (len(policy.Spec.PolicyTypes) == 1 && policy.Spec.PolicyTypes[0] == knet.PolicyTypeEgress) || len(policy.Spec.Egress) > 0 || len(policy.Spec.PolicyTypes) == 2 {
		if oc.lspEgressDenyCache[logicalPort] > 0 {
			oc.lspEgressDenyCache[logicalPort]--
			if oc.lspEgressDenyCache[logicalPort] == 0 {
				oc.deleteFromACLDeny(oc.portGroupEgressDeny, logicalPort)
			}
		}
	}
}
func (oc *Controller) handleLocalPodSelectorAddFunc(policy *knet.NetworkPolicy, np *namespacePolicy, obj interface{}) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	pod := obj.(*kapi.Pod)
	ipAddress := oc.getIPFromOvnAnnotation(pod.Annotations["ovn"])
	if ipAddress == "" {
		return
	}
	logicalSwitch := pod.Spec.NodeName
	if logicalSwitch == "" {
		return
	}
	logicalPort := fmt.Sprintf("%s_%s", pod.Namespace, pod.Name)
	logicalPortUUID := oc.getLogicalPortUUID(logicalPort)
	if logicalPortUUID == "" {
		return
	}
	np.Lock()
	defer np.Unlock()
	if np.deleted {
		return
	}
	if np.localPods[logicalPort] {
		return
	}
	oc.localPodAddDefaultDeny(policy, logicalPort)
	if np.portGroupUUID == "" {
		return
	}
	_, stderr, err := util.RunOVNNbctl("--if-exists", "remove", "port_group", np.portGroupUUID, "ports", logicalPortUUID, "--", "add", "port_group", np.portGroupUUID, "ports", logicalPortUUID)
	if err != nil {
		logrus.Errorf("Failed to add logicalPort %s to portGroup %s "+"stderr: %q (%v)", logicalPort, np.portGroupUUID, stderr, err)
	}
	np.localPods[logicalPort] = true
}
func (oc *Controller) handleLocalPodSelectorDelFunc(policy *knet.NetworkPolicy, np *namespacePolicy, obj interface{}) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	pod := obj.(*kapi.Pod)
	logicalSwitch := pod.Spec.NodeName
	if logicalSwitch == "" {
		return
	}
	logicalPort := fmt.Sprintf("%s_%s", pod.Namespace, pod.Name)
	logicalPortUUID := oc.getLogicalPortUUID(logicalPort)
	np.Lock()
	defer np.Unlock()
	if np.deleted {
		return
	}
	if !np.localPods[logicalPort] {
		return
	}
	delete(np.localPods, logicalPort)
	oc.localPodDelDefaultDeny(policy, logicalPort)
	if logicalPortUUID == "" || np.portGroupUUID == "" {
		return
	}
	_, stderr, err := util.RunOVNNbctl("--if-exists", "remove", "port_group", np.portGroupUUID, "ports", logicalPortUUID)
	if err != nil {
		logrus.Errorf("Failed to delete logicalPort %s from portGroup %s "+"stderr: %q (%v)", logicalPort, np.portGroupUUID, stderr, err)
	}
}
func (oc *Controller) handleLocalPodSelector(policy *knet.NetworkPolicy, np *namespacePolicy) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	h, err := oc.watchFactory.AddFilteredPodHandler(policy.Namespace, &policy.Spec.PodSelector, cache.ResourceEventHandlerFuncs{AddFunc: func(obj interface{}) {
		oc.handleLocalPodSelectorAddFunc(policy, np, obj)
	}, DeleteFunc: func(obj interface{}) {
		oc.handleLocalPodSelectorDelFunc(policy, np, obj)
	}, UpdateFunc: func(oldObj, newObj interface{}) {
		oc.handleLocalPodSelectorAddFunc(policy, np, newObj)
	}}, nil)
	if err != nil {
		logrus.Errorf("error watching local pods for policy %s in namespace %s: %v", policy.Name, policy.Namespace, err)
		return
	}
	np.podHandlerList = append(np.podHandlerList, h)
}
func (oc *Controller) handlePeerNamespaceAndPodSelector(policy *knet.NetworkPolicy, namespaceSelector *metav1.LabelSelector, podSelector *metav1.LabelSelector, addressSet string, addressMap map[string]bool, gress *gressPolicy, np *namespacePolicy) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	namespaceHandler, err := oc.watchFactory.AddFilteredNamespaceHandler("", namespaceSelector, cache.ResourceEventHandlerFuncs{AddFunc: func(obj interface{}) {
		namespace := obj.(*kapi.Namespace)
		np.Lock()
		defer np.Unlock()
		if np.deleted {
			return
		}
		podHandler, err := oc.watchFactory.AddFilteredPodHandler(namespace.Name, podSelector, cache.ResourceEventHandlerFuncs{AddFunc: func(obj interface{}) {
			oc.handlePeerPodSelectorAddUpdate(policy, np, addressMap, addressSet, obj)
		}, DeleteFunc: func(obj interface{}) {
			oc.handlePeerPodSelectorDelete(policy, np, addressMap, addressSet, obj)
		}, UpdateFunc: func(oldObj, newObj interface{}) {
			oc.handlePeerPodSelectorAddUpdate(policy, np, addressMap, addressSet, newObj)
		}}, nil)
		if err != nil {
			logrus.Errorf("error watching pods in namespace %s for policy %s: %v", namespace.Name, policy.Name, err)
			return
		}
		np.podHandlerList = append(np.podHandlerList, podHandler)
	}, DeleteFunc: func(obj interface{}) {
		return
	}, UpdateFunc: func(oldObj, newObj interface{}) {
		return
	}}, nil)
	if err != nil {
		logrus.Errorf("error watching namespaces for policy %s: %v", policy.Name, err)
		return
	}
	np.nsHandlerList = append(np.nsHandlerList, namespaceHandler)
}
func (oc *Controller) handlePeerPodSelectorAddUpdate(policy *knet.NetworkPolicy, np *namespacePolicy, addressMap map[string]bool, addressSet string, obj interface{}) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	pod := obj.(*kapi.Pod)
	ipAddress := oc.getIPFromOvnAnnotation(pod.Annotations["ovn"])
	if ipAddress == "" || addressMap[ipAddress] {
		return
	}
	np.Lock()
	defer np.Unlock()
	if np.deleted {
		return
	}
	addressMap[ipAddress] = true
	addresses := make([]string, 0, len(addressMap))
	for k := range addressMap {
		addresses = append(addresses, k)
	}
	oc.setAddressSet(addressSet, addresses)
}
func (oc *Controller) handlePeerPodSelectorDelete(policy *knet.NetworkPolicy, np *namespacePolicy, addressMap map[string]bool, addressSet string, obj interface{}) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	pod := obj.(*kapi.Pod)
	ipAddress := oc.getIPFromOvnAnnotation(pod.Annotations["ovn"])
	if ipAddress == "" {
		return
	}
	np.Lock()
	defer np.Unlock()
	if np.deleted {
		return
	}
	if !addressMap[ipAddress] {
		return
	}
	delete(addressMap, ipAddress)
	addresses := make([]string, 0, len(addressMap))
	for k := range addressMap {
		addresses = append(addresses, k)
	}
	oc.setAddressSet(addressSet, addresses)
}
func (oc *Controller) handlePeerPodSelector(policy *knet.NetworkPolicy, podSelector *metav1.LabelSelector, addressSet string, addressMap map[string]bool, np *namespacePolicy) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	h, err := oc.watchFactory.AddFilteredPodHandler(policy.Namespace, podSelector, cache.ResourceEventHandlerFuncs{AddFunc: func(obj interface{}) {
		oc.handlePeerPodSelectorAddUpdate(policy, np, addressMap, addressSet, obj)
	}, DeleteFunc: func(obj interface{}) {
		oc.handlePeerPodSelectorDelete(policy, np, addressMap, addressSet, obj)
	}, UpdateFunc: func(oldObj, newObj interface{}) {
		oc.handlePeerPodSelectorAddUpdate(policy, np, addressMap, addressSet, newObj)
	}}, nil)
	if err != nil {
		logrus.Errorf("error watching peer pods for policy %s in namespace %s: %v", policy.Name, policy.Namespace, err)
		return
	}
	np.podHandlerList = append(np.podHandlerList, h)
}
func (oc *Controller) handlePeerNamespaceSelectorModify(gress *gressPolicy, np *namespacePolicy, oldl3Match, newl3Match string) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	var lportMatch string
	if gress.policyType == knet.PolicyTypeIngress {
		lportMatch = fmt.Sprintf("outport == @%s", np.portGroupName)
	} else {
		lportMatch = fmt.Sprintf("inport == @%s", np.portGroupName)
	}
	if len(gress.portPolicies) == 0 {
		oldMatch := fmt.Sprintf("match=\"%s && %s\"", oldl3Match, lportMatch)
		newMatch := fmt.Sprintf("match=\"%s && %s\"", newl3Match, lportMatch)
		oc.modifyACLAllow(np.namespace, np.name, oldMatch, newMatch, gress.idx, gress.policyType)
	}
	for _, port := range gress.portPolicies {
		l4Match, err := port.getL4Match()
		if err != nil {
			continue
		}
		oldMatch := fmt.Sprintf("match=\"%s && %s && %s\"", oldl3Match, l4Match, lportMatch)
		newMatch := fmt.Sprintf("match=\"%s && %s && %s\"", newl3Match, l4Match, lportMatch)
		oc.modifyACLAllow(np.namespace, np.name, oldMatch, newMatch, gress.idx, gress.policyType)
	}
}
func (oc *Controller) handlePeerNamespaceSelector(policy *knet.NetworkPolicy, namespaceSelector *metav1.LabelSelector, gress *gressPolicy, np *namespacePolicy) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	h, err := oc.watchFactory.AddFilteredNamespaceHandler("", namespaceSelector, cache.ResourceEventHandlerFuncs{AddFunc: func(obj interface{}) {
		namespace := obj.(*kapi.Namespace)
		np.Lock()
		defer np.Unlock()
		if np.deleted {
			return
		}
		hashedAddressSet := hashedAddressSet(namespace.Name)
		oldL3Match, newL3Match, added := gress.addAddressSet(hashedAddressSet)
		if added {
			oc.handlePeerNamespaceSelectorModify(gress, np, oldL3Match, newL3Match)
		}
	}, DeleteFunc: func(obj interface{}) {
		namespace := obj.(*kapi.Namespace)
		np.Lock()
		defer np.Unlock()
		if np.deleted {
			return
		}
		hashedAddressSet := hashedAddressSet(namespace.Name)
		oldL3Match, newL3Match, removed := gress.delAddressSet(hashedAddressSet)
		if removed {
			oc.handlePeerNamespaceSelectorModify(gress, np, oldL3Match, newL3Match)
		}
	}, UpdateFunc: func(oldObj, newObj interface{}) {
		return
	}}, nil)
	if err != nil {
		logrus.Errorf("error watching namespaces for policy %s: %v", policy.Name, err)
		return
	}
	np.nsHandlerList = append(np.nsHandlerList, h)
}
func (oc *Controller) addNetworkPolicyPortGroup(policy *knet.NetworkPolicy) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	logrus.Infof("Adding network policy %s in namespace %s", policy.Name, policy.Namespace)
	if oc.namespacePolicies[policy.Namespace] != nil && oc.namespacePolicies[policy.Namespace][policy.Name] != nil {
		return
	}
	err := oc.waitForNamespaceEvent(policy.Namespace)
	if err != nil {
		logrus.Errorf("failed to wait for namespace %s event (%v)", policy.Namespace, err)
		return
	}
	np := &namespacePolicy{}
	np.name = policy.Name
	np.namespace = policy.Namespace
	np.ingressPolicies = make([]*gressPolicy, 0)
	np.egressPolicies = make([]*gressPolicy, 0)
	np.podHandlerList = make([]*factory.Handler, 0)
	np.nsHandlerList = make([]*factory.Handler, 0)
	np.localPods = make(map[string]bool)
	readableGroupName := fmt.Sprintf("%s_%s", policy.Namespace, policy.Name)
	np.portGroupName = hashedPortGroup(readableGroupName)
	np.portGroupUUID, err = oc.createPortGroup(readableGroupName, np.portGroupName)
	if err != nil {
		logrus.Errorf("Failed to create port_group for network policy %s in "+"namespace %s", policy.Name, policy.Namespace)
		return
	}
	for i, ingressJSON := range policy.Spec.Ingress {
		logrus.Debugf("Network policy ingress is %+v", ingressJSON)
		ingress := newGressPolicy(knet.PolicyTypeIngress, i)
		for _, portJSON := range ingressJSON.Ports {
			ingress.addPortPolicy(&portJSON)
		}
		hashedLocalAddressSet := ""
		peerPodAddressMap := make(map[string]bool)
		if len(ingressJSON.From) != 0 {
			localPeerPods := fmt.Sprintf("%s.%s.%s.%d", policy.Namespace, policy.Name, "ingress", i)
			hashedLocalAddressSet = hashedAddressSet(localPeerPods)
			oc.createAddressSet(localPeerPods, hashedLocalAddressSet, nil)
			ingress.addAddressSet(hashedLocalAddressSet)
		}
		for _, fromJSON := range ingressJSON.From {
			if fromJSON.IPBlock != nil {
				ingress.addIPBlock(fromJSON.IPBlock)
			}
		}
		oc.localPodAddACL(np, ingress)
		for _, fromJSON := range ingressJSON.From {
			if fromJSON.NamespaceSelector != nil && fromJSON.PodSelector != nil {
				oc.handlePeerNamespaceAndPodSelector(policy, fromJSON.NamespaceSelector, fromJSON.PodSelector, hashedLocalAddressSet, peerPodAddressMap, ingress, np)
			} else if fromJSON.NamespaceSelector != nil {
				oc.handlePeerNamespaceSelector(policy, fromJSON.NamespaceSelector, ingress, np)
			} else if fromJSON.PodSelector != nil {
				oc.handlePeerPodSelector(policy, fromJSON.PodSelector, hashedLocalAddressSet, peerPodAddressMap, np)
			}
		}
		np.ingressPolicies = append(np.ingressPolicies, ingress)
	}
	for i, egressJSON := range policy.Spec.Egress {
		logrus.Debugf("Network policy egress is %+v", egressJSON)
		egress := newGressPolicy(knet.PolicyTypeEgress, i)
		for _, portJSON := range egressJSON.Ports {
			egress.addPortPolicy(&portJSON)
		}
		hashedLocalAddressSet := ""
		peerPodAddressMap := make(map[string]bool)
		if len(egressJSON.To) != 0 {
			localPeerPods := fmt.Sprintf("%s.%s.%s.%d", policy.Namespace, policy.Name, "egress", i)
			hashedLocalAddressSet = hashedAddressSet(localPeerPods)
			oc.createAddressSet(localPeerPods, hashedLocalAddressSet, nil)
			egress.addAddressSet(hashedLocalAddressSet)
		}
		for _, toJSON := range egressJSON.To {
			if toJSON.IPBlock != nil {
				egress.addIPBlock(toJSON.IPBlock)
			}
		}
		oc.localPodAddACL(np, egress)
		for _, toJSON := range egressJSON.To {
			if toJSON.NamespaceSelector != nil && toJSON.PodSelector != nil {
				oc.handlePeerNamespaceAndPodSelector(policy, toJSON.NamespaceSelector, toJSON.PodSelector, hashedLocalAddressSet, peerPodAddressMap, egress, np)
			} else if toJSON.NamespaceSelector != nil {
				go oc.handlePeerNamespaceSelector(policy, toJSON.NamespaceSelector, egress, np)
			} else if toJSON.PodSelector != nil {
				oc.handlePeerPodSelector(policy, toJSON.PodSelector, hashedLocalAddressSet, peerPodAddressMap, np)
			}
		}
		np.egressPolicies = append(np.egressPolicies, egress)
	}
	oc.namespacePolicies[policy.Namespace][policy.Name] = np
	oc.handleLocalPodSelector(policy, np)
	return
}
func (oc *Controller) deleteNetworkPolicyPortGroup(policy *knet.NetworkPolicy) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	logrus.Infof("Deleting network policy %s in namespace %s", policy.Name, policy.Namespace)
	if oc.namespacePolicies[policy.Namespace] == nil || oc.namespacePolicies[policy.Namespace][policy.Name] == nil {
		logrus.Errorf("Delete network policy %s in namespace %s "+"received without getting a create event", policy.Name, policy.Namespace)
		return
	}
	np := oc.namespacePolicies[policy.Namespace][policy.Name]
	np.Lock()
	defer np.Unlock()
	np.deleted = true
	for i := range np.ingressPolicies {
		localPeerPods := fmt.Sprintf("%s.%s.%s.%d", policy.Namespace, policy.Name, "ingress", i)
		hashedAddressSet := hashedAddressSet(localPeerPods)
		oc.deleteAddressSet(hashedAddressSet)
	}
	for i := range np.egressPolicies {
		localPeerPods := fmt.Sprintf("%s.%s.%s.%d", policy.Namespace, policy.Name, "egress", i)
		hashedAddressSet := hashedAddressSet(localPeerPods)
		oc.deleteAddressSet(hashedAddressSet)
	}
	oc.shutdownHandlers(np)
	for logicalPort := range np.localPods {
		oc.localPodDelDefaultDeny(policy, logicalPort)
	}
	oc.deletePortGroup(np.portGroupName)
	oc.namespacePolicies[policy.Namespace][policy.Name] = nil
	return
}
