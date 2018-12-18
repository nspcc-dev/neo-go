package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/wallet"
	errs "github.com/pkg/errors"
)

func CreateRawContractTransaction(wif wallet.WIF, assetIdUint util.Uint256, address string, amount util.Fixed8) (*transaction.Transaction, error) {
	var (
		err                            error
		tx                             = transaction.NewContractTX()
		toAddressHash, fromAddressHash util.Uint160
		fromAddress                    string
		senderOutput, receiverOutput   *transaction.Output
		inputs                         []*transaction.Input
		spent                          util.Fixed8
		unspents                       []*Unspent
		witness                        transaction.Witness
		assetId                        = GlobalAssets[assetIdUint.String()]
	)

	fromAddress, err = wif.PrivateKey.Address()
	if err != nil {
		return nil, errs.Wrapf(err, "Failed to take address from WIF: %v", wif.S)
	}
	unspents, err = getBalance(fromAddress, "")
	if err != nil {
		return nil, errs.Wrap(err, "Failed to ge balance")
	}

	fromAddressHash, err = crypto.Uint160DecodeAddress(fromAddress)
	if err != nil {
		return nil, errs.Wrapf(err, "Failed to take script hash from address: %v", fromAddress)
	}
	toAddressHash, err = crypto.Uint160DecodeAddress(address)
	if err != nil {
		return nil, errs.Wrapf(err, "Failed to take script hash from address: %v", address)
	}
	tx.Attributes = append(tx.Attributes,
		&transaction.Attribute{
			Usage: transaction.Script,
			Data:  fromAddressHash.Bytes(),
		})

	inputs, spent = calculateInputs(assetId, unspents, amount)
	if inputs == nil {
		return nil, errors.New("Got zero inputs; check sender balance")
	}
	for _, input := range inputs {
		tx.AddInput(input)
	}

	senderOutput = transaction.NewOutput(assetIdUint, spent-amount, fromAddressHash)
	tx.AddOutput(senderOutput)
	receiverOutput = transaction.NewOutput(assetIdUint, amount, toAddressHash)
	tx.AddOutput(receiverOutput)

	witness.InvocationScript, err = getInvocationScript(tx, wif)
	if err != nil {
		return nil, errs.Wrap(err, "Failed to create invocation script")
	}
	witness.VerificationScript, err = wif.GetVerificationScript()
	if err != nil {
		return nil, errs.Wrap(err, "Failed to create verification script")
	}
	tx.Scripts = append(tx.Scripts, &witness)
	tx.Hash()

	return tx, nil
}

func getInvocationScript(tx *transaction.Transaction, wif wallet.WIF) ([]byte, error) {
	const (
		pushbytes64 = 0x40
	)
	var (
		err       error
		buf       = new(bytes.Buffer)
		signature []byte
	)
	if err = tx.EncodeBinary(buf); err != nil {
		return nil, errs.Wrap(err, "Failed to encode transaction to binary")
	}
	bytes := buf.Bytes()
	signature, err = wif.PrivateKey.Sign(bytes[:(len(bytes) - 1)])
	if err != nil {
		return nil, errs.Wrap(err, "Failed ti sign transaction with private key")
	}
	return append([]byte{pushbytes64}, signature...), nil
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

func calculateInputs(assetId string, us []*Unspent, cost util.Fixed8) ([]*transaction.Input, util.Fixed8) {
	var (
		num, i       = uint16(0), uint16(0)
		required     = cost
		selected     = util.Fixed8(0)
		assetUnspent Unspent
	)
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
		return nil, util.Fixed8(0)
	}

	inputs := make([]*transaction.Input, num)
	for i = 0; i < num; i++ {
		inputs[i] = &transaction.Input{
			PrevHash:  assetUnspent.Unspent[i].TxID,
			PrevIndex: assetUnspent.Unspent[i].N,
		}
	}

	return inputs, selected
}

func getBalance(address string, customBalanceURL string) ([]*Unspent, error) {
	const (
		neoScanURL  = "http://127.0.0.1:4000"
		balancePath = "/api/main_net/v1/get_balance/"
	)
	var (
		err        error
		req        *http.Request
		res        *http.Response
		balance    NeoScanBalance
		client     = http.Client{}
		balanceURL string
	)
	if customBalanceURL != "" {
		balanceURL = customBalanceURL
	} else {
		balanceURL = neoScanURL + balancePath
	}

	if req, err = http.NewRequest(http.MethodGet, balanceURL+address, nil); err != nil {
		return nil, err
	}

	if res, err = client.Do(req); err != nil {
		return nil, err
	}

	defer func() error {
		if err := res.Body.Close(); err != nil {
			return err
		}
		return nil
	}()

	if err = json.NewDecoder(res.Body).Decode(&balance); err != nil {
		return nil, err
	}

	return balance.Balance, nil
}
