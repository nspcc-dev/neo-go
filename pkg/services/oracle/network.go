package oracle

import (
	"fmt"
	"net"
	"net/http"
	"slices"
	"syscall"

	"github.com/nspcc-dev/neo-go/pkg/config"
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

func isReserved(ip net.IP) bool {
	if !ip.IsGlobalUnicast() {
		return true
	}
	return slices.ContainsFunc(privateNets, func(pn net.IPNet) bool {
		return pn.Contains(ip)
	})
}

func getDefaultClient(cfg config.OracleConfiguration) *http.Client {
	d := &net.Dialer{}
	if !cfg.AllowPrivateHost {
		// Control is used after request URI is resolved and network connection (network
		// file descriptor) is created, but right before listening/dialing
		// is started.
		// `address` represents a resolved IP address in the format of ip:port. `address`
		// is presented in its final (resolved) form that was used directly for network
		// connection establishing.
		// Control is called for each item in the set of IP addresses got from request
		// URI resolving. The first network connection with address that passes Control
		// function will be used for further request processing. Network connection
		// with address that failed Control will be ignored. If all the connections
		// fail Control, the most relevant error (the one from the first address)
		// will be returned after `Client.Do`.
		d.Control = func(network, address string, c syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("%w: failed to split address %s: %w", ErrRestrictedRedirect, address, err)
			}
			ip := net.ParseIP(host)
			if ip == nil {
				return fmt.Errorf("%w: failed to parse IP address %s", ErrRestrictedRedirect, address)
			}
			if isReserved(ip) {
				return fmt.Errorf("%w: IP is not global unicast", ErrRestrictedRedirect)
			}
			return nil
		}
	}
	var client http.Client
	client.Transport = &http.Transport{
		DisableKeepAlives: true,
		// Do not set DialTLSContext, so that DialContext will be used to establish the
		// connection. After that, TLS connection will be added to a persistent connection
		// by standard library code and handshaking will be performed.
		DialContext: d.DialContext,
	}
	client.Timeout = cfg.RequestTimeout
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > maxRedirections { // from https://github.com/neo-project/neo-modules/pull/698
			return fmt.Errorf("%w: %d redirections are reached", ErrRestrictedRedirect, maxRedirections)
		}
		if len(via) > 0 && via[0].URL.Scheme == "https" && req.URL.Scheme != "https" {
			lastHop := via[len(via)-1].URL
			return fmt.Errorf("%w: redirected from secure URL %s to insecure URL %s", ErrRestrictedRedirect, lastHop, req.URL)
		}
		return nil
	}
	return &client
}
