package result

import (
	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/nspcc-dev/neo-go/pkg/encoding/fixedn"
)

type (
	// Version model used for reporting server version
	// info.
	Version struct {
		// Magic contains network magic.
		// Deprecated: use Protocol.StateRootInHeader instead
		Magic     netmode.Magic `json:"network"`
		TCPPort   uint16        `json:"tcpport"`
		WSPort    uint16        `json:"wsport,omitempty"`
		Nonce     uint32        `json:"nonce"`
		UserAgent string        `json:"useragent"`
		Protocol  Protocol      `json:"protocol"`
		// StateRootInHeader is true if state root is contained in block header.
		// Deprecated: use Protocol.StateRootInHeader instead
		StateRootInHeader bool `json:"staterootinheader,omitempty"`
	}

	// Protocol represents network-dependent parameters.
	Protocol struct {
		AddressVersion              byte          `json:"addressversion"`
		Network                     netmode.Magic `json:"network"`
		MillisecondsPerBlock        int           `json:"msperblock"`
		MaxTraceableBlocks          uint32        `json:"maxtraceableblocks"`
		MaxValidUntilBlockIncrement uint32        `json:"maxvaliduntilblockincrement"`
		MaxTransactionsPerBlock     uint16        `json:"maxtransactionsperblock"`
		MemoryPoolMaxTransactions   int           `json:"memorypoolmaxtransactions"`
		ValidatorsCount             byte          `json:"validatorscount"`
		InitialGasDistribution      fixedn.Fixed8 `json:"initialgasdistribution"`
		// StateRootInHeader is true if state root is contained in block header.
		StateRootInHeader bool `json:"staterootinheader,omitempty"`
	}
)
