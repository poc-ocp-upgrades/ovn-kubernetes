package config

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	gcfg "gopkg.in/gcfg.v1"
	kexec "k8s.io/utils/exec"
)

var (
	Version		= "0.3.0"
	Default		= DefaultConfig{MTU: 1400, ConntrackZone: 64000, EncapType: "geneve", EncapIP: "", InactivityProbe: 100000}
	Logging		= LoggingConfig{File: "", Level: 4}
	CNI		= CNIConfig{ConfDir: "/etc/cni/net.d", Plugin: "ovn-k8s-cni-overlay", WinHNSNetworkID: ""}
	Kubernetes	= KubernetesConfig{APIServer: "http://localhost:8080"}
	OvnNorth	OvnAuthConfig
	OvnSouth	OvnAuthConfig
)

type DefaultConfig struct {
	MTU		int	`gcfg:"mtu"`
	ConntrackZone	int	`gcfg:"conntrack-zone"`
	EncapType	string	`gcfg:"encap-type"`
	EncapIP		string	`gcfg:"encap-ip"`
	InactivityProbe	int	`gcfg:"inactivity-probe"`
}
type LoggingConfig struct {
	File	string	`gcfg:"logfile"`
	Level	int	`gcfg:"loglevel"`
}
type CNIConfig struct {
	ConfDir		string	`gcfg:"conf-dir"`
	Plugin		string	`gcfg:"plugin"`
	WinHNSNetworkID	string	`gcfg:"win-hnsnetwork-id"`
}
type KubernetesConfig struct {
	Kubeconfig	string	`gcfg:"kubeconfig"`
	CACert		string	`gcfg:"cacert"`
	APIServer	string	`gcfg:"apiserver"`
	Token		string	`gcfg:"token"`
}
type OvnAuthConfig struct {
	ClientAuth	*OvnDBAuth
	ServerAuth	*OvnDBAuth
}
type rawOvnAuthConfig struct {
	Address		string	`gcfg:"address"`
	ClientPrivKey	string	`gcfg:"client-privkey"`
	ClientCert	string	`gcfg:"client-cert"`
	ClientCACert	string	`gcfg:"client-cacert"`
	ServerPrivKey	string	`gcfg:"server-privkey"`
	ServerCert	string	`gcfg:"server-cert"`
	ServerCACert	string	`gcfg:"server-cacert"`
}
type OvnDBScheme string

const (
	OvnDBSchemeSSL	OvnDBScheme	= "ssl"
	OvnDBSchemeTCP	OvnDBScheme	= "tcp"
	OvnDBSchemeUnix	OvnDBScheme	= "unix"
)

type config struct {
	Default		DefaultConfig
	Logging		LoggingConfig
	CNI		CNIConfig
	Kubernetes	KubernetesConfig
	OvnNorth	rawOvnAuthConfig
	OvnSouth	rawOvnAuthConfig
}

var (
	savedDefault	DefaultConfig
	savedLogging	LoggingConfig
	savedCNI	CNIConfig
	savedKubernetes	KubernetesConfig
	savedOvnNorth	OvnAuthConfig
	savedOvnSouth	OvnAuthConfig
)

func init() {
	_logClusterCodePath()
	defer _logClusterCodePath()
	savedDefault = Default
	savedLogging = Logging
	savedCNI = CNI
	savedKubernetes = Kubernetes
	savedOvnNorth = OvnNorth
	savedOvnSouth = OvnSouth
}
func RestoreDefaultConfig() {
	_logClusterCodePath()
	defer _logClusterCodePath()
	Default = savedDefault
	Logging = savedLogging
	CNI = savedCNI
	Kubernetes = savedKubernetes
	OvnNorth = savedOvnNorth
	OvnSouth = savedOvnSouth
}
func overrideFields(dst, src interface{}) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	dstStruct := reflect.ValueOf(dst).Elem()
	srcStruct := reflect.ValueOf(src).Elem()
	if dstStruct.Kind() != srcStruct.Kind() || dstStruct.Kind() != reflect.Struct {
		panic("mismatched value types")
	}
	if dstStruct.NumField() != srcStruct.NumField() {
		panic("mismatched struct types")
	}
	for i := 0; i < dstStruct.NumField(); i++ {
		dstField := dstStruct.Field(i)
		srcField := srcStruct.Field(i)
		if dstField.Kind() != srcField.Kind() {
			panic("mismatched struct fields")
		}
		switch srcField.Kind() {
		case reflect.String:
			if srcField.String() != "" {
				dstField.Set(srcField)
			}
		case reflect.Int:
			if srcField.Int() != 0 {
				dstField.Set(srcField)
			}
		default:
			panic(fmt.Sprintf("unhandled struct field type: %v", srcField.Kind()))
		}
	}
}

var cliConfig config
var Flags = []cli.Flag{cli.StringFlag{Name: "config-file", Usage: "configuration file path (default: /etc/openvswitch/ovn_k8s.conf)"}, cli.IntFlag{Name: "mtu", Usage: "MTU value used for the overlay networks (default: 1400)", Destination: &cliConfig.Default.MTU}, cli.IntFlag{Name: "conntrack-zone", Usage: "For gateway nodes, the conntrack zone used for conntrack flow rules (default: 64000)", Destination: &cliConfig.Default.ConntrackZone}, cli.StringFlag{Name: "encap-type", Usage: "The encapsulation protocol to use to transmit packets between hypervisors (default: geneve)", Destination: &cliConfig.Default.EncapType}, cli.StringFlag{Name: "encap-ip", Usage: "The IP address of the encapsulation endpoint (default: Node IP address resolved from Node hostname)", Destination: &cliConfig.Default.EncapIP}, cli.IntFlag{Name: "inactivity-probe", Usage: "Maximum number of milliseconds of idle time on " + "connection for ovn-controller before it sends a inactivity probe", Destination: &cliConfig.Default.InactivityProbe}, cli.IntFlag{Name: "loglevel", Usage: "log verbosity and level: 5=debug, 4=info, 3=warn, 2=error, 1=fatal (default: 4)", Destination: &cliConfig.Logging.Level}, cli.StringFlag{Name: "logfile", Usage: "path of a file to direct log output to", Destination: &cliConfig.Logging.File}, cli.StringFlag{Name: "cni-conf-dir", Usage: "the CNI config directory in which to write the overlay CNI config file (default: /etc/cni/net.d)", Destination: &cliConfig.CNI.ConfDir}, cli.StringFlag{Name: "cni-plugin", Usage: "the name of the CNI plugin (default: ovn-k8s-cni-overlay)", Destination: &cliConfig.CNI.Plugin}, cli.StringFlag{Name: "win-hnsnetwork-id", Usage: "the ID of the HNS network to which containers will be attached (default: not set)", Destination: &cliConfig.CNI.WinHNSNetworkID}, cli.StringFlag{Name: "k8s-kubeconfig", Usage: "absolute path to the Kubernetes kubeconfig file (not required if the --k8s-apiserver, --k8s-ca-cert, and --k8s-token are given)", Destination: &cliConfig.Kubernetes.Kubeconfig}, cli.StringFlag{Name: "k8s-apiserver", Usage: "URL of the Kubernetes API server (not required if --k8s-kubeconfig is given) (default: http://localhost:8443)", Destination: &cliConfig.Kubernetes.APIServer}, cli.StringFlag{Name: "k8s-cacert", Usage: "the absolute path to the Kubernetes API CA certificate (not required if --k8s-kubeconfig is given)", Destination: &cliConfig.Kubernetes.CACert}, cli.StringFlag{Name: "k8s-token", Usage: "the Kubernetes API authentication token (not required if --k8s-kubeconfig is given)", Destination: &cliConfig.Kubernetes.Token}, cli.StringFlag{Name: "nb-address", Usage: "IP address and port of the OVN northbound API " + "(eg, ssl://1.2.3.4:6641,ssl://1.2.3.5:6642).  Leave empty to " + "use a local unix socket.", Destination: &cliConfig.OvnNorth.Address}, cli.StringFlag{Name: "nb-server-privkey", Usage: "Private key that the OVN northbound API should use for securing the API.  Leave empty to use local unix socket. (default: /etc/openvswitch/ovnnb-privkey.pem)", Destination: &cliConfig.OvnNorth.ServerPrivKey}, cli.StringFlag{Name: "nb-server-cert", Usage: "Server certificate that the OVN northbound API should use for securing the API.  Leave empty to use local unix socket. (default: /etc/openvswitch/ovnnb-cert.pem)", Destination: &cliConfig.OvnNorth.ServerCert}, cli.StringFlag{Name: "nb-server-cacert", Usage: "CA certificate that the OVN northbound API should use for securing the API.  Leave empty to use local unix socket. (default: /etc/openvswitch/ovnnb-ca.cert)", Destination: &cliConfig.OvnNorth.ServerCACert}, cli.StringFlag{Name: "nb-client-privkey", Usage: "Private key that the client should use for talking to the OVN database.  Leave empty to use local unix socket. (default: /etc/openvswitch/ovnnb-privkey.pem)", Destination: &cliConfig.OvnNorth.ClientPrivKey}, cli.StringFlag{Name: "nb-client-cert", Usage: "Client certificate that the client should use for talking to the OVN database.  Leave empty to use local unix socket. (default: /etc/openvswitch/ovnnb-cert.pem)", Destination: &cliConfig.OvnNorth.ClientCert}, cli.StringFlag{Name: "nb-client-cacert", Usage: "CA certificate that the client should use for talking to the OVN database.  Leave empty to use local unix socket. (default: /etc/openvswitch/ovnnb-ca.cert)", Destination: &cliConfig.OvnNorth.ClientCACert}, cli.StringFlag{Name: "sb-address", Usage: "IP address and port of the OVN southbound API " + "(eg, ssl://1.2.3.4:6642,ssl://1.2.3.5:6642).  " + "Leave empty to use a local unix socket.", Destination: &cliConfig.OvnSouth.Address}, cli.StringFlag{Name: "sb-server-privkey", Usage: "Private key that the OVN southbound API should use for securing the API.  Leave empty to use local unix socket. (default: /etc/openvswitch/ovnsb-privkey.pem)", Destination: &cliConfig.OvnSouth.ServerPrivKey}, cli.StringFlag{Name: "sb-server-cert", Usage: "Server certificate that the OVN southbound API should use for securing the API.  Leave empty to use local unix socket. (default: /etc/openvswitch/ovnsb-cert.pem)", Destination: &cliConfig.OvnSouth.ServerCert}, cli.StringFlag{Name: "sb-server-cacert", Usage: "CA certificate that the OVN southbound API should use for securing the API.  Leave empty to use local unix socket. (default: /etc/openvswitch/ovnsb-ca.cert)", Destination: &cliConfig.OvnSouth.ServerCACert}, cli.StringFlag{Name: "sb-client-privkey", Usage: "Private key that the client should use for talking to the OVN database.  Leave empty to use local unix socket. (default: /etc/openvswitch/ovnsb-privkey.pem)", Destination: &cliConfig.OvnSouth.ClientPrivKey}, cli.StringFlag{Name: "sb-client-cert", Usage: "Client certificate that the client should use for talking to the OVN database.  Leave empty to use local unix socket. (default: /etc/openvswitch/ovnsb-cert.pem)", Destination: &cliConfig.OvnSouth.ClientCert}, cli.StringFlag{Name: "sb-client-cacert", Usage: "CA certificate that the client should use for talking to the OVN database.  Leave empty to use local unix socket. (default: /etc/openvswitch/ovnsb-ca.cert)", Destination: &cliConfig.OvnSouth.ClientCACert}}

type Defaults struct {
	OvnNorthAddress	bool
	K8sAPIServer	bool
	K8sToken	bool
	K8sCert		bool
}

const (
	ovsVsctlCommand = "ovs-vsctl"
)

func rawExec(exec kexec.Interface, cmd string, args ...string) (string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	cmdPath, err := exec.LookPath(cmd)
	if err != nil {
		return "", err
	}
	logrus.Debugf("exec: %s %s", cmdPath, strings.Join(args, " "))
	out, err := exec.Command(cmdPath, args...).CombinedOutput()
	if err != nil {
		logrus.Debugf("exec: %s %s => %v", cmdPath, strings.Join(args, " "), err)
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
func runOVSVsctl(exec kexec.Interface, args ...string) (string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	newArgs := append([]string{"--timeout=15"}, args...)
	out, err := rawExec(exec, ovsVsctlCommand, newArgs...)
	if err != nil {
		return "", err
	}
	return strings.Trim(strings.TrimSpace(out), "\""), nil
}
func getOVSExternalID(exec kexec.Interface, name string) string {
	_logClusterCodePath()
	defer _logClusterCodePath()
	out, err := runOVSVsctl(exec, "--if-exists", "get", "Open_vSwitch", ".", "external_ids:"+name)
	if err != nil {
		logrus.Debugf("failed to get OVS external_id %s: %v\n\t%s", name, err, out)
		return ""
	}
	return out
}
func setOVSExternalID(exec kexec.Interface, key, value string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	out, err := runOVSVsctl(exec, "set", "Open_vSwitch", ".", fmt.Sprintf("external_ids:%s=%s", key, value))
	if err != nil {
		return fmt.Errorf("Error setting OVS external ID '%s=%s': %v\n  %q", key, value, err, out)
	}
	return nil
}
func buildKubernetesConfig(exec kexec.Interface, cli, file *config, defaults *Defaults) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	if defaults.K8sAPIServer {
		Kubernetes.APIServer = getOVSExternalID(exec, "k8s-api-server")
	}
	if defaults.K8sToken {
		Kubernetes.Token = getOVSExternalID(exec, "k8s-api-token")
	}
	if defaults.K8sCert {
		Kubernetes.CACert = getOVSExternalID(exec, "k8s-ca-certificate")
	}
	overrideFields(&Kubernetes, &file.Kubernetes)
	overrideFields(&Kubernetes, &cli.Kubernetes)
	if Kubernetes.Kubeconfig != "" && !pathExists(Kubernetes.Kubeconfig) {
		return fmt.Errorf("kubernetes kubeconfig file %q not found", Kubernetes.Kubeconfig)
	}
	if Kubernetes.CACert != "" && !pathExists(Kubernetes.CACert) {
		return fmt.Errorf("kubernetes CA certificate file %q not found", Kubernetes.CACert)
	}
	url, err := url.Parse(Kubernetes.APIServer)
	if err != nil {
		return fmt.Errorf("kubernetes API server address %q invalid: %v", Kubernetes.APIServer, err)
	} else if url.Scheme != "https" && url.Scheme != "http" {
		return fmt.Errorf("kubernetes API server URL scheme %q invalid", url.Scheme)
	}
	return nil
}
func buildOvnAuth(exec kexec.Interface, direction, externalID string, cliAuth, confAuth *rawOvnAuthConfig, readAddress bool) (OvnAuthConfig, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	ctlCmd := "ovn-" + direction + "ctl"
	address := cliAuth.Address
	if address == "" {
		address = confAuth.Address
	}
	if address == "" && readAddress {
		address = getOVSExternalID(exec, "ovn-"+direction)
	}
	auth := &rawOvnAuthConfig{Address: address}
	if strings.HasPrefix(address, "ssl") {
		auth.ClientCACert = "/etc/openvswitch/ovn" + direction + "-ca.cert"
		auth.ServerCACert = auth.ClientCACert
		auth.ClientPrivKey = "/etc/openvswitch/ovn" + direction + "-privkey.pem"
		auth.ServerPrivKey = auth.ClientPrivKey
		auth.ClientCert = "/etc/openvswitch/ovn" + direction + "-cert.pem"
		auth.ServerCert = auth.ClientCert
	}
	overrideFields(auth, confAuth)
	overrideFields(auth, cliAuth)
	clientAuth, err := newOvnDBAuth(exec, ctlCmd, externalID, auth.Address, auth.ClientPrivKey, auth.ClientCert, auth.ClientCACert, false)
	if err != nil {
		return OvnAuthConfig{}, err
	}
	serverAuth, err := newOvnDBAuth(exec, ctlCmd, externalID, auth.Address, auth.ServerPrivKey, auth.ServerCert, auth.ServerCACert, true)
	if err != nil {
		return OvnAuthConfig{}, err
	}
	return OvnAuthConfig{ClientAuth: clientAuth, ServerAuth: serverAuth}, nil
}
func getConfigFilePath(ctx *cli.Context) (string, bool) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	configFile := ctx.String("config-file")
	if configFile != "" {
		return configFile, false
	}
	if runtime.GOOS != "windows" {
		return filepath.Join("/etc", "openvswitch", "ovn_k8s.conf"), true
	}
	return filepath.Join(os.Getenv("OVS_SYSCONFDIR"), "ovn_k8s.conf"), true
}
func InitConfig(ctx *cli.Context, exec kexec.Interface, defaults *Defaults) (string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	return InitConfigWithPath(ctx, exec, "", defaults)
}
func InitConfigWithPath(ctx *cli.Context, exec kexec.Interface, configFile string, defaults *Defaults) (string, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	var cfg config
	var retConfigFile string
	var configFileIsDefault bool
	if configFile == "" {
		configFile, configFileIsDefault = getConfigFilePath(ctx)
	}
	logrus.SetOutput(os.Stderr)
	if !configFileIsDefault {
		retConfigFile = configFile
	}
	f, err := os.Open(configFile)
	if err != nil && !configFileIsDefault {
		return "", fmt.Errorf("failed to open config file %s: %v", configFile, err)
	}
	if f != nil {
		defer f.Close()
		if err = gcfg.ReadInto(&cfg, f); err != nil {
			return "", fmt.Errorf("failed to parse config file %s: %v", f.Name(), err)
		}
		logrus.Infof("Parsed config file %s", f.Name())
		logrus.Infof("Parsed config: %+v", cfg)
	}
	if defaults == nil {
		defaults = &Defaults{}
	}
	overrideFields(&Default, &cfg.Default)
	overrideFields(&Default, &cliConfig.Default)
	overrideFields(&CNI, &cfg.CNI)
	overrideFields(&CNI, &cliConfig.CNI)
	overrideFields(&Logging, &cfg.Logging)
	overrideFields(&Logging, &cliConfig.Logging)
	logrus.SetLevel(logrus.Level(Logging.Level))
	if Logging.File != "" {
		var file *os.File
		file, err = os.OpenFile(Logging.File, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0660)
		if err != nil {
			logrus.Errorf("failed to open logfile %s (%v). Ignoring..", Logging.File, err)
		} else {
			logrus.SetOutput(file)
		}
	}
	if err = buildKubernetesConfig(exec, &cliConfig, &cfg, defaults); err != nil {
		return "", err
	}
	OvnNorth, err = buildOvnAuth(exec, "nb", "ovn-nb", &cliConfig.OvnNorth, &cfg.OvnNorth, defaults.OvnNorthAddress)
	if err != nil {
		return "", err
	}
	OvnSouth, err = buildOvnAuth(exec, "sb", "ovn-remote", &cliConfig.OvnSouth, &cfg.OvnSouth, false)
	if err != nil {
		return "", err
	}
	logrus.Debugf("Default config: %+v", Default)
	logrus.Debugf("Logging config: %+v", Logging)
	logrus.Debugf("CNI config: %+v", CNI)
	logrus.Debugf("Kubernetes config: %+v", Kubernetes)
	logrus.Debugf("OVN North config: %+v", OvnNorth)
	logrus.Debugf("OVN South config: %+v", OvnSouth)
	return retConfigFile, nil
}

type OvnDBAuth struct {
	OvnAddressForClient	string
	OvnAddressForServer	string
	PrivKey			string
	Cert			string
	CACert			string
	Scheme			OvnDBScheme
	server			bool
	ctlCmd			string
	externalID		string
	exec			kexec.Interface
}

func pathExists(path string) bool {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_, err := os.Stat(path)
	if err != nil && os.IsNotExist(err) {
		return false
	}
	return true
}
func newOvnDBAuth(exec kexec.Interface, ctlCmd, externalID, urlString, privkey, cert, cacert string, server bool) (*OvnDBAuth, error) {
	_logClusterCodePath()
	defer _logClusterCodePath()
	if urlString == "" {
		if privkey != "" || cert != "" || cacert != "" {
			return nil, fmt.Errorf("certificate or key given; perhaps you mean to use the 'ssl' scheme?")
		}
		return &OvnDBAuth{server: server, Scheme: OvnDBSchemeUnix, ctlCmd: ctlCmd, externalID: externalID, exec: exec}, nil
	}
	auth := &OvnDBAuth{server: server, ctlCmd: ctlCmd, externalID: externalID, exec: exec}
	scheme := ""
	urlString = strings.Replace(urlString, "//", "", -1)
	ovnAddresses := strings.Split(urlString, ",")
	for _, ovnAddress := range ovnAddresses {
		splits := strings.Split(ovnAddress, ":")
		if len(splits) != 3 {
			return nil, fmt.Errorf("Failed to parse OVN address %s", urlString)
		}
		hostPort := splits[1] + ":" + splits[2]
		if scheme == "" {
			scheme = splits[0]
		} else if scheme != splits[0] {
			return nil, fmt.Errorf("Invalid protocols in OVN address %s", urlString)
		}
		host, port, err := net.SplitHostPort(hostPort)
		if err != nil {
			return nil, fmt.Errorf("failed to parse OVN DB host/port %q: %v", hostPort, err)
		}
		ip := net.ParseIP(host)
		if ip == nil {
			return nil, fmt.Errorf("OVN DB host %q must be an IP address, "+"not a DNS name", hostPort)
		}
		if server && auth.OvnAddressForServer == "" {
			auth.OvnAddressForServer = fmt.Sprintf("p%s:%s", scheme, port)
		}
		if !server {
			if auth.OvnAddressForClient == "" {
				auth.OvnAddressForClient = fmt.Sprintf("%s:%s:%s", scheme, host, port)
			} else {
				auth.OvnAddressForClient = fmt.Sprintf("%s,%s:%s:%s", auth.OvnAddressForClient, scheme, host, port)
			}
		}
	}
	switch {
	case scheme == "ssl":
		if privkey == "" || cert == "" || cacert == "" {
			return nil, fmt.Errorf("must specify private key, certificate, and CA certificate for 'ssl' scheme")
		}
		auth.Scheme = OvnDBSchemeSSL
		auth.PrivKey = privkey
		auth.Cert = cert
		auth.CACert = cacert
	case scheme == "tcp":
		if privkey != "" || cert != "" || cacert != "" {
			return nil, fmt.Errorf("certificate or key given; perhaps you mean to use the 'ssl' scheme?")
		}
		auth.Scheme = OvnDBSchemeTCP
	default:
		return nil, fmt.Errorf("unknown OVN DB scheme %q", scheme)
	}
	return auth, nil
}
func (a *OvnDBAuth) ensureCACert() error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	if pathExists(a.CACert) {
		return nil
	}
	if a.server {
		return fmt.Errorf("CA certificate file %s not found", a.CACert)
	}
	args := []string{"--db=" + a.GetURL(), "--timeout=5"}
	if a.Scheme == OvnDBSchemeSSL {
		args = append(args, "--private-key="+a.PrivKey)
		args = append(args, "--certificate="+a.Cert)
		args = append(args, "--bootstrap-ca-cert="+a.CACert)
	}
	args = append(args, "list", "nb_global")
	_, _ = rawExec(a.exec, "ovn-nbctl", args...)
	if _, err := os.Stat(a.CACert); os.IsNotExist(err) {
		logrus.Warnf("bootstrapping %s CA certificate failed", a.CACert)
	}
	return nil
}
func (a *OvnDBAuth) GetURL() string {
	_logClusterCodePath()
	defer _logClusterCodePath()
	if a.server {
		return a.OvnAddressForServer
	}
	return a.OvnAddressForClient
}
func (a *OvnDBAuth) SetDBAuth() error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	if a.Scheme == OvnDBSchemeUnix {
		return nil
	} else if a.Scheme == OvnDBSchemeSSL {
		if !pathExists(a.PrivKey) {
			return fmt.Errorf("private key file %s not found", a.PrivKey)
		}
		if !pathExists(a.Cert) {
			return fmt.Errorf("certificate file %s not found", a.Cert)
		}
	}
	if a.server {
		out, err := rawExec(a.exec, a.ctlCmd, "set-connection", a.GetURL(), "--", "set", "connection", ".", "inactivity_probe=0")
		if err != nil {
			return fmt.Errorf("error setting %s API connection: %v\n  %q", a.ctlCmd, err, out)
		}
		if a.Scheme == OvnDBSchemeSSL {
			if !pathExists(a.CACert) {
				return fmt.Errorf("server CA certificate file %s not found", a.CACert)
			}
			out, err = rawExec(a.exec, a.ctlCmd, "del-ssl")
			if err != nil {
				return fmt.Errorf("error deleting %s SSL configuration: %v\n %q", a.ctlCmd, err, out)
			}
			out, err = rawExec(a.exec, a.ctlCmd, "set-ssl", a.PrivKey, a.Cert, a.CACert)
			if err != nil {
				return fmt.Errorf("error setting %s SSL API certificates: %v\n  %q", a.ctlCmd, err, out)
			}
		}
	} else {
		if a.Scheme == OvnDBSchemeSSL {
			if err := a.ensureCACert(); err != nil {
				return err
			}
			if a.ctlCmd == "ovn-sbctl" {
				out, err := runOVSVsctl(a.exec, "del-ssl")
				if err != nil {
					return fmt.Errorf("error deleting ovs-vsctl SSL "+"configuration: %q (%v)", out, err)
				}
				out, err = runOVSVsctl(a.exec, "set-ssl", a.PrivKey, a.Cert, a.CACert)
				if err != nil {
					return fmt.Errorf("error setting client southbound DB SSL options: %v\n  %q", err, out)
				}
			}
		}
		if err := setOVSExternalID(a.exec, a.externalID, "\""+a.GetURL()+"\""); err != nil {
			return err
		}
	}
	return nil
}
func (a *OvnDBAuth) updateIP(newIP string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	if a.OvnAddressForClient != "" {
		s := strings.Split(a.OvnAddressForClient, ":")
		if len(s) != 3 {
			return fmt.Errorf("failed to parse OvnDBAuth "+"a.OvnAddressForClient: %q", a.OvnAddressForClient)
		}
		a.OvnAddressForClient = s[0] + ":" + newIP + s[2]
	}
	return nil
}
func UpdateOvnNodeAuth(masterIP string) error {
	_logClusterCodePath()
	defer _logClusterCodePath()
	logrus.Debugf("Update OVN node auth with new master ip: %s", masterIP)
	if err := OvnNorth.ClientAuth.updateIP(masterIP); err != nil {
		return fmt.Errorf("failed to update OvnNorth ClientAuth URL: %v", err)
	}
	if err := OvnNorth.ServerAuth.updateIP(masterIP); err != nil {
		return fmt.Errorf("failed to update OvnNorth ServerAuth URL: %v", err)
	}
	if err := OvnSouth.ClientAuth.updateIP(masterIP); err != nil {
		return fmt.Errorf("failed to update OvnSouth ClientAuth URL: %v", err)
	}
	if err := OvnSouth.ServerAuth.updateIP(masterIP); err != nil {
		return fmt.Errorf("failed to update OvnSouth ServerAuth URL: %v", err)
	}
	return nil
}
