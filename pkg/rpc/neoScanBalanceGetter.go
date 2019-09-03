package rpc

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/util"
	errs "github.com/pkg/errors"
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
func (s NeoScanServer) CalculateInputs(address string, assetIdUint util.Uint256, cost util.Fixed8) ([]transaction.Input, util.Fixed8, error) {
	var (
		err          error
		num, i       uint16
		required     = cost
		selected     = util.Fixed8(0)
		us           []*Unspent
		assetUnspent Unspent
		assetId      = GlobalAssets[assetIdUint.ReverseString()]
	)
	if us, err = s.GetBalance(address); err != nil {
		return nil, util.Fixed8(0), errs.Wrapf(err, "Cannot get balance for address %v", address)
	}
	filterSpecificAsset(assetId, us, &assetUnspent)
	sort.Sort(assetUnspent.Unspent)

	for _, us := range assetUnspent.Unspent {
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
			PrevHash:  assetUnspent.Unspent[i].TxID,
			PrevIndex: assetUnspent.Unspent[i].N,
		})
	}

	return inputs, selected, nil
}
