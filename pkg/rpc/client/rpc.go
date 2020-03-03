package client

import (
	"encoding/hex"

	"github.com/CityOfZion/neo-go/pkg/core"
	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/rpc/request"
	"github.com/CityOfZion/neo-go/pkg/rpc/response/result"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/CityOfZion/neo-go/pkg/wallet"
	"github.com/pkg/errors"
)

// getBlock returns a block by its hash or index/height. If verbose is true
// the response will contain a pretty Block object instead of the raw hex string.
// missing output wrapper at the moment, thus commented out
// func (c *Client) getBlock(indexOrHash interface{}, verbose bool) (*response, error) {
// 	var (
// 		params = request.NewRawParams(indexOrHash)
// 		resp   = &response{}
// 	)
// 	if verbose {
// 		params = request.NewRawParams(indexOrHash, 1)
// 	}
// 	if err := c.performRequest("getblock", params, resp); err != nil {
// 		return nil, err
// 	}
// 	return resp, nil
// }

// GetAccountState returns detailed information about a NEO account.
func (c *Client) GetAccountState(address string) (*result.AccountState, error) {
	var (
		params = request.NewRawParams(address)
		resp   = &result.AccountState{}
	)
	if err := c.performRequest("getaccountstate", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetClaimable returns tx outputs which can be claimed.
func (c *Client) GetClaimable(address string) (*result.ClaimableInfo, error) {
	params := request.NewRawParams(address)
	resp := new(result.ClaimableInfo)
	if err := c.performRequest("getclaimable", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetUnspents returns UTXOs for the given NEO account.
func (c *Client) GetUnspents(address string) (*result.Unspents, error) {
	var (
		params = request.NewRawParams(address)
		resp   = &result.Unspents{}
	)
	if err := c.performRequest("getunspents", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// InvokeScript returns the result of the given script after running it true the VM.
// NOTE: This is a test invoke and will not affect the blockchain.
func (c *Client) InvokeScript(script string) (*result.Invoke, error) {
	var (
		params = request.NewRawParams(script)
		resp   = &result.Invoke{}
	)
	if err := c.performRequest("invokescript", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// InvokeFunction returns the results after calling the smart contract scripthash
// with the given operation and parameters.
// NOTE: this is test invoke and will not affect the blockchain.
func (c *Client) InvokeFunction(script, operation string, params []smartcontract.Parameter) (*result.Invoke, error) {
	var (
		p    = request.NewRawParams(script, operation, params)
		resp = &result.Invoke{}
	)
	if err := c.performRequest("invokefunction", p, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Invoke returns the results after calling the smart contract scripthash
// with the given parameters.
func (c *Client) Invoke(script string, params []smartcontract.Parameter) (*result.Invoke, error) {
	var (
		p    = request.NewRawParams(script, params)
		resp = &result.Invoke{}
	)
	if err := c.performRequest("invoke", p, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// getRawTransaction queries a transaction by hash.
// missing output wrapper at the moment, thus commented out
// func (c *Client) getRawTransaction(hash string, verbose bool) (*response, error) {
// 	var (
// 		params = request.NewRawParams(hash, verbose)
// 		resp   = &response{}
// 	)
// 	if err := c.performRequest("getrawtransaction", params, resp); err != nil {
// 		return nil, err
// 	}
// 	return resp, nil
// }

// SendRawTransaction broadcasts a transaction over the NEO network.
// The given hex string needs to be signed with a keypair.
// When the result of the response object is true, the TX has successfully
// been broadcasted to the network.
func (c *Client) SendRawTransaction(rawTX *transaction.Transaction) error {
	var (
		params = request.NewRawParams(hex.EncodeToString(rawTX.Bytes()))
		resp   bool
	)
	if err := c.performRequest("sendrawtransaction", params, &resp); err != nil {
		return err
	}
	if !resp {
		return errors.New("sendrawtransaction returned false")
	}
	return nil
}

// TransferAsset sends an amount of specific asset to a given address.
// This call requires open wallet. (`wif` key in client struct.)
// If response.Result is `true` then transaction was formed correctly and was written in blockchain.
func (c *Client) TransferAsset(asset util.Uint256, address string, amount util.Fixed8) (util.Uint256, error) {
	var (
		err      error
		rawTx    *transaction.Transaction
		txParams = request.ContractTxParams{
			AssetID:  asset,
			Address:  address,
			Value:    amount,
			WIF:      c.WIF(),
			Balancer: c.Balancer(),
		}
		resp util.Uint256
	)

	if rawTx, err = request.CreateRawContractTransaction(txParams); err != nil {
		return resp, errors.Wrap(err, "failed to create raw transaction")
	}
	if err = c.SendRawTransaction(rawTx); err != nil {
		return resp, errors.Wrap(err, "failed to send raw transaction")
	}
	return rawTx.Hash(), nil
}

// SignAndPushInvocationTx signs and pushes given script as an invocation
// transaction  using given wif to sign it and spending the amount of gas
// specified. It returns a hash of the invocation transaction and an error.
func (c *Client) SignAndPushInvocationTx(script []byte, wif *keys.WIF, gas util.Fixed8) (util.Uint256, error) {
	var txHash util.Uint256
	var err error

	tx := transaction.NewInvocationTX(script, gas)

	fromAddress := wif.PrivateKey.Address()

	if gas > 0 {
		if err = request.AddInputsAndUnspentsToTx(tx, fromAddress, core.UtilityTokenID(), gas, c); err != nil {
			return txHash, errors.Wrap(err, "failed to add inputs and unspents to transaction")
		}
	}

	acc, err := wallet.NewAccountFromWIF(wif.S)
	if err != nil {
		return txHash, err
	} else if err = acc.SignTx(tx); err != nil {
		return txHash, errors.Wrap(err, "failed to sign tx")
	}
	txHash = tx.Hash()
	err = c.SendRawTransaction(tx)

	if err != nil {
		return txHash, errors.Wrap(err, "failed sendning tx")
	}
	return txHash, nil
}
