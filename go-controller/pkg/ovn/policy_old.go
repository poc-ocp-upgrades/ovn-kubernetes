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

func (oc *Controller) syncNetworkPoliciesOld(networkPolicies []interface{}) {
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
			oc.deleteAclsPolicyOld(namespaceName, policyName)
			oc.deleteAddressSet(hashedAddressSet(addrSetName))
		}
	})
	if err != nil {
		logrus.Errorf("Error in syncing network policies: %v", err)
	}
}
func (oc *Controller) addACLAllowOld(namespace, policy, logicalSwitch, logicalPort, match, l4Match string, ipBlockCidr bool, gressNum int, policyType knet.PolicyType) {
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
	uuid, stderr, err := util.RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "ACL", fmt.Sprintf("external-ids:l4Match=\"%s\"", l4Match), fmt.Sprintf("external-ids:ipblock_cidr=%t", ipBlockCidr), fmt.Sprintf("external-ids:namespace=%s", namespace), fmt.Sprintf("external-ids:policy=%s", policy), fmt.Sprintf("external-ids:%s_num=%d", policyType, gressNum), fmt.Sprintf("external-ids:policy_type=%s", policyType), fmt.Sprintf("external-ids:logical_switch=%s", logicalSwitch), fmt.Sprintf("external-ids:logical_port=%s", logicalPort))
	if err != nil {
		logrus.Errorf("find failed to get the allow rule for "+"namespace=%s, logical_port=%s, stderr: %q (%v)", namespace, logicalPort, stderr, err)
		return
	}
	if uuid != "" {
		return
	}
	_, stderr, err = util.RunOVNNbctl("--id=@acl", "create", "acl", fmt.Sprintf("priority=%s", defaultAllowPriority), fmt.Sprintf("direction=%s", direction), match, fmt.Sprintf("action=%s", action), fmt.Sprintf("external-ids:l4Match=\"%s\"", l4Match), fmt.Sprintf("external-ids:ipblock_cidr=%t", ipBlockCidr), fmt.Sprintf("external-ids:namespace=%s", namespace), fmt.Sprintf("external-ids:policy=%s", policy), fmt.Sprintf("external-ids:%s_num=%d", policyType, gressNum), fmt.Sprintf("external-ids:policy_type=%s", policyType), fmt.Sprintf("external-ids:logical_switch=%s", logicalSwitch), fmt.Sprintf("external-ids:logical_port=%s", logicalPort), "--", "add", "logical_switch", logicalSwitch, "acls", "@acl")
	if err != nil {
		logrus.Errorf("failed to create the allow-from rule for "+"namespace=%s, logical_port=%s, stderr: %q (%v)", namespace, logicalPort, stderr, err)
		return
	}
}
func (oc *Controller) modifyACLAllowOld(namespace, policy, logicalPort, oldMatch string, newMatch string, gressNum int, policyType knet.PolicyType) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	uuid, stderr, err := util.RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "ACL", oldMatch, fmt.Sprintf("external-ids:namespace=%s", namespace), fmt.Sprintf("external-ids:policy=%s", policy), fmt.Sprintf("external-ids:%s_num=%d", policyType, gressNum), fmt.Sprintf("external-ids:policy_type=%s", policyType), fmt.Sprintf("external-ids:logical_port=%s", logicalPort))
	if err != nil {
		logrus.Errorf("find failed to get the allow rule for "+"namespace=%s, logical_port=%s, stderr: %q (%v)", namespace, logicalPort, stderr, err)
		return
	}
	if uuid != "" {
		_, stderr, err = util.RunOVNNbctl("set", "acl", uuid, fmt.Sprintf("%s", newMatch))
		if err != nil {
			logrus.Errorf("failed to modify the allow-from rule for "+"namespace=%s, logical_port=%s, stderr: %q (%v)", namespace, logicalPort, stderr, err)
		}
		return
	}
}
func (oc *Controller) deleteACLAllowOld(namespace, policy, logicalSwitch, logicalPort, match, l4Match string, ipBlockCidr bool, gressNum int, policyType knet.PolicyType) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	uuid, stderr, err := util.RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "ACL", fmt.Sprintf("external-ids:l4Match=\"%s\"", l4Match), fmt.Sprintf("external-ids:ipblock_cidr=%t", ipBlockCidr), fmt.Sprintf("external-ids:namespace=%s", namespace), fmt.Sprintf("external-ids:policy=%s", policy), fmt.Sprintf("external-ids:%s_num=%d", policyType, gressNum), fmt.Sprintf("external-ids:policy_type=%s", policyType), fmt.Sprintf("external-ids:logical_switch=%s", logicalSwitch), fmt.Sprintf("external-ids:logical_port=%s", logicalPort))
	if err != nil {
		logrus.Errorf("find failed to get the allow rule for "+"namespace=%s, logical_port=%s, stderr: %q, (%v)", namespace, logicalPort, stderr, err)
		return
	}
	if uuid == "" {
		logrus.Infof("deleteACLAllow: returning because find returned empty")
		return
	}
	_, stderr, err = util.RunOVNNbctl("remove", "logical_switch", logicalSwitch, "acls", uuid)
	if err != nil {
		logrus.Errorf("remove failed to delete the allow-from rule for "+"namespace=%s, logical_port=%s, stderr: %q (%v)", namespace, logicalPort, stderr, err)
		return
	}
}
func (oc *Controller) addIPBlockACLDenyOld(namespace, policy, logicalSwitch, logicalPort, except, priority string, policyType knet.PolicyType) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	var match, l3Match, direction, lportMatch string
	direction = toLport
	if policyType == knet.PolicyTypeIngress {
		lportMatch = fmt.Sprintf("outport == \\\"%s\\\"", logicalPort)
		l3Match = fmt.Sprintf("ip4.src == %s", except)
		match = fmt.Sprintf("match=\"%s && %s\"", lportMatch, l3Match)
	} else {
		lportMatch = fmt.Sprintf("inport == \\\"%s\\\"", logicalPort)
		l3Match = fmt.Sprintf("ip4.dst == %s", except)
		match = fmt.Sprintf("match=\"%s && %s\"", lportMatch, l3Match)
	}
	uuid, stderr, err := util.RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "ACL", match, "action=drop", fmt.Sprintf("external-ids:ipblock-deny-policy-type=%s", policyType), fmt.Sprintf("external-ids:namespace=%s", namespace), fmt.Sprintf("external-ids:policy=%s", policy), fmt.Sprintf("external-ids:logical_switch=%s", logicalSwitch), fmt.Sprintf("external-ids:logical_port=%s", logicalPort))
	if err != nil {
		logrus.Errorf("find failed to get the default deny rule for "+"namespace=%s, logical_port=%s stderr: %q, (%v)", namespace, logicalPort, stderr, err)
		return
	}
	if uuid != "" {
		return
	}
	_, stderr, err = util.RunOVNNbctl("--id=@acl", "create", "acl", fmt.Sprintf("priority=%s", priority), fmt.Sprintf("direction=%s", direction), match, "action=drop", fmt.Sprintf("external-ids:ipblock-deny-policy-type=%s", policyType), fmt.Sprintf("external-ids:namespace=%s", namespace), fmt.Sprintf("external-ids:policy=%s", policy), fmt.Sprintf("external-ids:logical_switch=%s", logicalSwitch), fmt.Sprintf("external-ids:logical_port=%s", logicalPort), "--", "add", "logical_switch", logicalSwitch, "acls", "@acl")
	if err != nil {
		logrus.Errorf("error executing create ACL command, stderr: %q, %+v", stderr, err)
	}
	return
}
func (oc *Controller) deleteIPBlockACLDenyOld(namespace, policy, logicalSwitch, logicalPort, except string, policyType knet.PolicyType) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	var match, lportMatch, l3Match string
	if policyType == knet.PolicyTypeIngress {
		lportMatch = fmt.Sprintf("outport == \\\"%s\\\"", logicalPort)
		l3Match = fmt.Sprintf("ip4.src == %s", except)
		match = fmt.Sprintf("match=\"%s && %s\"", lportMatch, l3Match)
	} else {
		lportMatch = fmt.Sprintf("inport == \\\"%s\\\"", logicalPort)
		l3Match = fmt.Sprintf("ip4.dst == %s", except)
		match = fmt.Sprintf("match=\"%s && %s\"", lportMatch, l3Match)
	}
	uuid, stderr, err := util.RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "ACL", match, "action=drop", fmt.Sprintf("external-ids:ipblock-deny-policy-type=%s", policyType), fmt.Sprintf("external-ids:namespace=%s", namespace), fmt.Sprintf("external-ids:policy=%s", policy), fmt.Sprintf("external-ids:logical_switch=%s", logicalSwitch), fmt.Sprintf("external-ids:logical_port=%s", logicalPort))
	if err != nil {
		logrus.Errorf("find failed to get the default deny rule for "+"namespace=%s, logical_port=%s, stderr: %q. (%v)", namespace, logicalPort, stderr, err)
		return
	}
	if uuid == "" {
		return
	}
	_, stderr, err = util.RunOVNNbctl("remove", "logical_switch", logicalSwitch, "acls", uuid)
	if err != nil {
		logrus.Errorf("remove failed to delete the deny rule for "+"namespace=%s, logical_port=%s, stderr: %q (%v)", namespace, logicalPort, stderr, err)
		return
	}
	return
}
func (oc *Controller) addACLDenyOld(namespace, logicalSwitch, logicalPort, priority string, policyType knet.PolicyType) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	var match, direction string
	direction = toLport
	if policyType == knet.PolicyTypeIngress {
		match = fmt.Sprintf("match=\"outport == \\\"%s\\\"\"", logicalPort)
	} else {
		match = fmt.Sprintf("match=\"inport == \\\"%s\\\"\"", logicalPort)
	}
	uuid, stderr, err := util.RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "ACL", match, "action=drop", fmt.Sprintf("external-ids:default-deny-policy-type=%s", policyType), fmt.Sprintf("external-ids:namespace=%s", namespace), fmt.Sprintf("external-ids:logical_switch=%s", logicalSwitch), fmt.Sprintf("external-ids:logical_port=%s", logicalPort))
	if err != nil {
		logrus.Errorf("find failed to get the default deny rule for "+"namespace=%s, logical_port=%s, stderr: %q (%v)", namespace, logicalPort, stderr, err)
		return
	}
	if uuid != "" {
		return
	}
	_, stderr, err = util.RunOVNNbctl("--id=@acl", "create", "acl", fmt.Sprintf("priority=%s", priority), fmt.Sprintf("direction=%s", direction), match, "action=drop", fmt.Sprintf("external-ids:default-deny-policy-type=%s", policyType), fmt.Sprintf("external-ids:namespace=%s", namespace), fmt.Sprintf("external-ids:logical_switch=%s", logicalSwitch), fmt.Sprintf("external-ids:logical_port=%s", logicalPort), "--", "add", "logical_switch", logicalSwitch, "acls", "@acl")
	if err != nil {
		logrus.Errorf("error executing create ACL command, stderr: %q, %+v", stderr, err)
	}
	return
}
func (oc *Controller) deleteACLDenyOld(namespace, logicalSwitch, logicalPort string, policyType knet.PolicyType) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	var match string
	if policyType == knet.PolicyTypeIngress {
		match = fmt.Sprintf("match=\"outport == \\\"%s\\\"\"", logicalPort)
	} else {
		match = fmt.Sprintf("match=\"inport == \\\"%s\\\"\"", logicalPort)
	}
	uuid, stderr, err := util.RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "ACL", match, "action=drop", fmt.Sprintf("external-ids:default-deny-policy-type=%s", policyType), fmt.Sprintf("external-ids:namespace=%s", namespace), fmt.Sprintf("external-ids:logical_switch=%s", logicalSwitch), fmt.Sprintf("external-ids:logical_port=%s", logicalPort))
	if err != nil {
		logrus.Errorf("find failed to get the default deny rule for "+"namespace=%s, logical_port=%s, stderr: %q, (%v)", namespace, logicalPort, stderr, err)
		return
	}
	if uuid == "" {
		return
	}
	_, stderr, err = util.RunOVNNbctl("remove", "logical_switch", logicalSwitch, "acls", uuid)
	if err != nil {
		logrus.Errorf("remove failed to delete the deny rule for "+"namespace=%s, logical_port=%s, stderr: %q (%v)", namespace, logicalPort, stderr, err)
		return
	}
	return
}
func (oc *Controller) deleteAclsPolicyOld(namespace, policy string) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	uuids, stderr, err := util.RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "ACL", fmt.Sprintf("external-ids:namespace=%s", namespace), fmt.Sprintf("external-ids:policy=%s", policy))
	if err != nil {
		logrus.Errorf("find failed to get the allow rule for "+"namespace=%s, policy=%s, stderr: %q (%v)", namespace, policy, stderr, err)
		return
	}
	if uuids == "" {
		logrus.Debugf("deleteAclsPolicy: returning because find " + "returned no ACLs")
		return
	}
	uuidSlice := strings.Fields(uuids)
	for _, uuid := range uuidSlice {
		logicalSwitch, stderr, err := util.RunOVNNbctl("--data=bare", "--no-heading", "--columns=_uuid", "find", "logical_switch", fmt.Sprintf("acls{>=}%s", uuid))
		if err != nil {
			logrus.Errorf("find failed to get the logical_switch of acl"+"uuid=%s, stderr: %q (%v)", uuid, stderr, err)
			continue
		}
		if logicalSwitch == "" {
			continue
		}
		_, stderr, err = util.RunOVNNbctl("remove", "logical_switch", logicalSwitch, "acls", uuid)
		if err != nil {
			logrus.Errorf("remove failed to delete the allow-from rule %s for"+" namespace=%s, policy=%s, logical_switch=%s, stderr: %q (%v)", uuid, namespace, policy, logicalSwitch, stderr, err)
			continue
		}
	}
}
func (oc *Controller) localPodAddOrDelACLOld(addDel string, policy *knet.NetworkPolicy, pod *kapi.Pod, gress *gressPolicy, logicalSwitch string) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	logicalPort := fmt.Sprintf("%s_%s", pod.Namespace, pod.Name)
	l3Match := gress.getL3MatchFromAddressSet()
	var lportMatch, cidrMatch string
	if gress.policyType == knet.PolicyTypeIngress {
		lportMatch = fmt.Sprintf("outport == \\\"%s\\\"", logicalPort)
	} else {
		lportMatch = fmt.Sprintf("inport == \\\"%s\\\"", logicalPort)
	}
	if len(gress.ipBlockCidr) > 0 && len(gress.ipBlockExcept) > 0 {
		except := fmt.Sprintf("{%s}", strings.Join(gress.ipBlockExcept, ", "))
		if addDel == addACL {
			oc.addIPBlockACLDenyOld(policy.Namespace, policy.Name, logicalSwitch, logicalPort, except, ipBlockDenyPriority, gress.policyType)
		} else {
			oc.deleteIPBlockACLDenyOld(policy.Namespace, policy.Name, logicalSwitch, logicalPort, except, gress.policyType)
		}
	}
	if len(gress.portPolicies) == 0 {
		match := fmt.Sprintf("match=\"%s && %s\"", l3Match, lportMatch)
		l4Match := noneMatch
		if addDel == addACL {
			if len(gress.ipBlockCidr) > 0 {
				cidrMatch = gress.getMatchFromIPBlock(lportMatch, l4Match)
				oc.addACLAllowOld(policy.Namespace, policy.Name, logicalSwitch, logicalPort, cidrMatch, l4Match, true, gress.idx, gress.policyType)
			}
			oc.addACLAllowOld(policy.Namespace, policy.Name, logicalSwitch, logicalPort, match, l4Match, false, gress.idx, gress.policyType)
		} else {
			if len(gress.ipBlockCidr) > 0 {
				cidrMatch = gress.getMatchFromIPBlock(lportMatch, l4Match)
				oc.deleteACLAllowOld(policy.Namespace, policy.Name, logicalSwitch, logicalPort, cidrMatch, l4Match, true, gress.idx, gress.policyType)
			}
			oc.deleteACLAllowOld(policy.Namespace, policy.Name, logicalSwitch, logicalPort, match, l4Match, false, gress.idx, gress.policyType)
		}
	}
	for _, port := range gress.portPolicies {
		l4Match, err := port.getL4Match()
		if err != nil {
			continue
		}
		match := fmt.Sprintf("match=\"%s && %s && %s\"", l3Match, l4Match, lportMatch)
		if addDel == addACL {
			if len(gress.ipBlockCidr) > 0 {
				cidrMatch = gress.getMatchFromIPBlock(lportMatch, l4Match)
				oc.addACLAllowOld(policy.Namespace, policy.Name, logicalSwitch, logicalPort, cidrMatch, l4Match, true, gress.idx, gress.policyType)
			}
			oc.addACLAllowOld(policy.Namespace, policy.Name, pod.Spec.NodeName, logicalPort, match, l4Match, false, gress.idx, gress.policyType)
		} else {
			if len(gress.ipBlockCidr) > 0 {
				cidrMatch = gress.getMatchFromIPBlock(lportMatch, l4Match)
				oc.deleteACLAllowOld(policy.Namespace, policy.Name, logicalSwitch, logicalPort, cidrMatch, l4Match, true, gress.idx, gress.policyType)
			}
			oc.deleteACLAllowOld(policy.Namespace, policy.Name, pod.Spec.NodeName, logicalPort, match, l4Match, false, gress.idx, gress.policyType)
		}
	}
}
func (oc *Controller) localPodAddDefaultDenyOld(policy *knet.NetworkPolicy, logicalPort, logicalSwitch string) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	oc.lspMutex.Lock()
	if !(len(policy.Spec.PolicyTypes) == 1 && policy.Spec.PolicyTypes[0] == knet.PolicyTypeEgress) {
		if oc.lspIngressDenyCache[logicalPort] == 0 {
			oc.addACLDenyOld(policy.Namespace, logicalSwitch, logicalPort, defaultDenyPriority, knet.PolicyTypeIngress)
		}
		oc.lspIngressDenyCache[logicalPort]++
	}
	if (len(policy.Spec.PolicyTypes) == 1 && policy.Spec.PolicyTypes[0] == knet.PolicyTypeEgress) || len(policy.Spec.Egress) > 0 || len(policy.Spec.PolicyTypes) == 2 {
		if oc.lspEgressDenyCache[logicalPort] == 0 {
			oc.addACLDenyOld(policy.Namespace, logicalSwitch, logicalPort, defaultDenyPriority, knet.PolicyTypeEgress)
		}
		oc.lspEgressDenyCache[logicalPort]++
	}
	oc.lspMutex.Unlock()
}
func (oc *Controller) localPodDelDefaultDenyOld(policy *knet.NetworkPolicy, logicalPort, logicalSwitch string) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	oc.lspMutex.Lock()
	if !(len(policy.Spec.PolicyTypes) == 1 && policy.Spec.PolicyTypes[0] == knet.PolicyTypeEgress) {
		if oc.lspIngressDenyCache[logicalPort] > 0 {
			oc.lspIngressDenyCache[logicalPort]--
			if oc.lspIngressDenyCache[logicalPort] == 0 {
				oc.deleteACLDenyOld(policy.Namespace, logicalSwitch, logicalPort, knet.PolicyTypeIngress)
			}
		}
	}
	if (len(policy.Spec.PolicyTypes) == 1 && policy.Spec.PolicyTypes[0] == knet.PolicyTypeEgress) || len(policy.Spec.Egress) > 0 || len(policy.Spec.PolicyTypes) == 2 {
		if oc.lspEgressDenyCache[logicalPort] > 0 {
			oc.lspEgressDenyCache[logicalPort]--
			if oc.lspEgressDenyCache[logicalPort] == 0 {
				oc.deleteACLDenyOld(policy.Namespace, logicalSwitch, logicalPort, knet.PolicyTypeEgress)
			}
		}
	}
	oc.lspMutex.Unlock()
}
func (oc *Controller) handleLocalPodSelectorAddFuncOld(policy *knet.NetworkPolicy, np *namespacePolicy, obj interface{}) {
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
	np.Lock()
	defer np.Unlock()
	if np.deleted {
		return
	}
	if np.localPods[logicalPort] {
		return
	}
	oc.localPodAddDefaultDenyOld(policy, logicalPort, logicalSwitch)
	for _, ingress := range np.ingressPolicies {
		oc.localPodAddOrDelACLOld(addACL, policy, pod, ingress, logicalSwitch)
	}
	for _, egress := range np.egressPolicies {
		oc.localPodAddOrDelACLOld(addACL, policy, pod, egress, logicalSwitch)
	}
	np.localPods[logicalPort] = true
}
func (oc *Controller) handleLocalPodSelectorDelFuncOld(policy *knet.NetworkPolicy, np *namespacePolicy, obj interface{}) {
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
	np.Lock()
	defer np.Unlock()
	if np.deleted {
		return
	}
	if !np.localPods[logicalPort] {
		return
	}
	delete(np.localPods, logicalPort)
	oc.localPodDelDefaultDenyOld(policy, logicalPort, logicalSwitch)
	for _, ingress := range np.ingressPolicies {
		oc.localPodAddOrDelACLOld(deleteACL, policy, pod, ingress, logicalSwitch)
	}
	for _, egress := range np.egressPolicies {
		oc.localPodAddOrDelACLOld(deleteACL, policy, pod, egress, logicalSwitch)
	}
}
func (oc *Controller) handleLocalPodSelectorOld(policy *knet.NetworkPolicy, np *namespacePolicy) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	h, err := oc.watchFactory.AddFilteredPodHandler(policy.Namespace, &policy.Spec.PodSelector, cache.ResourceEventHandlerFuncs{AddFunc: func(obj interface{}) {
		oc.handleLocalPodSelectorAddFuncOld(policy, np, obj)
	}, DeleteFunc: func(obj interface{}) {
		oc.handleLocalPodSelectorDelFuncOld(policy, np, obj)
	}, UpdateFunc: func(oldObj, newObj interface{}) {
		oc.handleLocalPodSelectorAddFuncOld(policy, np, newObj)
	}}, nil)
	if err != nil {
		logrus.Errorf("error watching local pods for policy %s in namespace %s: %v", policy.Name, policy.Namespace, err)
		return
	}
	np.podHandlerList = append(np.podHandlerList, h)
}
func (oc *Controller) handlePeerPodSelectorAddUpdateOld(policy *knet.NetworkPolicy, np *namespacePolicy, addressMap map[string]bool, addressSet string, obj interface{}) {
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
func (oc *Controller) handlePeerPodSelectorDeleteOld(policy *knet.NetworkPolicy, np *namespacePolicy, addressMap map[string]bool, addressSet string, obj interface{}) {
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
func (oc *Controller) handlePeerPodSelectorOld(policy *knet.NetworkPolicy, podSelector *metav1.LabelSelector, addressSet string, addressMap map[string]bool, np *namespacePolicy) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	h, err := oc.watchFactory.AddFilteredPodHandler(policy.Namespace, podSelector, cache.ResourceEventHandlerFuncs{AddFunc: func(obj interface{}) {
		oc.handlePeerPodSelectorAddUpdateOld(policy, np, addressMap, addressSet, obj)
	}, DeleteFunc: func(obj interface{}) {
		oc.handlePeerPodSelectorDeleteOld(policy, np, addressMap, addressSet, obj)
	}, UpdateFunc: func(oldObj, newObj interface{}) {
		oc.handlePeerPodSelectorAddUpdateOld(policy, np, addressMap, addressSet, newObj)
	}}, nil)
	if err != nil {
		logrus.Errorf("error watching peer pods for policy %s in namespace %s: %v", policy.Name, policy.Namespace, err)
		return
	}
	np.podHandlerList = append(np.podHandlerList, h)
}
func (oc *Controller) handlePeerNamespaceAndPodSelectorOld(policy *knet.NetworkPolicy, namespaceSelector *metav1.LabelSelector, podSelector *metav1.LabelSelector, addressSet string, addressMap map[string]bool, gress *gressPolicy, np *namespacePolicy) {
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
			oc.handlePeerPodSelectorAddUpdateOld(policy, np, addressMap, addressSet, obj)
		}, DeleteFunc: func(obj interface{}) {
			oc.handlePeerPodSelectorDeleteOld(policy, np, addressMap, addressSet, obj)
		}, UpdateFunc: func(oldObj, newObj interface{}) {
			oc.handlePeerPodSelectorAddUpdateOld(policy, np, addressMap, addressSet, newObj)
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
func (oc *Controller) handlePeerNamespaceSelectorModifyOld(gress *gressPolicy, np *namespacePolicy, oldl3Match, newl3Match string) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	for logicalPort := range np.localPods {
		var lportMatch string
		if gress.policyType == knet.PolicyTypeIngress {
			lportMatch = fmt.Sprintf("outport == \\\"%s\\\"", logicalPort)
		} else {
			lportMatch = fmt.Sprintf("inport == \\\"%s\\\"", logicalPort)
		}
		if len(gress.portPolicies) == 0 {
			oldMatch := fmt.Sprintf("match=\"%s && %s\"", oldl3Match, lportMatch)
			newMatch := fmt.Sprintf("match=\"%s && %s\"", newl3Match, lportMatch)
			oc.modifyACLAllowOld(np.namespace, np.name, logicalPort, oldMatch, newMatch, gress.idx, gress.policyType)
		}
		for _, port := range gress.portPolicies {
			l4Match, err := port.getL4Match()
			if err != nil {
				continue
			}
			oldMatch := fmt.Sprintf("match=\"%s && %s && %s\"", oldl3Match, l4Match, lportMatch)
			newMatch := fmt.Sprintf("match=\"%s && %s && %s\"", newl3Match, l4Match, lportMatch)
			oc.modifyACLAllowOld(np.namespace, np.name, logicalPort, oldMatch, newMatch, gress.idx, gress.policyType)
		}
	}
}
func (oc *Controller) handlePeerNamespaceSelectorOld(policy *knet.NetworkPolicy, namespaceSelector *metav1.LabelSelector, gress *gressPolicy, np *namespacePolicy) {
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
			oc.handlePeerNamespaceSelectorModifyOld(gress, np, oldL3Match, newL3Match)
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
			oc.handlePeerNamespaceSelectorModifyOld(gress, np, oldL3Match, newL3Match)
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
func (oc *Controller) addNetworkPolicyOld(policy *knet.NetworkPolicy) {
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
			if fromJSON.NamespaceSelector != nil && fromJSON.PodSelector != nil {
				oc.handlePeerNamespaceAndPodSelectorOld(policy, fromJSON.NamespaceSelector, fromJSON.PodSelector, hashedLocalAddressSet, peerPodAddressMap, ingress, np)
			} else if fromJSON.NamespaceSelector != nil {
				oc.handlePeerNamespaceSelectorOld(policy, fromJSON.NamespaceSelector, ingress, np)
			} else if fromJSON.PodSelector != nil {
				oc.handlePeerPodSelectorOld(policy, fromJSON.PodSelector, hashedLocalAddressSet, peerPodAddressMap, np)
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
			if toJSON.NamespaceSelector != nil && toJSON.PodSelector != nil {
				oc.handlePeerNamespaceAndPodSelectorOld(policy, toJSON.NamespaceSelector, toJSON.PodSelector, hashedLocalAddressSet, peerPodAddressMap, egress, np)
			} else if toJSON.NamespaceSelector != nil {
				oc.handlePeerNamespaceSelectorOld(policy, toJSON.NamespaceSelector, egress, np)
			} else if toJSON.PodSelector != nil {
				oc.handlePeerPodSelectorOld(policy, toJSON.PodSelector, hashedLocalAddressSet, peerPodAddressMap, np)
			}
		}
		np.egressPolicies = append(np.egressPolicies, egress)
	}
	oc.namespacePolicies[policy.Namespace][policy.Name] = np
	oc.handleLocalPodSelectorOld(policy, np)
	return
}
func (oc *Controller) getLogicalSwitchForLogicalPort(logicalPort string) string {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	if oc.logicalPortCache[logicalPort] != "" {
		return oc.logicalPortCache[logicalPort]
	}
	logicalSwitch, stderr, err := util.RunOVNNbctl("get", "logical_switch_port", logicalPort, "external-ids:logical_switch")
	if err != nil {
		logrus.Errorf("Error obtaining logical switch for %s, stderr: %q (%v)", logicalPort, stderr, err)
		return ""
	}
	if logicalSwitch == "" {
		logrus.Errorf("Error obtaining logical switch for %s", logicalPort)
		return ""
	}
	return logicalSwitch
}
func (oc *Controller) deleteNetworkPolicyOld(policy *knet.NetworkPolicy) {
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
		logicalSwitch := oc.getLogicalSwitchForLogicalPort(logicalPort)
		oc.localPodDelDefaultDenyOld(policy, logicalPort, logicalSwitch)
	}
	oc.namespacePolicies[policy.Namespace][policy.Name] = nil
	oc.deleteAclsPolicyOld(policy.Namespace, policy.Name)
	return
}
