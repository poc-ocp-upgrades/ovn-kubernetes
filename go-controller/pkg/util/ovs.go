package util

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode"
	"github.com/sirupsen/logrus"
	kexec "k8s.io/utils/exec"
	"github.com/openvswitch/ovn-kubernetes/go-controller/pkg/config"
)

const (
	ovsCommandTimeout	= 15
	ovsVsctlCommand		= "ovs-vsctl"
	ovsOfctlCommand		= "ovs-ofctl"
	ovnNbctlCommand		= "ovn-nbctl"
	ipCommand		= "ip"
	powershellCommand	= "powershell"
	netshCommand		= "netsh"
	routeCommand		= "route"
	osRelease		= "/etc/os-release"
	rhel			= "RHEL"
	ubuntu			= "Ubuntu"
	windowsOS		= "windows"
)

func runningPlatform() (string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	if runtime.GOOS == windowsOS {
		return windowsOS, nil
	}
	fileContents, err := ioutil.ReadFile(osRelease)
	if err != nil {
		return "", fmt.Errorf("failed to parse file %s (%v)", osRelease, err)
	}
	var platform string
	ss := strings.Split(string(fileContents), "\n")
	for _, pair := range ss {
		keyValue := strings.Split(pair, "=")
		if len(keyValue) == 2 {
			if keyValue[0] == "Name" || keyValue[0] == "NAME" {
				platform = keyValue[1]
				break
			}
		}
	}
	if platform == "" {
		return "", fmt.Errorf("failed to find the platform name")
	}
	if strings.Contains(platform, "Fedora") || strings.Contains(platform, "Red Hat") || strings.Contains(platform, "CentOS") {
		return rhel, nil
	} else if strings.Contains(platform, "Debian") || strings.Contains(platform, ubuntu) {
		return ubuntu, nil
	} else if strings.Contains(platform, "VMware") {
		return "Photon", nil
	}
	return "", fmt.Errorf("Unknown platform")
}

type execHelper struct {
	exec		kexec.Interface
	ofctlPath	string
	vsctlPath	string
	nbctlPath	string
	ipPath		string
	powershellPath	string
	netshPath	string
	routePath	string
}

var runner *execHelper

func SetExec(exec kexec.Interface) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	var err error
	runner = &execHelper{exec: exec}
	runner.ofctlPath, err = exec.LookPath(ovsOfctlCommand)
	if err != nil {
		return err
	}
	runner.vsctlPath, err = exec.LookPath(ovsVsctlCommand)
	if err != nil {
		return err
	}
	runner.nbctlPath, err = exec.LookPath(ovnNbctlCommand)
	if err != nil {
		return err
	}
	if runtime.GOOS == windowsOS {
		runner.powershellPath, err = exec.LookPath(powershellCommand)
		if err != nil {
			return err
		}
		runner.netshPath, err = exec.LookPath(netshCommand)
		if err != nil {
			return err
		}
		runner.routePath, err = exec.LookPath(routeCommand)
		if err != nil {
			return err
		}
	} else {
		runner.ipPath, err = exec.LookPath(ipCommand)
		if err != nil {
			return err
		}
	}
	return nil
}
func GetExec() kexec.Interface {
	_logClusterCodePath()
	defer _logClusterCodePath()
	return runner.exec
}
func run(cmdPath string, args ...string) (*bytes.Buffer, *bytes.Buffer, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd := runner.exec.Command(cmdPath, args...)
	cmd.SetStdout(stdout)
	cmd.SetStderr(stderr)
	logrus.Debugf("exec: %s %s", cmdPath, strings.Join(args, " "))
	err := cmd.Run()
	if err != nil {
		logrus.Debugf("exec: %s %s => %v", cmdPath, strings.Join(args, " "), err)
	}
	return stdout, stderr, err
}
func RunOVSOfctl(args ...string) (string, string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	stdout, stderr, err := run(runner.ofctlPath, args...)
	return strings.Trim(stdout.String(), "\" \n"), stderr.String(), err
}
func RunOVSVsctl(args ...string) (string, string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	cmdArgs := []string{fmt.Sprintf("--timeout=%d", ovsCommandTimeout)}
	cmdArgs = append(cmdArgs, args...)
	stdout, stderr, err := run(runner.vsctlPath, cmdArgs...)
	return strings.Trim(strings.TrimSpace(stdout.String()), "\""), stderr.String(), err
}
func runOVNretry(cmdPath string, args ...string) (*bytes.Buffer, *bytes.Buffer, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	retriesLeft := 200
	for {
		stdout, stderr, err := run(cmdPath, args...)
		if err == nil {
			return stdout, stderr, err
		}
		if strings.Contains(stderr.String(), "Connection refused") {
			if retriesLeft == 0 {
				return stdout, stderr, err
			}
			retriesLeft--
			time.Sleep(2 * time.Second)
		} else {
			return stdout, stderr, fmt.Errorf("OVN command '%s %s' failed: %s", cmdPath, strings.Join(args, " "), err)
		}
	}
}
func RunOVNNbctlUnix(args ...string) (string, string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	cmdArgs := []string{fmt.Sprintf("--timeout=%d", ovsCommandTimeout)}
	cmdArgs = append(cmdArgs, args...)
	stdout, stderr, err := runOVNretry(runner.nbctlPath, cmdArgs...)
	return strings.Trim(strings.TrimFunc(stdout.String(), unicode.IsSpace), "\""), stderr.String(), err
}
func RunOVNNbctlWithTimeout(timeout int, args ...string) (string, string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	var cmdArgs []string
	if config.OvnNorth.ClientAuth.Scheme == config.OvnDBSchemeSSL {
		cmdArgs = []string{fmt.Sprintf("--private-key=%s", config.OvnNorth.ClientAuth.PrivKey), fmt.Sprintf("--certificate=%s", config.OvnNorth.ClientAuth.Cert), fmt.Sprintf("--bootstrap-ca-cert=%s", config.OvnNorth.ClientAuth.CACert), fmt.Sprintf("--db=%s", config.OvnNorth.ClientAuth.GetURL())}
	} else if config.OvnNorth.ClientAuth.Scheme == config.OvnDBSchemeTCP {
		cmdArgs = []string{fmt.Sprintf("--db=%s", config.OvnNorth.ClientAuth.GetURL())}
	}
	cmdArgs = append(cmdArgs, fmt.Sprintf("--timeout=%d", timeout))
	cmdArgs = append(cmdArgs, args...)
	stdout, stderr, err := runOVNretry(runner.nbctlPath, cmdArgs...)
	return strings.Trim(strings.TrimSpace(stdout.String()), "\""), stderr.String(), err
}
func RunOVNNbctl(args ...string) (string, string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	return RunOVNNbctlWithTimeout(ovsCommandTimeout, args...)
}
func RunIP(args ...string) (string, string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	stdout, stderr, err := run(runner.ipPath, args...)
	return strings.TrimSpace(stdout.String()), stderr.String(), err
}
func RunPowershell(args ...string) (string, string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	stdout, stderr, err := run(runner.powershellPath, args...)
	return strings.TrimSpace(stdout.String()), stderr.String(), err
}
func RunNetsh(args ...string) (string, string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	stdout, stderr, err := run(runner.netshPath, args...)
	return strings.TrimSpace(stdout.String()), stderr.String(), err
}
func RunRoute(args ...string) (string, string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	stdout, stderr, err := run(runner.routePath, args...)
	return strings.TrimSpace(stdout.String()), stderr.String(), err
}
func RawExec(cmdPath string, args ...string) (string, string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	if filepath.Base(cmdPath) == cmdPath {
		var err error
		cmdPath, err = runner.exec.LookPath(cmdPath)
		if err != nil {
			return "", "", err
		}
	}
	stdout, stderr, err := run(cmdPath, args...)
	return strings.TrimSpace(stdout.String()), stderr.String(), err
}
