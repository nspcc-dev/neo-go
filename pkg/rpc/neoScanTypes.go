package rpc

import (
	"github.com/CityOfZion/neo-go/pkg/core/entities"
	"github.com/CityOfZion/neo-go/pkg/util"
)

/*
	Definition of types, helper functions and variables
	required for calculation of transaction inputs using
	NeoScan API.
*/

type (
	// NeoScanServer stores NEOSCAN URL and API path.
	NeoScanServer struct {
		URL  string // "protocol://host:port/"
		Path string // path to API endpoint without wallet address
	}

	// Unspent stores Unspents per asset
	Unspent struct {
		Unspent entities.UnspentBalances
		Asset   string      // "NEO" / "GAS"
		Amount  util.Fixed8 // total unspent of this asset
	}

	// NeoScanBalance is a struct of NeoScan response to 'get_balance' request
	NeoScanBalance struct {
		Balance []*Unspent
		Address string
	}
)
