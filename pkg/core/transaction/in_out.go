package transaction

import (
	"math/big"

	"github.com/CityOfZion/neo-go/pkg/util"
)

// Output represents a Transaction output.
type Output struct {
	// The NEO asset id used in the transaction.
	AssetID util.Uint256

	// Amount of AssetType send or received.
	Amount *big.Int
}

// Input represents a Transaction input.
type Input struct {
}
