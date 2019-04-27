package util

import (
	"fmt"
	"math/big"
	"math/rand"
	"net"
	"time"
)

func GenerateMac() string {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	prefix := "00:00:00"
	newRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	mac := fmt.Sprintf("%s:%02X:%02X:%02X", prefix, newRand.Intn(255), newRand.Intn(255), newRand.Intn(255))
	return mac
}
func NextIP(ip net.IP) net.IP {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	i := ipToInt(ip)
	return intToIP(i.Add(i, big.NewInt(1)))
}
func ipToInt(ip net.IP) *big.Int {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	if v := ip.To4(); v != nil {
		return big.NewInt(0).SetBytes(v)
	}
	return big.NewInt(0).SetBytes(ip.To16())
}
func intToIP(i *big.Int) net.IP {
	_logClusterCodePath()
	defer _logClusterCodePath()
	_logClusterCodePath()
	defer _logClusterCodePath()
	return net.IP(i.Bytes())
}
