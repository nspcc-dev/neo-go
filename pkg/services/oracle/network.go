package oracle

import (
	"errors"
	"net"
)

// reservedCIDRs is a list of ip addresses for private networks.
// https://tools.ietf.org/html/rfc6890
var reservedCIDRs = []string{
	// IPv4
	"10.0.0.0/8",
	"100.64.0.0/10",
	"172.16.0.0/12",
	"192.0.0.0/24",
	"192.168.0.0/16",
	"198.18.0.0/15",
	// IPv6
	"fc00::/7",
}

var privateNets = make([]net.IPNet, 0, len(reservedCIDRs))

func init() {
	for i := range reservedCIDRs {
		_, ipNet, err := net.ParseCIDR(reservedCIDRs[i])
		if err != nil {
			panic(err)
		}
		privateNets = append(privateNets, *ipNet)
	}
}

func resolveAndCheck(network string, address string) (*net.IPAddr, error) {
	ip, err := net.ResolveIPAddr(network, address)
	if err != nil {
		return nil, err
	}
	if isReserved(ip.IP) {
		return nil, errors.New("IP is not global unicast")
	}
	return ip, nil
}

func isReserved(ip net.IP) bool {
	if !ip.IsGlobalUnicast() {
		return true
	}
	for i := range privateNets {
		if privateNets[i].Contains(ip) {
			return true
		}
	}
	return false
}
