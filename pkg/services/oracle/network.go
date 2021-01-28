package oracle

import (
	"errors"
	"net"
	"net/url"
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

func defaultURIValidator(u *url.URL) error {
	ip, err := net.ResolveIPAddr("ip", u.Hostname())
	if err != nil {
		return err
	}
	if isReserved(ip.IP) {
		return errors.New("IP is not global unicast")
	}
	return nil
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
