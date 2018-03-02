package core

import (
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// TransactionResult represents the output of a transaction.
type TransactionResult struct {
	// The NEO asset id used in the transaction.
	AssetID util.Uint256

	// Amount of AssetType send or received.
	Amount *big.Int
}
