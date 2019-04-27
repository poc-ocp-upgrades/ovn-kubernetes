package util

import (
	"fmt"
	"os/exec"
	"strings"
	"github.com/urfave/cli"
)

func StringArg(context *cli.Context, name string) (string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	val := context.String(name)
	if val == "" {
		return "", fmt.Errorf("argument --%s should be non-null", name)
	}
	return val, nil
}
func FetchIfMacWindows(interfaceName string) (string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	stdoutStderr, err := exec.Command("powershell", "$(Get-NetAdapter", "-IncludeHidden", "-InterfaceAlias", fmt.Sprintf("\"%s\"", interfaceName), ").MacAddress").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("Failed to get mac address of ovn-k8s-master, stderr: %q, error: %v", fmt.Sprintf("%s", stdoutStderr), err)
	}
	macAddress := strings.Replace(strings.TrimSpace(fmt.Sprintf("%s", stdoutStderr)), "-", ":", -1)
	return strings.ToLower(macAddress), nil
}
