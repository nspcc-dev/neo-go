package client

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"github.com/CityOfZion/neo-go/pkg/core/state"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/rpc/response/result"
	"github.com/CityOfZion/neo-go/pkg/util"
	errs "github.com/pkg/errors"
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
		Unspent state.UnspentBalances
		Asset   string      // "NEO" / "GAS"
		Amount  util.Fixed8 // total unspent of this asset
	}

	// NeoScanBalance is a struct of NeoScan response to 'get_balance' request
	NeoScanBalance struct {
		Balance []*Unspent
		Address string
	}
)

// GetBalance performs a request to get balance for the address specified.
func (s NeoScanServer) GetBalance(address string) ([]*Unspent, error) {
	var (
		err        error
		req        *http.Request
		res        *http.Response
		balance    NeoScanBalance
		client     = http.Client{}
		balanceURL = s.URL + s.Path
	)

	if req, err = http.NewRequest(http.MethodGet, balanceURL+address, nil); err != nil {
		return nil, errs.Wrap(err, "Failed to compose HTTP request")
	}

	if res, err = client.Do(req); err != nil {
		return nil, errs.Wrap(err, "Failed to perform HTTP request")
	}

	defer res.Body.Close()

	if err = json.NewDecoder(res.Body).Decode(&balance); err != nil {
		return nil, errs.Wrap(err, "Failed to decode HTTP response")
	}
	return balance.Balance, nil
}

func filterSpecificAsset(asset string, balance []*Unspent, assetBalance *Unspent) {
	for _, us := range balance {
		if us.Asset == asset {
			assetBalance.Unspent = us.Unspent
			assetBalance.Asset = us.Asset
			assetBalance.Amount = us.Amount
			return
		}
	}
}

// CalculateInputs creates input transactions for the specified amount of given asset belonging to specified address.
func (s NeoScanServer) CalculateInputs(address string, assetIDUint util.Uint256, cost util.Fixed8) ([]transaction.Input, util.Fixed8, error) {
	var (
		err          error
		us           []*Unspent
		assetUnspent Unspent
		assetID      = result.GlobalAssets[assetIDUint.StringLE()]
	)
	if us, err = s.GetBalance(address); err != nil {
		return nil, util.Fixed8(0), errs.Wrapf(err, "Cannot get balance for address %v", address)
	}
	filterSpecificAsset(assetID, us, &assetUnspent)
	return unspentsToInputs(assetUnspent.Unspent, cost)
}

// unspentsToInputs uses UnspentBalances to create a slice of inputs for a new
// transcation containing the required amount of asset.
func unspentsToInputs(utxos state.UnspentBalances, required util.Fixed8) ([]transaction.Input, util.Fixed8, error) {
	var (
		num, i   uint16
		selected = util.Fixed8(0)
	)
	sort.Sort(utxos)

	for _, us := range utxos {
		if selected >= required {
			break
		}
		selected += us.Value
		num++
	}
	if selected < required {
		return nil, util.Fixed8(0), errors.New("cannot compose inputs for transaction; check sender balance")
	}

	inputs := make([]transaction.Input, 0, num)
	for i = 0; i < num; i++ {
		inputs = append(inputs, transaction.Input{
			PrevHash:  utxos[i].Tx,
			PrevIndex: utxos[i].Index,
		})
	}

	return inputs, selected, nil
}
