package cni

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

func ovsExec(args ...string) (string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	args = append([]string{"--timeout=30"}, args...)
	output, err := exec.Command("ovs-vsctl", args...).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("failed to run 'ovs-vsctl %s': %v\n  %q", strings.Join(args, " "), err, string(output))
	}
	outStr := string(output)
	trimmed := strings.TrimSpace(outStr)
	if strings.Count(trimmed, "\n") == 0 {
		outStr = trimmed
	}
	return outStr, nil
}
func ovsCreate(table string, values ...string) (string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	args := append([]string{"create", table}, values...)
	return ovsExec(args...)
}
func ovsDestroy(table, record string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_, err := ovsExec("--if-exists", "destroy", table, record)
	return err
}
func ovsSet(table, record string, values ...string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	args := append([]string{"set", table, record}, values...)
	_, err := ovsExec(args...)
	return err
}
func ovsFind(table, column, condition string) ([]string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	output, err := ovsExec("--no-heading", "--columns="+column, "find", table, condition)
	if err != nil {
		return nil, err
	}
	values := strings.Split(output, "\n\n")
	for i, val := range values {
		if unquoted, err := strconv.Unquote(val); err == nil {
			values[i] = unquoted
		}
	}
	return values, nil
}
func ovsClear(table, record string, columns ...string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	args := append([]string{"--if-exists", "clear", table, record}, columns...)
	_, err := ovsExec(args...)
	return err
}
