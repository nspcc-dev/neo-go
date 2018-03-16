package core

import (
	"github.com/CityOfZion/neo-go/pkg/util"
)

// Utilities for quick bootstrapping blockchains. Normally we should
// create the genisis block. For now (to speed up development) we will add
// The hashes manually.

func GenesisHashPrivNet() util.Uint256 {
	hash, _ := util.Uint256DecodeString("996e37358dc369912041f966f8c5d8d3a8255ba5dcbd3447f8a82b55db869099")
	return hash
}

func GenesisHashTestNet() util.Uint256 {
	hash, _ := util.Uint256DecodeString("b3181718ef6167105b70920e4a8fbbd0a0a56aacf460d70e10ba6fa1668f1fef")
	return hash
}

func GenesisHashMainNet() util.Uint256 {
	hash, _ := util.Uint256DecodeString("d42561e3d30e15be6400b6df2f328e02d2bf6354c41dce433bc57687c82144bf")
	return hash
}

// headerSliceReverse reverses the given slice of *Header.
func headerSliceReverse(dest []*Header) {
	for i, j := 0, len(dest)-1; i < j; i, j = i+1, j-1 {
		dest[i], dest[j] = dest[j], dest[i]
	}
}
