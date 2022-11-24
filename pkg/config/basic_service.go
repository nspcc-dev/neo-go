package config

import (
	"net"
	"strconv"
)

// BasicService is used as a simple base for node services like Pprof, RPC or
// Prometheus monitoring.
type BasicService struct {
	Enabled bool   `yaml:"Enabled"`
	Address string `yaml:"Address"`
	Port    uint16 `yaml:"Port"`
}

// FormatAddress returns the full service's address in the form of "address:port".
func (s BasicService) FormatAddress() string {
	return net.JoinHostPort(s.Address, strconv.FormatUint(uint64(s.Port), 10))
}
