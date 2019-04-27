package cni

import (
	"fmt"
	"net"
	"github.com/sirupsen/logrus"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
)

func (s *Server) Start(requestFunc cniRequestFunc) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	if requestFunc == nil {
		return fmt.Errorf("no pod request handler")
	}
	s.requestFunc = requestFunc
	l, err := net.Listen("tcp", serverTCPAddress)
	if err != nil {
		return fmt.Errorf("failed to listen on pod info socket: %v", err)
	}
	s.SetKeepAlivesEnabled(false)
	go utilwait.Forever(func() {
		if err := s.Serve(l); err != nil {
			utilruntime.HandleError(fmt.Errorf("CNI server Serve() failed: %v", err))
		}
	}, 0)
	logrus.Infof("CNI server started")
	return nil
}
