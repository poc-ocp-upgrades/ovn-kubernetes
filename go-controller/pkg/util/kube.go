package util

import (
	"fmt"
	"strings"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/cert"
	"github.com/openvswitch/ovn-kubernetes/go-controller/pkg/config"
)

func NewClientset(conf *config.KubernetesConfig) (*kubernetes.Clientset, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	var kconfig *rest.Config
	var err error
	if conf.Kubeconfig != "" {
		kconfig, err = clientcmd.BuildConfigFromFlags("", conf.Kubeconfig)
	} else if strings.HasPrefix(conf.APIServer, "https") {
		if conf.APIServer == "" || conf.Token == "" {
			return nil, fmt.Errorf("TLS-secured apiservers require token and CA certificate")
		}
		kconfig = &rest.Config{Host: conf.APIServer, BearerToken: conf.Token}
		if conf.CACert != "" {
			if _, err := cert.NewPool(conf.CACert); err != nil {
				return nil, err
			}
			kconfig.TLSClientConfig = rest.TLSClientConfig{CAFile: conf.CACert}
		}
	} else if strings.HasPrefix(conf.APIServer, "http") {
		kconfig, err = clientcmd.BuildConfigFromFlags(conf.APIServer, "")
	} else {
		kconfig, err = rest.InClusterConfig()
	}
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(kconfig)
}
