package result

import "github.com/CityOfZion/neo-go/pkg/util"

// ClaimableInfo is a result of a getclaimable RPC call.
type ClaimableInfo struct {
	Spents    []Claimable `json:"claimable"`
	Address   string      `json:"address"`
	Unclaimed util.Fixed8 `json:"unclaimed"`
}

// Claimable represents spent outputs which can be claimed.
type Claimable struct {
	Tx          util.Uint256 `json:"txid"`
	N           int          `json:"n"`
	Value       util.Fixed8  `json:"value"`
	StartHeight uint32       `json:"start_height"`
	EndHeight   uint32       `json:"end_height"`
	Generated   util.Fixed8  `json:"generated"`
	SysFee      util.Fixed8  `json:"sys_fee"`
	Unclaimed   util.Fixed8  `json:"unclaimed"`
}
