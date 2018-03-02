package wallet

import (
	"math/big"

	"github.com/anthdm/neo-go/pkg/util"
)

// TransferOutput respresents the output of a transaction.
type TransferOutput struct {
	// The asset identifier. This should be of type Uint256.
	// NEO governing token: c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b
	// NEO gas: 			602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7
	AssetID util.Uint256

	// Value of the transfer
	Value *big.Int

	// ScriptHash of the transfer.
	ScriptHash util.Uint160
}
