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
	// Deprecated: please, use Addresses instead, this field will be removed in future versions.
	Address *string `yaml:"Address,omitempty"`
	// Addresses stores the node address list in the form of "[host]:[port][:announcedPort]".
	Addresses []string `yaml:"Addresses"`
	// Deprecated: please, use Addresses instead, this field will be removed in future versions.
	AnnouncedNodePort *uint16 `yaml:"AnnouncedPort,omitempty"`
	AttemptConnPeers  int     `yaml:"AttemptConnPeers"`
	// BroadcastFactor is the factor (0-100) controlling gossip fan-out number optimization.
	BroadcastFactor int                      `yaml:"BroadcastFactor"`
	DBConfiguration dbconfig.DBConfiguration `yaml:"DBConfiguration"`
	DialTimeout     int64                    `yaml:"DialTimeout"`
	LogLevel        string                   `yaml:"LogLevel"`
	LogPath         string                   `yaml:"LogPath"`
	MaxPeers        int                      `yaml:"MaxPeers"`
	MinPeers        int                      `yaml:"MinPeers"`
	// Deprecated: please, use Addresses instead, this field will be removed in future versions.
	NodePort          *uint16             `yaml:"NodePort,omitempty"`
	PingInterval      int64               `yaml:"PingInterval"`
	PingTimeout       int64               `yaml:"PingTimeout"`
	Pprof             BasicService        `yaml:"Pprof"`
	Prometheus        BasicService        `yaml:"Prometheus"`
	ProtoTickInterval int64               `yaml:"ProtoTickInterval"`
	Relay             bool                `yaml:"Relay"`
	RPC               RPC                 `yaml:"RPC"`
	UnlockWallet      Wallet              `yaml:"UnlockWallet"`
	Oracle            OracleConfiguration `yaml:"Oracle"`
	P2PNotary         P2PNotary           `yaml:"P2PNotary"`
	StateRoot         StateRoot           `yaml:"StateRoot"`
	// ExtensiblePoolSize is the maximum amount of the extensible payloads from a single sender.
	ExtensiblePoolSize int `yaml:"ExtensiblePoolSize"`
}

// EqualsButServices returns true when the o is the same as a except for services
// (Oracle, P2PNotary, Pprof, Prometheus, RPC, StateRoot and UnlockWallet sections)
// and LogLevel field.
func (a *ApplicationConfiguration) EqualsButServices(o *ApplicationConfiguration) bool {
	if len(a.Addresses) != len(o.Addresses) {
		return false
	}
	aCp := make([]string, len(a.Addresses))
	oCp := make([]string, len(o.Addresses))
	copy(aCp, a.Addresses)
	copy(oCp, o.Addresses)
	sort.Strings(aCp)
	sort.Strings(oCp)
	for i := range aCp {
		if aCp[i] != oCp[i] {
			return false
		}
	}
	if a.Address != o.Address ||
		a.AnnouncedNodePort != o.AnnouncedNodePort ||
		a.AttemptConnPeers != o.AttemptConnPeers ||
		a.BroadcastFactor != o.BroadcastFactor ||
		a.DBConfiguration != o.DBConfiguration ||
		a.DialTimeout != o.DialTimeout ||
		a.ExtensiblePoolSize != o.ExtensiblePoolSize ||
		a.LogPath != o.LogPath ||
		a.MaxPeers != o.MaxPeers ||
		a.MinPeers != o.MinPeers ||
		a.NodePort != o.NodePort ||
		a.PingInterval != o.PingInterval ||
		a.PingTimeout != o.PingTimeout ||
		a.ProtoTickInterval != o.ProtoTickInterval ||
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
	addrs := make([]AnnounceableAddress, 0, len(a.Addresses)+1)
	if a.Address != nil || a.NodePort != nil || a.AnnouncedNodePort != nil {
		var (
			host     string
			nodePort uint16
		)
		if a.Address != nil {
			host = *a.Address
		}
		if a.NodePort != nil {
			nodePort = *a.NodePort
		}
		addr := AnnounceableAddress{Address: net.JoinHostPort(host, strconv.Itoa(int(nodePort)))}
		if a.AnnouncedNodePort != nil {
			addr.AnnouncedPort = *a.AnnouncedNodePort
		}
		addrs = append(addrs, addr)
	}
	for i, addrStr := range a.Addresses {
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
