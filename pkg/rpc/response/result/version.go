package result

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coreos/go-semver/semver"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
)

type (
	// Version model used for reporting server version
	// info.
	Version struct {
		// Magic contains network magic.
		// Deprecated: use Protocol.StateRootInHeader instead
		Magic     netmode.Magic
		TCPPort   uint16
		WSPort    uint16
		Nonce     uint32
		UserAgent string
		Protocol  Protocol
		// StateRootInHeader is true if state root is contained in the block header.
		// Deprecated: use Protocol.StateRootInHeader instead
		StateRootInHeader bool
	}

	// Protocol represents network-dependent parameters.
	Protocol struct {
		AddressVersion              byte
		Network                     netmode.Magic
		MillisecondsPerBlock        int
		MaxTraceableBlocks          uint32
		MaxValidUntilBlockIncrement uint32
		MaxTransactionsPerBlock     uint16
		MemoryPoolMaxTransactions   int
		ValidatorsCount             byte
		InitialGasDistribution      fixedn.Fixed8
		// StateRootInHeader is true if state root is contained in block header.
		StateRootInHeader bool
	}
)

type (
	// versionMarshallerAux is an auxiliary struct used for Version JSON marshalling.
	versionMarshallerAux struct {
		Magic             netmode.Magic         `json:"network"`
		TCPPort           uint16                `json:"tcpport"`
		WSPort            uint16                `json:"wsport,omitempty"`
		Nonce             uint32                `json:"nonce"`
		UserAgent         string                `json:"useragent"`
		Protocol          protocolMarshallerAux `json:"protocol"`
		StateRootInHeader bool                  `json:"staterootinheader,omitempty"`
	}

	// protocolMarshallerAux is an auxiliary struct used for Protocol JSON marshalling.
	protocolMarshallerAux struct {
		AddressVersion              byte          `json:"addressversion"`
		Network                     netmode.Magic `json:"network"`
		MillisecondsPerBlock        int           `json:"msperblock"`
		MaxTraceableBlocks          uint32        `json:"maxtraceableblocks"`
		MaxValidUntilBlockIncrement uint32        `json:"maxvaliduntilblockincrement"`
		MaxTransactionsPerBlock     uint16        `json:"maxtransactionsperblock"`
		MemoryPoolMaxTransactions   int           `json:"memorypoolmaxtransactions"`
		ValidatorsCount             byte          `json:"validatorscount"`
		InitialGasDistribution      int64         `json:"initialgasdistribution"`
		StateRootInHeader           bool          `json:"staterootinheader,omitempty"`
	}

	// versionUnmarshallerAux is an auxiliary struct used for Version JSON unmarshalling.
	versionUnmarshallerAux struct {
		Magic             netmode.Magic           `json:"network"`
		TCPPort           uint16                  `json:"tcpport"`
		WSPort            uint16                  `json:"wsport,omitempty"`
		Nonce             uint32                  `json:"nonce"`
		UserAgent         string                  `json:"useragent"`
		Protocol          protocolUnmarshallerAux `json:"protocol"`
		StateRootInHeader bool                    `json:"staterootinheader,omitempty"`
	}

	// protocolUnmarshallerAux is an auxiliary struct used for Protocol JSON unmarshalling.
	protocolUnmarshallerAux struct {
		AddressVersion              byte            `json:"addressversion"`
		Network                     netmode.Magic   `json:"network"`
		MillisecondsPerBlock        int             `json:"msperblock"`
		MaxTraceableBlocks          uint32          `json:"maxtraceableblocks"`
		MaxValidUntilBlockIncrement uint32          `json:"maxvaliduntilblockincrement"`
		MaxTransactionsPerBlock     uint16          `json:"maxtransactionsperblock"`
		MemoryPoolMaxTransactions   int             `json:"memorypoolmaxtransactions"`
		ValidatorsCount             byte            `json:"validatorscount"`
		InitialGasDistribution      json.RawMessage `json:"initialgasdistribution"`
		StateRootInHeader           bool            `json:"staterootinheader,omitempty"`
	}
)

// latestNonBreakingVersion is a latest NeoGo revision that keeps older RPC
// clients compatibility with newer RPC servers (https://github.com/nspcc-dev/neo-go/pull/2435).
var latestNonBreakingVersion = *semver.New("0.98.2")

// MarshalJSON implements the json marshaller interface.
func (v *Version) MarshalJSON() ([]byte, error) {
	aux := versionMarshallerAux{
		Magic:     v.Magic,
		TCPPort:   v.TCPPort,
		WSPort:    v.WSPort,
		Nonce:     v.Nonce,
		UserAgent: v.UserAgent,
		Protocol: protocolMarshallerAux{
			AddressVersion:              v.Protocol.AddressVersion,
			Network:                     v.Protocol.Network,
			MillisecondsPerBlock:        v.Protocol.MillisecondsPerBlock,
			MaxTraceableBlocks:          v.Protocol.MaxTraceableBlocks,
			MaxValidUntilBlockIncrement: v.Protocol.MaxValidUntilBlockIncrement,
			MaxTransactionsPerBlock:     v.Protocol.MaxTransactionsPerBlock,
			MemoryPoolMaxTransactions:   v.Protocol.MemoryPoolMaxTransactions,
			ValidatorsCount:             v.Protocol.ValidatorsCount,
			InitialGasDistribution:      int64(v.Protocol.InitialGasDistribution),
			StateRootInHeader:           v.Protocol.StateRootInHeader,
		},
		StateRootInHeader: v.StateRootInHeader,
	}
	return json.Marshal(aux)
}

// UnmarshalJSON implements the json unmarshaller interface.
func (v *Version) UnmarshalJSON(data []byte) error {
	var aux versionUnmarshallerAux
	err := json.Unmarshal(data, &aux)
	if err != nil {
		return err
	}
	v.Magic = aux.Magic
	v.TCPPort = aux.TCPPort
	v.WSPort = aux.WSPort
	v.Nonce = aux.Nonce
	v.UserAgent = aux.UserAgent
	v.Protocol.AddressVersion = aux.Protocol.AddressVersion
	v.Protocol.Network = aux.Protocol.Network
	v.Protocol.MillisecondsPerBlock = aux.Protocol.MillisecondsPerBlock
	v.Protocol.MaxTraceableBlocks = aux.Protocol.MaxTraceableBlocks
	v.Protocol.MaxValidUntilBlockIncrement = aux.Protocol.MaxValidUntilBlockIncrement
	v.Protocol.MaxTransactionsPerBlock = aux.Protocol.MaxTransactionsPerBlock
	v.Protocol.MemoryPoolMaxTransactions = aux.Protocol.MemoryPoolMaxTransactions
	v.Protocol.ValidatorsCount = aux.Protocol.ValidatorsCount
	v.Protocol.StateRootInHeader = aux.Protocol.StateRootInHeader
	v.StateRootInHeader = aux.StateRootInHeader
	if len(aux.Protocol.InitialGasDistribution) == 0 {
		return nil
	}

	if strings.HasPrefix(v.UserAgent, config.UserAgentWrapper+config.UserAgentPrefix) {
		ver, err := userAgentToVersion(v.UserAgent)
		if err == nil && ver.Compare(latestNonBreakingVersion) <= 0 {
			err := json.Unmarshal(aux.Protocol.InitialGasDistribution, &v.Protocol.InitialGasDistribution)
			if err != nil {
				return fmt.Errorf("failed to unmarshal InitialGASDistribution into fixed8: %w", err)
			}
			return nil
		}
	}
	var val int64
	err = json.Unmarshal(aux.Protocol.InitialGasDistribution, &val)
	if err != nil {
		return fmt.Errorf("failed to unmarshal InitialGASDistribution into int64: %w", err)
	}
	v.Protocol.InitialGasDistribution = fixedn.Fixed8(val)

	return nil
}

func userAgentToVersion(userAgent string) (*semver.Version, error) {
	verStr := strings.Trim(userAgent, config.UserAgentWrapper)
	verStr = strings.TrimPrefix(verStr, config.UserAgentPrefix)
	ver, err := semver.NewVersion(verStr)
	if err != nil {
		return nil, fmt.Errorf("can't retrieve neo-go version from UserAgent: %w", err)
	}
	return ver, nil
}
