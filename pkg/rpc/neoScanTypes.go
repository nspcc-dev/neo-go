package rpc

import "github.com/CityOfZion/neo-go/pkg/util"

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

	// UTXO stores unspent TX output for some transaction.
	UTXO struct {
		Value util.Fixed8
		TxID  util.Uint256
		N     uint16
	}

	// Unspents is a slice of UTXOs (TODO: drop it?).
	Unspents []UTXO

	// Unspent stores Unspents per asset
	Unspent struct {
		Unspent Unspents
		Asset   string      // "NEO" / "GAS"
		Amount  util.Fixed8 // total unspent of this asset
	}

	// NeoScanBalance is a struct of NeoScan response to 'get_balance' request
	NeoScanBalance struct {
		Balance []*Unspent
		Address string
	}
)

// functions for sorting array of `Unspents`
func (us Unspents) Len() int           { return len(us) }
func (us Unspents) Less(i, j int) bool { return us[i].Value < us[j].Value }
func (us Unspents) Swap(i, j int)      { us[i], us[j] = us[j], us[i] }
