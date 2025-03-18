package result

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
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
		RPC       RPC      `json:"rpc"`
	}

	// RPC represents the RPC server configuration.
	RPC struct {
		MaxIteratorResultItems int  `json:"maxiteratorresultitems"`
		SessionEnabled         bool `json:"sessionenabled"`

		// Below are NeoGo-specific extensions to the JSON-RPC protocol that are
		// returned by the server in case they're enabled.

		// SessionExpansionEnabled is true if the server supports iterator expansion.
		SessionExpansionEnabled bool `json:"sessionexpansionenabled,omitempty"`
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
		// Hardforks is the map of network hardforks with the enabling height.
		Hardforks        map[config.Hardfork]uint32
		StandbyCommittee keys.PublicKeys
		SeedList         []string

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
		Hardforks                   []hardforkAux `json:"hardforks"`
		StandbyCommittee            []string      `json:"standbycommittee"`
		SeedList                    []string      `json:"seedlist"`

		CommitteeHistory  map[uint32]uint32 `json:"committeehistory,omitempty"`
		P2PSigExtensions  bool              `json:"p2psigextensions,omitempty"`
		StateRootInHeader bool              `json:"staterootinheader,omitempty"`
		ValidatorsHistory map[uint32]uint32 `json:"validatorshistory,omitempty"`
	}

	// hardforkAux is an auxiliary struct used for Hardfork JSON marshalling.
	hardforkAux struct {
		Name   string `json:"name"`
		Height uint32 `json:"blockheight"`
	}
)

// prefixHardfork is a prefix used for hardfork names in C# node.
const prefixHardfork = "HF_"

// MarshalJSON implements the JSON marshaler interface.
func (p Protocol) MarshalJSON() ([]byte, error) {
	// Keep hardforks sorted by name in the result.
	hfs := make([]hardforkAux, 0, len(p.Hardforks))
	for _, hf := range config.Hardforks {
		if h, ok := p.Hardforks[hf]; ok {
			hfs = append(hfs, hardforkAux{
				Name:   hf.String(),
				Height: h,
			})
		}
	}
	standbyCommittee := make([]string, len(p.StandbyCommittee))
	for i, key := range p.StandbyCommittee {
		standbyCommittee[i] = key.StringCompressed()
	}

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
		Hardforks:                   hfs,
		StandbyCommittee:            standbyCommittee,
		SeedList:                    p.SeedList,

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
	standbyCommittee, err := keys.NewPublicKeysFromStrings(aux.StandbyCommittee)
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
	p.StandbyCommittee = standbyCommittee
	p.SeedList = aux.SeedList

	// Filter out unknown hardforks.
	for i := range aux.Hardforks {
		aux.Hardforks[i].Name = strings.TrimPrefix(aux.Hardforks[i].Name, prefixHardfork)
		if !config.IsHardforkValid(aux.Hardforks[i].Name) {
			return fmt.Errorf("unexpected hardfork: %s", aux.Hardforks[i].Name)
		}
	}
	p.Hardforks = make(map[config.Hardfork]uint32, len(aux.Hardforks))
	for _, cfgHf := range config.Hardforks {
		for _, auxHf := range aux.Hardforks {
			if auxHf.Name == cfgHf.String() {
				p.Hardforks[cfgHf] = auxHf.Height
			}
		}
	}

	return nil
}
