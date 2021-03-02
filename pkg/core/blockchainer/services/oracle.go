package services

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/crypto/keys"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Oracle specifies oracle service interface.
type Oracle interface {
	// AddRequests processes new requests.
	AddRequests(map[uint64]*state.OracleRequest)
	// RemoveRequests removes already processed requests.
	RemoveRequests([]uint64)
	// UpdateOracleNodes updates oracle nodes.
	UpdateOracleNodes(keys.PublicKeys)
	// UpdateNativeContract updates oracle contract native script and hash.
	UpdateNativeContract([]byte, []byte, util.Uint160, int)
	// Run runs oracle module. Must be invoked in a separate goroutine.
	Run()
	// Shutdown shutdowns oracle module.
	Shutdown()
}
