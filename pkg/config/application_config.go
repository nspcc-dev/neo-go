package config

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
)

// ApplicationConfiguration config specific to the node.
type ApplicationConfiguration struct {
	Ledger `yaml:",inline"`

	DBConfiguration dbconfig.DBConfiguration `yaml:"DBConfiguration"`

	Logger `yaml:",inline"`

	P2P P2P `yaml:"P2P"`

	Pprof      BasicService `yaml:"Pprof"`
	Prometheus BasicService `yaml:"Prometheus"`

	Relay             bool                `yaml:"Relay"`
	Consensus         Consensus           `yaml:"Consensus"`
	RPC               RPC                 `yaml:"RPC"`
	Oracle            OracleConfiguration `yaml:"Oracle"`
	P2PNotary         P2PNotary           `yaml:"P2PNotary"`
	StateRoot         StateRoot           `yaml:"StateRoot"`
	NeoFSBlockFetcher NeoFSBlockFetcher   `yaml:"NeoFSBlockFetcher"`
}

// EqualsButServices returns true when the o is the same as a except for services
// (Oracle, P2PNotary, Pprof, Prometheus, RPC and StateRoot sections)
// and LogLevel field.
func (a *ApplicationConfiguration) EqualsButServices(o *ApplicationConfiguration) bool {
	if len(a.P2P.Addresses) != len(o.P2P.Addresses) {
		return false
	}
	aCp := slices.Clone(a.P2P.Addresses)
	oCp := slices.Clone(o.P2P.Addresses)
	slices.Sort(aCp)
	slices.Sort(oCp)
	if !slices.Equal(aCp, oCp) {
		return false
	}
	if a.P2P.AttemptConnPeers != o.P2P.AttemptConnPeers ||
		a.P2P.BroadcastFactor != o.P2P.BroadcastFactor ||
		a.DBConfiguration != o.DBConfiguration ||
		a.P2P.DialTimeout != o.P2P.DialTimeout ||
		a.P2P.ExtensiblePoolSize != o.P2P.ExtensiblePoolSize ||
		a.LogPath != o.LogPath ||
		a.P2P.MaxPeers != o.P2P.MaxPeers ||
		a.P2P.MinPeers != o.P2P.MinPeers ||
		a.P2P.PingInterval != o.P2P.PingInterval ||
		a.P2P.PingTimeout != o.P2P.PingTimeout ||
		a.P2P.ProtoTickInterval != o.P2P.ProtoTickInterval ||
		a.Relay != o.Relay {
		return false
	}
	return true
}

// AnnounceableAddress is a pair of node address in the form of "[host]:[port]"
// with optional corresponding announced port to be used in version exchange.
type AnnounceableAddress struct {
	Address       string
	AnnouncedPort uint16
}

// GetAddresses parses returns the list of AnnounceableAddress containing information
// gathered from Addresses.
func (a *ApplicationConfiguration) GetAddresses() ([]AnnounceableAddress, error) {
	addrs := make([]AnnounceableAddress, 0, len(a.P2P.Addresses))
	for i, addrStr := range a.P2P.Addresses {
		if len(addrStr) == 0 {
			return nil, fmt.Errorf("address #%d is empty", i)
		}
		lastCln := strings.LastIndex(addrStr, ":")
		if lastCln == -1 {
			addrs = append(addrs, AnnounceableAddress{
				Address: addrStr, // Plain IPv4 address without port.
			})
			continue
		}
		lastPort, err := strconv.ParseUint(addrStr[lastCln+1:], 10, 16)
		if err != nil {
			addrs = append(addrs, AnnounceableAddress{
				Address: addrStr, // Still may be a valid IPv4 of the form "X.Y.Z.Q:" or plain IPv6 "A:B::", keep it.
			})
			continue
		}
		penultimateCln := strings.LastIndex(addrStr[:lastCln], ":")
		if penultimateCln == -1 {
			addrs = append(addrs, AnnounceableAddress{
				Address: addrStr, // IPv4 address with port "X.Y.Z.Q:123"
			})
			continue
		}
		isV6 := strings.Count(addrStr, ":") > 2
		hasBracket := strings.Contains(addrStr, "]")
		if penultimateCln == lastCln-1 {
			if isV6 && !hasBracket {
				addrs = append(addrs, AnnounceableAddress{
					Address: addrStr, // Plain IPv6 of the form "A:B::123"
				})
			} else {
				addrs = append(addrs, AnnounceableAddress{
					Address:       addrStr[:lastCln], // IPv4 with empty port and non-empty announced port "X.Y.Z.Q::123" or IPv6 with non-empty announced port "[A:B::]::123".
					AnnouncedPort: uint16(lastPort),
				})
			}
			continue
		}
		_, err = strconv.ParseUint(addrStr[penultimateCln+1:lastCln], 10, 16)
		if err != nil {
			if isV6 {
				addrs = append(addrs, AnnounceableAddress{
					Address: addrStr, // Still may be a valid plain IPv6 of the form "A::B:123" or IPv6 with single port [A:B::]:123, keep it.
				})
				continue
			}
			return nil, fmt.Errorf("failed to parse port from %s: %w", addrStr, err) // Some garbage.
		}
		if isV6 && !hasBracket {
			addrs = append(addrs, AnnounceableAddress{
				Address: addrStr, // Plain IPv6 of the form "A::1:1"
			})
		} else {
			addrs = append(addrs, AnnounceableAddress{
				Address:       addrStr[:lastCln], // IPv4 with both ports or IPv6 with both ports specified.
				AnnouncedPort: uint16(lastPort),
			})
		}
	}
	if len(addrs) == 0 {
		addrs = append(addrs, AnnounceableAddress{
			Address: ":0",
		})
	}
	return addrs, nil
}

// Validate checks ApplicationConfiguration for internal consistency and returns
// an error if any invalid settings are found. This ensures that the application
// configuration is valid and safe to use for further operations.
func (a *ApplicationConfiguration) Validate() error {
	if err := a.NeoFSBlockFetcher.Validate(); err != nil {
		return fmt.Errorf("invalid NeoFSBlockFetcher config: %w", err)
	}
	if err := a.RPC.Validate(); err != nil {
		return fmt.Errorf("invalid RPC config: %w", err)
	}
	if err := a.Logger.Validate(); err != nil {
		return fmt.Errorf("invalid logger config: %w", err)
	}
	return nil
}
