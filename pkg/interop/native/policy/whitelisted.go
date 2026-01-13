package policy

import "github.com/nspcc-dev/neo-go/pkg/interop"

// WhitelistFeeContract represents a whitelisted contract method with fixed
// execution price. Iterator values returned from GetWhitelistFeeContracts can
// be directly cast to WhitelistFeeContract.
type WhitelistFeeContract struct {
	Hash   interop.Hash160
	Method string
	ArgCnt int
	Fee    int
}
