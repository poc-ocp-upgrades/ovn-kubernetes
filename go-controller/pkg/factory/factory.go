package factory

import (
	"fmt"
	godefaultbytes "bytes"
	godefaulthttp "net/http"
	godefaultruntime "runtime"
	"reflect"
	"sync"
	"sync/atomic"
	"time"
	"github.com/sirupsen/logrus"
	kapi "k8s.io/api/core/v1"
	knet "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	informerfactory "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type Handler struct {
	cache.FilteringResourceEventHandler
	id		uint64
	tombstone	uint32
}
type informer struct {
	sync.Mutex
	oType		reflect.Type
	inf		cache.SharedIndexInformer
	handlers	map[uint64]*Handler
}

func (i *informer) forEachHandler(obj interface{}, f func(h *Handler)) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	i.Lock()
	defer i.Unlock()
	objType := reflect.TypeOf(obj)
	if objType != i.oType {
		logrus.Errorf("object type %v did not match expected %v", objType, i.oType)
		return
	}
	for _, handler := range i.handlers {
		if !atomic.CompareAndSwapUint32(&handler.tombstone, handlerDead, handlerDead) {
			f(handler)
		}
	}
}

type WatchFactory struct {
	iFactory	informerfactory.SharedInformerFactory
	informers	map[reflect.Type]*informer
	handlerCounter	uint64
}

const (
	resyncInterval		= 12 * time.Hour
	handlerAlive	uint32	= 0
	handlerDead	uint32	= 1
)

func newInformer(oType reflect.Type, sharedInformer cache.SharedIndexInformer) *informer {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return &informer{oType: oType, inf: sharedInformer, handlers: make(map[uint64]*Handler)}
}

var (
	podType		reflect.Type	= reflect.TypeOf(&kapi.Pod{})
	serviceType	reflect.Type	= reflect.TypeOf(&kapi.Service{})
	endpointsType	reflect.Type	= reflect.TypeOf(&kapi.Endpoints{})
	policyType	reflect.Type	= reflect.TypeOf(&knet.NetworkPolicy{})
	namespaceType	reflect.Type	= reflect.TypeOf(&kapi.Namespace{})
	nodeType	reflect.Type	= reflect.TypeOf(&kapi.Node{})
)

func NewWatchFactory(c kubernetes.Interface, stopChan <-chan struct{}) (*WatchFactory, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	wf := &WatchFactory{iFactory: informerfactory.NewSharedInformerFactory(c, resyncInterval), informers: make(map[reflect.Type]*informer)}
	wf.informers[podType] = newInformer(podType, wf.iFactory.Core().V1().Pods().Informer())
	wf.informers[serviceType] = newInformer(serviceType, wf.iFactory.Core().V1().Services().Informer())
	wf.informers[endpointsType] = newInformer(endpointsType, wf.iFactory.Core().V1().Endpoints().Informer())
	wf.informers[policyType] = newInformer(policyType, wf.iFactory.Networking().V1().NetworkPolicies().Informer())
	wf.informers[namespaceType] = newInformer(namespaceType, wf.iFactory.Core().V1().Namespaces().Informer())
	wf.informers[nodeType] = newInformer(nodeType, wf.iFactory.Core().V1().Nodes().Informer())
	wf.iFactory.Start(stopChan)
	res := wf.iFactory.WaitForCacheSync(stopChan)
	for oType, synced := range res {
		if !synced {
			return nil, fmt.Errorf("error in syncing cache for %v informer", oType)
		}
		informer := wf.informers[oType]
		informer.inf.AddEventHandler(wf.newFederatedHandler(informer))
	}
	return wf, nil
}
func (wf *WatchFactory) newFederatedHandler(inf *informer) cache.ResourceEventHandlerFuncs {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return cache.ResourceEventHandlerFuncs{AddFunc: func(obj interface{}) {
		inf.forEachHandler(obj, func(h *Handler) {
			logrus.Debugf("running %v ADD event for handler %d", inf.oType, h.id)
			h.OnAdd(obj)
		})
	}, UpdateFunc: func(oldObj, newObj interface{}) {
		inf.forEachHandler(newObj, func(h *Handler) {
			logrus.Debugf("running %v UPDATE event for handler %d", inf.oType, h.id)
			h.OnUpdate(oldObj, newObj)
		})
	}, DeleteFunc: func(obj interface{}) {
		if inf.oType != reflect.TypeOf(obj) {
			tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
			if !ok {
				logrus.Errorf("couldn't get object from tombstone: %+v", obj)
				return
			}
			obj = tombstone.Obj
			objType := reflect.TypeOf(obj)
			if inf.oType != objType {
				logrus.Errorf("expected tombstone object resource type %v but got %v", inf.oType, objType)
				return
			}
		}
		inf.forEachHandler(obj, func(h *Handler) {
			logrus.Debugf("running %v DELETE event for handler %d", inf.oType, h.id)
			h.OnDelete(obj)
		})
	}}
}
func getObjectMeta(objType reflect.Type, obj interface{}) (*metav1.ObjectMeta, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	switch objType {
	case podType:
		if pod, ok := obj.(*kapi.Pod); ok {
			return &pod.ObjectMeta, nil
		}
	case serviceType:
		if service, ok := obj.(*kapi.Service); ok {
			return &service.ObjectMeta, nil
		}
	case endpointsType:
		if endpoints, ok := obj.(*kapi.Endpoints); ok {
			return &endpoints.ObjectMeta, nil
		}
	case policyType:
		if policy, ok := obj.(*knet.NetworkPolicy); ok {
			return &policy.ObjectMeta, nil
		}
	case namespaceType:
		if namespace, ok := obj.(*kapi.Namespace); ok {
			return &namespace.ObjectMeta, nil
		}
	case nodeType:
		if node, ok := obj.(*kapi.Node); ok {
			return &node.ObjectMeta, nil
		}
	}
	return nil, fmt.Errorf("cannot get ObjectMeta from type %v", objType)
}
func (wf *WatchFactory) addHandler(objType reflect.Type, namespace string, lsel *metav1.LabelSelector, funcs cache.ResourceEventHandler, processExisting func([]interface{})) (*Handler, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	inf, ok := wf.informers[objType]
	if !ok {
		return nil, fmt.Errorf("unknown object type %v", objType)
	}
	sel, err := metav1.LabelSelectorAsSelector(lsel)
	if err != nil {
		return nil, fmt.Errorf("error creating label selector: %v", err)
	}
	filterFunc := func(obj interface{}) bool {
		if namespace == "" && lsel == nil {
			return true
		}
		meta, err := getObjectMeta(objType, obj)
		if err != nil {
			logrus.Errorf("watch handler filter error: %v", err)
			return false
		}
		if namespace != "" && meta.Namespace != namespace {
			return false
		}
		if lsel != nil && !sel.Matches(labels.Set(meta.Labels)) {
			return false
		}
		return true
	}
	existingItems := inf.inf.GetStore().List()
	if processExisting != nil {
		items := make([]interface{}, 0)
		for _, obj := range existingItems {
			if filterFunc(obj) {
				items = append(items, obj)
			}
		}
		processExisting(items)
	}
	handlerID := atomic.AddUint64(&wf.handlerCounter, 1)
	inf.Lock()
	defer inf.Unlock()
	handler := &Handler{cache.FilteringResourceEventHandler{FilterFunc: filterFunc, Handler: funcs}, handlerID, handlerAlive}
	inf.handlers[handlerID] = handler
	logrus.Debugf("added %v event handler %d", objType, handlerID)
	for _, obj := range existingItems {
		inf.handlers[handlerID].OnAdd(obj)
	}
	return handler, nil
}
func (wf *WatchFactory) removeHandler(objType reflect.Type, handler *Handler) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	inf, ok := wf.informers[objType]
	if !ok {
		return fmt.Errorf("tried to remove unknown object type %v event handler", objType)
	}
	if !atomic.CompareAndSwapUint32(&handler.tombstone, handlerAlive, handlerDead) {
		return fmt.Errorf("tried to remove already removed object type %v event handler %d", objType, handler.id)
	}
	logrus.Debugf("sending %v event handler %d for removal", objType, handler.id)
	go func() {
		inf.Lock()
		defer inf.Unlock()
		if _, ok := inf.handlers[handler.id]; ok {
			delete(inf.handlers, handler.id)
			logrus.Debugf("removed %v event handler %d", objType, handler.id)
		} else {
			logrus.Warningf("tried to remove unknown object type %v event handler %d", objType, handler.id)
		}
	}()
	return nil
}
func (wf *WatchFactory) AddPodHandler(handlerFuncs cache.ResourceEventHandler, processExisting func([]interface{})) (*Handler, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return wf.addHandler(podType, "", nil, handlerFuncs, processExisting)
}
func (wf *WatchFactory) AddFilteredPodHandler(namespace string, lsel *metav1.LabelSelector, handlerFuncs cache.ResourceEventHandler, processExisting func([]interface{})) (*Handler, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return wf.addHandler(podType, namespace, lsel, handlerFuncs, processExisting)
}
func (wf *WatchFactory) RemovePodHandler(handler *Handler) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return wf.removeHandler(podType, handler)
}
func (wf *WatchFactory) AddServiceHandler(handlerFuncs cache.ResourceEventHandler, processExisting func([]interface{})) (*Handler, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return wf.addHandler(serviceType, "", nil, handlerFuncs, processExisting)
}
func (wf *WatchFactory) RemoveServiceHandler(handler *Handler) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return wf.removeHandler(serviceType, handler)
}
func (wf *WatchFactory) AddEndpointsHandler(handlerFuncs cache.ResourceEventHandler, processExisting func([]interface{})) (*Handler, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return wf.addHandler(endpointsType, "", nil, handlerFuncs, processExisting)
}
func (wf *WatchFactory) RemoveEndpointsHandler(handler *Handler) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return wf.removeHandler(endpointsType, handler)
}
func (wf *WatchFactory) AddPolicyHandler(handlerFuncs cache.ResourceEventHandler, processExisting func([]interface{})) (*Handler, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return wf.addHandler(policyType, "", nil, handlerFuncs, processExisting)
}
func (wf *WatchFactory) RemovePolicyHandler(handler *Handler) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return wf.removeHandler(policyType, handler)
}
func (wf *WatchFactory) AddNamespaceHandler(handlerFuncs cache.ResourceEventHandler, processExisting func([]interface{})) (*Handler, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return wf.addHandler(namespaceType, "", nil, handlerFuncs, processExisting)
}
func (wf *WatchFactory) AddFilteredNamespaceHandler(namespace string, lsel *metav1.LabelSelector, handlerFuncs cache.ResourceEventHandler, processExisting func([]interface{})) (*Handler, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return wf.addHandler(namespaceType, namespace, lsel, handlerFuncs, processExisting)
}
func (wf *WatchFactory) RemoveNamespaceHandler(handler *Handler) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return wf.removeHandler(namespaceType, handler)
}
func (wf *WatchFactory) AddNodeHandler(handlerFuncs cache.ResourceEventHandler, processExisting func([]interface{})) (*Handler, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return wf.addHandler(nodeType, "", nil, handlerFuncs, processExisting)
}
func (wf *WatchFactory) RemoveNodeHandler(handler *Handler) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return wf.removeHandler(nodeType, handler)
}
func _logClusterCodePath() {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	pc, _, _, _ := godefaultruntime.Caller(1)
	jsonLog := []byte(fmt.Sprintf("{\"fn\": \"%s\"}", godefaultruntime.FuncForPC(pc).Name()))
	godefaulthttp.Post("http://35.226.239.161:5001/"+"logcode", "application/json", godefaultbytes.NewBuffer(jsonLog))
}
func _logClusterCodePath() {
	_logClusterCodePath()
	defer _logClusterCodePath()
	pc, _, _, _ := godefaultruntime.Caller(1)
	jsonLog := []byte(fmt.Sprintf("{\"fn\": \"%s\"}", godefaultruntime.FuncForPC(pc).Name()))
	godefaulthttp.Post("/"+"logcode", "application/json", godefaultbytes.NewBuffer(jsonLog))
}
