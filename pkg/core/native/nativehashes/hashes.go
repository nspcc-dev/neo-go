package nativehashes

import (
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativenames"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// Hashes of all native contracts.
var (
	Management  util.Uint160
	Ledger      util.Uint160
	Neo         util.Uint160
	Gas         util.Uint160
	Policy      util.Uint160
	Oracle      util.Uint160
	Designation util.Uint160
	Notary      util.Uint160
	CryptoLib   util.Uint160
	StdLib      util.Uint160
)

func init() {
	Management = state.CreateNativeContractHash(nativenames.Management)
	Ledger = state.CreateNativeContractHash(nativenames.Ledger)
	Neo = state.CreateNativeContractHash(nativenames.Neo)
	Gas = state.CreateNativeContractHash(nativenames.Gas)
	Policy = state.CreateNativeContractHash(nativenames.Policy)
	Oracle = state.CreateNativeContractHash(nativenames.Oracle)
	Designation = state.CreateNativeContractHash(nativenames.Designation)
	Notary = state.CreateNativeContractHash(nativenames.Notary)
	CryptoLib = state.CreateNativeContractHash(nativenames.CryptoLib)
	StdLib = state.CreateNativeContractHash(nativenames.StdLib)
}
