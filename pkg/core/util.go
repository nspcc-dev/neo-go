package core

import "github.com/CityOfZion/neo-go/pkg/util"

// Utilities for quick bootstrapping blockchains. Normally we should
// create the genisis block. For now (to speed up development) we will add
// The hashes manually.

func GenesisHashPrivNet() util.Uint256 {
	hash, _ := util.Uint256DecodeString("996e37358dc369912041f966f8c5d8d3a8255ba5dcbd3447f8a82b55db869099")
	return hash
}
