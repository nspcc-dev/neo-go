package rpc

/*
	Declaration of types, helper functions and variables
	required for raw transaction composing.
*/

import "github.com/CityOfZion/neo-go/pkg/util"

type (
	UTXO struct {
		Value util.Fixed8
		TxID  util.Uint256
		N     uint16
	}

	Unspents []UTXO

	// unspent per asset
	Unspent struct {
		Unspent Unspents
		Asset   string      // "NEO" / "GAS"
		Amount  util.Fixed8 // total unspent of this asset
	}

	// struct of NeoScan response to 'get_balance' request
	NeoScanBalance struct {
		Balance []*Unspent
		Address string
	}
)

var GlobalAssets = map[string]string{
	"c56f33fc6ecfcd0c225c4ab356fee59390af8560be0e930faebe74a6daff7c9b": "NEO",
	"602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7": "GAS",
}

func (us Unspents) Len() int           { return len(us) }
func (us Unspents) Less(i, j int) bool { return us[i].Value < us[j].Value }
func (us Unspents) Swap(i, j int)      { us[i], us[j] = us[j], us[i] }
