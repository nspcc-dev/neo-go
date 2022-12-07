package config

import (
	"net"
)

// BasicService is used as a simple base for node services like Pprof, RPC or
// Prometheus monitoring.
type BasicService struct {
	Enabled bool `yaml:"Enabled"`
	// Deprecated: please, use Addresses section instead. This field will be removed later.
	Address *string `yaml:"Address,omitempty"`
	// Deprecated: please, use Addresses section instead. This field will be removed later.
	Port *string `yaml:"Port,omitempty"`
	// Addresses holds the list of bind addresses in the form of "address:port".
	Addresses []string `yaml:"Addresses"`
}

// GetAddresses returns the set of unique (in terms of raw strings) pairs host:port
// for the given basic service.
func (s BasicService) GetAddresses() []string {
	addrs := make([]string, len(s.Addresses), len(s.Addresses)+1)
	copy(addrs, s.Addresses)
	if s.Address != nil || s.Port != nil { //nolint:staticcheck // SA1019: s.Address is deprecated
		var (
			addr string
			port string
		)
		if s.Address != nil { //nolint:staticcheck // SA1019: s.Address is deprecated
			addr = *s.Address //nolint:staticcheck // SA1019: s.Address is deprecated
		}
		if s.Port != nil { //nolint:staticcheck // SA1019: s.Port is deprecated
			port = *s.Port //nolint:staticcheck // SA1019: s.Port is deprecated
		}
		addrs = append(addrs, net.JoinHostPort(addr, port))
	}
	return addrs
}
