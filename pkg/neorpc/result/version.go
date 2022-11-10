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
		TCPPort   uint16
		WSPort    uint16
		Nonce     uint32
		UserAgent string
		Protocol  Protocol
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
		CommitteeHistory map[uint32]int
		// P2PSigExtensions is true when Notary subsystem is enabled on the network.
		P2PSigExtensions bool
		// StateRootInHeader is true if state root is contained in block header.
		StateRootInHeader bool
		// ValidatorsHistory stores height:size map of the validators count.
		ValidatorsHistory map[uint32]int
	}
)

type (
	// versionMarshallerAux is an auxiliary struct used for Version JSON marshalling.
	versionMarshallerAux struct {
		TCPPort   uint16                `json:"tcpport"`
		WSPort    uint16                `json:"wsport,omitempty"`
		Nonce     uint32                `json:"nonce"`
		UserAgent string                `json:"useragent"`
		Protocol  protocolMarshallerAux `json:"protocol"`
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

		CommitteeHistory  map[uint32]int `json:"committeehistory,omitempty"`
		P2PSigExtensions  bool           `json:"p2psigextensions,omitempty"`
		StateRootInHeader bool           `json:"staterootinheader,omitempty"`
		ValidatorsHistory map[uint32]int `json:"validatorshistory,omitempty"`
	}
)

// MarshalJSON implements the json marshaller interface.
func (v *Version) MarshalJSON() ([]byte, error) {
	aux := versionMarshallerAux{
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

			CommitteeHistory:  v.Protocol.CommitteeHistory,
			P2PSigExtensions:  v.Protocol.P2PSigExtensions,
			StateRootInHeader: v.Protocol.StateRootInHeader,
			ValidatorsHistory: v.Protocol.ValidatorsHistory,
		},
	}
	return json.Marshal(aux)
}

// UnmarshalJSON implements the json unmarshaller interface.
func (v *Version) UnmarshalJSON(data []byte) error {
	var aux versionMarshallerAux
	err := json.Unmarshal(data, &aux)
	if err != nil {
		return err
	}
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
	v.Protocol.CommitteeHistory = aux.Protocol.CommitteeHistory
	v.Protocol.P2PSigExtensions = aux.Protocol.P2PSigExtensions
	v.Protocol.StateRootInHeader = aux.Protocol.StateRootInHeader
	v.Protocol.ValidatorsHistory = aux.Protocol.ValidatorsHistory
	v.Protocol.InitialGasDistribution = fixedn.Fixed8(aux.Protocol.InitialGasDistribution)

	return nil
}
