package config

import (
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/core/storage/dbconfig"
)

// ApplicationConfiguration config specific to the node.
type ApplicationConfiguration struct {
	// Deprecated: please, use Addresses field of P2P section instead, this field will be removed in future versions.
	Address *string `yaml:"Address,omitempty"`
	// Deprecated: please, use Addresses field of P2P section instead, this field will be removed in future versions.
	AnnouncedNodePort *uint16 `yaml:"AnnouncedPort,omitempty"`
	// Deprecated: this option is moved to the P2P section.
	AttemptConnPeers int `yaml:"AttemptConnPeers"`
	// BroadcastFactor is the factor (0-100) controlling gossip fan-out number optimization.
	//
	// Deprecated: this option is moved to the P2P section.
	BroadcastFactor int                      `yaml:"BroadcastFactor"`
	DBConfiguration dbconfig.DBConfiguration `yaml:"DBConfiguration"`
	// Deprecated: this option is moved to the P2P section.
	DialTimeout int64  `yaml:"DialTimeout"`
	LogLevel    string `yaml:"LogLevel"`
	LogPath     string `yaml:"LogPath"`
	// Deprecated: this option is moved to the P2P section.
	MaxPeers int `yaml:"MaxPeers"`
	// Deprecated: this option is moved to the P2P section.
	MinPeers int `yaml:"MinPeers"`
	// Deprecated: please, use Addresses field of P2P section instead, this field will be removed in future versions.
	NodePort *uint16 `yaml:"NodePort,omitempty"`
	P2P      P2P     `yaml:"P2P"`
	// Deprecated: this option is moved to the P2P section.
	PingInterval int64 `yaml:"PingInterval"`
	// Deprecated: this option is moved to the P2P section.
	PingTimeout int64        `yaml:"PingTimeout"`
	Pprof       BasicService `yaml:"Pprof"`
	Prometheus  BasicService `yaml:"Prometheus"`
	// Deprecated: this option is moved to the P2P section.
	ProtoTickInterval int64               `yaml:"ProtoTickInterval"`
	Relay             bool                `yaml:"Relay"`
	RPC               RPC                 `yaml:"RPC"`
	UnlockWallet      Wallet              `yaml:"UnlockWallet"`
	Oracle            OracleConfiguration `yaml:"Oracle"`
	P2PNotary         P2PNotary           `yaml:"P2PNotary"`
	StateRoot         StateRoot           `yaml:"StateRoot"`
	// ExtensiblePoolSize is the maximum amount of the extensible payloads from a single sender.
	//
	// Deprecated: this option is moved to the P2P section.
	ExtensiblePoolSize int `yaml:"ExtensiblePoolSize"`
}

// EqualsButServices returns true when the o is the same as a except for services
// (Oracle, P2PNotary, Pprof, Prometheus, RPC, StateRoot and UnlockWallet sections)
// and LogLevel field.
func (a *ApplicationConfiguration) EqualsButServices(o *ApplicationConfiguration) bool {
	if len(a.P2P.Addresses) != len(o.P2P.Addresses) {
		return false
	}
	aCp := make([]string, len(a.P2P.Addresses))
	oCp := make([]string, len(o.P2P.Addresses))
	copy(aCp, a.P2P.Addresses)
	copy(oCp, o.P2P.Addresses)
	sort.Strings(aCp)
	sort.Strings(oCp)
	for i := range aCp {
		if aCp[i] != oCp[i] {
			return false
		}
	}
	if a.Address != o.Address || //nolint:staticcheck // SA1019: a.Address is deprecated
		a.AnnouncedNodePort != o.AnnouncedNodePort || //nolint:staticcheck // SA1019: a.AnnouncedNodePort is deprecated
		a.AttemptConnPeers != o.AttemptConnPeers || //nolint:staticcheck // SA1019: a.AttemptConnPeers is deprecated
		a.P2P.AttemptConnPeers != o.P2P.AttemptConnPeers ||
		a.BroadcastFactor != o.BroadcastFactor || //nolint:staticcheck // SA1019: a.BroadcastFactor is deprecated
		a.P2P.BroadcastFactor != o.P2P.BroadcastFactor ||
		a.DBConfiguration != o.DBConfiguration ||
		a.DialTimeout != o.DialTimeout || //nolint:staticcheck // SA1019: a.DialTimeout is deprecated
		a.P2P.DialTimeout != o.P2P.DialTimeout ||
		a.ExtensiblePoolSize != o.ExtensiblePoolSize || //nolint:staticcheck // SA1019: a.ExtensiblePoolSize is deprecated
		a.P2P.ExtensiblePoolSize != o.P2P.ExtensiblePoolSize ||
		a.LogPath != o.LogPath ||
		a.MaxPeers != o.MaxPeers || //nolint:staticcheck // SA1019: a.MaxPeers is deprecated
		a.P2P.MaxPeers != o.P2P.MaxPeers ||
		a.MinPeers != o.MinPeers || //nolint:staticcheck // SA1019: a.MinPeers is deprecated
		a.P2P.MinPeers != o.P2P.MinPeers ||
		a.NodePort != o.NodePort || //nolint:staticcheck // SA1019: a.NodePort is deprecated
		a.PingInterval != o.PingInterval || //nolint:staticcheck // SA1019: a.PingInterval is deprecated
		a.P2P.PingInterval != o.P2P.PingInterval ||
		a.PingTimeout != o.PingTimeout || //nolint:staticcheck // SA1019: a.PingTimeout is deprecated
		a.P2P.PingTimeout != o.P2P.PingTimeout ||
		a.ProtoTickInterval != o.ProtoTickInterval || //nolint:staticcheck // SA1019: a.ProtoTickInterval is deprecated
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
// gathered from both deprecated Address / NodePort / AnnouncedNodePort and newly
// created Addresses fields.
func (a *ApplicationConfiguration) GetAddresses() ([]AnnounceableAddress, error) {
	addrs := make([]AnnounceableAddress, 0, len(a.P2P.Addresses)+1)
	if a.Address != nil || a.NodePort != nil || a.AnnouncedNodePort != nil { //nolint:staticcheck // SA1019: a.Address is deprecated
		var (
			host     string
			nodePort uint16
		)
		if a.Address != nil { //nolint:staticcheck // SA1019: a.Address is deprecated
			host = *a.Address //nolint:staticcheck // SA1019: a.Address is deprecated
		}
		if a.NodePort != nil { //nolint:staticcheck // SA1019: a.NodePort is deprecated
			nodePort = *a.NodePort //nolint:staticcheck // SA1019: a.NodePort is deprecated
		}
		addr := AnnounceableAddress{Address: net.JoinHostPort(host, strconv.Itoa(int(nodePort)))}
		if a.AnnouncedNodePort != nil { //nolint:staticcheck // SA1019: a.AnnouncedNodePort is deprecated
			addr.AnnouncedPort = *a.AnnouncedNodePort //nolint:staticcheck // SA1019: a.AnnouncedNodePort is deprecated
		}
		addrs = append(addrs, addr)
	}
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
