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
)

type (
	UTXO struct {
		Value util.Fixed8
		TxID  util.Uint256
		N     uint16
	}

	// unspent per asset
	Unspent struct {
		Unspent []UTXO
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
		return nil, err
	}
	unspents, err = getBalance(fromAddress, "")
	if err != nil {
		return nil, err
	}

	fromAddressHash, err = crypto.Uint160DecodeAddress(fromAddress)
	if err != nil {
		return nil, err
	}
	toAddressHash, err = crypto.Uint160DecodeAddress(address)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	witness.VerificationScript, err = wif.GetVerificationScript()
	if err != nil {
		return nil, err
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
		return nil, err
	}
	bytes := buf.Bytes()
	signature, err = wif.PrivateKey.Sign(bytes[:(len(bytes) - 1)])
	if err != nil {
		return nil, err
	}
	return append([]byte{pushbytes64}, signature...), nil
}

func sortUnspent(us Unspent) {
	sort.Slice(us.Unspent, func(i, j int) bool {
		return us.Unspent[i].Value > us.Unspent[j].Value
	})
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
	sortUnspent(assetUnspent)

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
