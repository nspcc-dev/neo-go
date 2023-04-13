package result

import (
	"encoding/json"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
)

type (
	// Version model used for reporting server version
	// info.
	Version struct {
		TCPPort   uint16   `json:"tcpport"`
		WSPort    uint16   `json:"wsport,omitempty"`
		Nonce     uint32   `json:"nonce"`
		UserAgent string   `json:"useragent"`
		Protocol  Protocol `json:"protocol"`
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

		// Below are NeoGo-specific extensions to the protocol that are
		// returned by the server in case they're enabled.

		// CommitteeHistory stores height:size map of the committee size.
		CommitteeHistory map[uint32]uint32
		// P2PSigExtensions is true when Notary subsystem is enabled on the network.
		P2PSigExtensions bool
		// StateRootInHeader is true if state root is contained in block header.
		StateRootInHeader bool
		// ValidatorsHistory stores height:size map of the validators count.
		ValidatorsHistory map[uint32]uint32
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

		CommitteeHistory  map[uint32]uint32 `json:"committeehistory,omitempty"`
		P2PSigExtensions  bool              `json:"p2psigextensions,omitempty"`
		StateRootInHeader bool              `json:"staterootinheader,omitempty"`
		ValidatorsHistory map[uint32]uint32 `json:"validatorshistory,omitempty"`
	}
)

// MarshalJSON implements the JSON marshaler interface.
func (p Protocol) MarshalJSON() ([]byte, error) {
	aux := protocolMarshallerAux{
		AddressVersion:              p.AddressVersion,
		Network:                     p.Network,
		MillisecondsPerBlock:        p.MillisecondsPerBlock,
		MaxTraceableBlocks:          p.MaxTraceableBlocks,
		MaxValidUntilBlockIncrement: p.MaxValidUntilBlockIncrement,
		MaxTransactionsPerBlock:     p.MaxTransactionsPerBlock,
		MemoryPoolMaxTransactions:   p.MemoryPoolMaxTransactions,
		ValidatorsCount:             p.ValidatorsCount,
		InitialGasDistribution:      int64(p.InitialGasDistribution),

		CommitteeHistory:  p.CommitteeHistory,
		P2PSigExtensions:  p.P2PSigExtensions,
		StateRootInHeader: p.StateRootInHeader,
		ValidatorsHistory: p.ValidatorsHistory,
	}
	return json.Marshal(aux)
}

// UnmarshalJSON implements the JSON unmarshaler interface.
func (p *Protocol) UnmarshalJSON(data []byte) error {
	var aux protocolMarshallerAux
	err := json.Unmarshal(data, &aux)
	if err != nil {
		return err
	}
	p.AddressVersion = aux.AddressVersion
	p.Network = aux.Network
	p.MillisecondsPerBlock = aux.MillisecondsPerBlock
	p.MaxTraceableBlocks = aux.MaxTraceableBlocks
	p.MaxValidUntilBlockIncrement = aux.MaxValidUntilBlockIncrement
	p.MaxTransactionsPerBlock = aux.MaxTransactionsPerBlock
	p.MemoryPoolMaxTransactions = aux.MemoryPoolMaxTransactions
	p.ValidatorsCount = aux.ValidatorsCount
	p.CommitteeHistory = aux.CommitteeHistory
	p.P2PSigExtensions = aux.P2PSigExtensions
	p.StateRootInHeader = aux.StateRootInHeader
	p.ValidatorsHistory = aux.ValidatorsHistory
	p.InitialGasDistribution = fixedn.Fixed8(aux.InitialGasDistribution)

	return nil
}
