package cni

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"syscall"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	utilwait "k8s.io/apimachinery/pkg/util/wait"
)

func (s *Server) Start(requestFunc cniRequestFunc) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	if requestFunc == nil {
		return fmt.Errorf("no pod request handler")
	}
	s.requestFunc = requestFunc
	socketPath := filepath.Join(s.rundir, serverSocketName)
	if err := os.RemoveAll(s.rundir); err != nil && !os.IsNotExist(err) {
		info, err := os.Stat(s.rundir)
		if err != nil {
			return fmt.Errorf("failed to stat old pod info socket directory %s: %v", s.rundir, err)
		}
		tmp := info.Sys()
		statt, ok := tmp.(*syscall.Stat_t)
		if !ok {
			return fmt.Errorf("failed to read pod info socket directory stat info: %T", tmp)
		}
		if statt.Uid != 0 {
			return fmt.Errorf("insecure owner of pod info socket directory %s: %v", s.rundir, statt.Uid)
		}
		if info.Mode()&0777 != 0700 {
			return fmt.Errorf("insecure permissions on pod info socket directory %s: %v", s.rundir, info.Mode())
		}
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove old pod info socket %s: %v", socketPath, err)
		}
	}
	if err := os.MkdirAll(s.rundir, 0700); err != nil {
		return fmt.Errorf("failed to create pod info socket directory %s: %v", s.rundir, err)
	}
	l, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("failed to listen on pod info socket: %v", err)
	}
	if err := os.Chmod(socketPath, 0600); err != nil {
		l.Close()
		return fmt.Errorf("failed to set pod info socket mode: %v", err)
	}
	s.SetKeepAlivesEnabled(false)
	go utilwait.Forever(func() {
		if err := s.Serve(l); err != nil {
			utilruntime.HandleError(fmt.Errorf("CNI server Serve() failed: %v", err))
		}
	}, 0)
	return nil
}
