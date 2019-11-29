package rpc

import (
	"encoding/hex"
	"fmt"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/crypto/keys"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/pkg/errors"
)

// getBlock returns a block by its hash or index/height. If verbose is true
// the response will contain a pretty Block object instead of the raw hex string.
// missing output wrapper at the moment, thus commented out
// func (c *Client) getBlock(indexOrHash interface{}, verbose bool) (*response, error) {
// 	var (
// 		params = newParams(indexOrHash)
// 		resp   = &response{}
// 	)
// 	if verbose {
// 		params = newParams(indexOrHash, 1)
// 	}
// 	if err := c.performRequest("getblock", params, resp); err != nil {
// 		return nil, err
// 	}
// 	return resp, nil
// }

// GetAccountState returns detailed information about a NEO account.
func (c *Client) GetAccountState(address string) (*AccountStateResponse, error) {
	var (
		params = newParams(address)
		resp   = &AccountStateResponse{}
	)
	if err := c.performRequest("getaccountstate", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetUnspents returns UTXOs for the given NEO account.
func (c *Client) GetUnspents(address string) (*UnspentResponse, error) {
	var (
		params = newParams(address)
		resp   = &UnspentResponse{}
	)
	if err := c.performRequest("getunspents", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// InvokeScript returns the result of the given script after running it true the VM.
// NOTE: This is a test invoke and will not affect the blockchain.
func (c *Client) InvokeScript(script string) (*InvokeScriptResponse, error) {
	var (
		params = newParams(script)
		resp   = &InvokeScriptResponse{}
	)
	if err := c.performRequest("invokescript", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// InvokeFunction returns the results after calling the smart contract scripthash
// with the given operation and parameters.
// NOTE: this is test invoke and will not affect the blockchain.
func (c *Client) InvokeFunction(script, operation string, params []smartcontract.Parameter) (*InvokeScriptResponse, error) {
	var (
		p    = newParams(script, operation, params)
		resp = &InvokeScriptResponse{}
	)
	if err := c.performRequest("invokefunction", p, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// Invoke returns the results after calling the smart contract scripthash
// with the given parameters.
func (c *Client) Invoke(script string, params []smartcontract.Parameter) (*InvokeScriptResponse, error) {
	var (
		p    = newParams(script, params)
		resp = &InvokeScriptResponse{}
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
// 		params = newParams(hash, verbose)
// 		resp   = &response{}
// 	)
// 	if err := c.performRequest("getrawtransaction", params, resp); err != nil {
// 		return nil, err
// 	}
// 	return resp, nil
// }

// sendRawTransaction broadcasts a transaction over the NEO network.
// The given hex string needs to be signed with a keypair.
// When the result of the response object is true, the TX has successfully
// been broadcasted to the network.
func (c *Client) sendRawTransaction(rawTX *transaction.Transaction) (*response, error) {
	var (
		params = newParams(hex.EncodeToString(rawTX.Bytes()))
		resp   = &response{}
	)
	if err := c.performRequest("sendrawtransaction", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// SendToAddress sends an amount of specific asset to a given address.
// This call requires open wallet. (`wif` key in client struct.)
// If response.Result is `true` then transaction was formed correctly and was written in blockchain.
func (c *Client) SendToAddress(asset util.Uint256, address string, amount util.Fixed8) (*SendToAddressResponse, error) {
	var (
		err      error
		rawTx    *transaction.Transaction
		txParams = ContractTxParams{
			assetID:  asset,
			address:  address,
			value:    amount,
			wif:      c.WIF(),
			balancer: c.Balancer(),
		}
		resp     *response
		response = &SendToAddressResponse{}
	)

	if rawTx, err = CreateRawContractTransaction(txParams); err != nil {
		return nil, errors.Wrap(err, "failed to create raw transaction for `sendtoaddress`")
	}
	if resp, err = c.sendRawTransaction(rawTx); err != nil {
		return nil, errors.Wrap(err, "failed to send raw transaction")
	}
	response.Error = resp.Error
	response.ID = resp.ID
	response.JSONRPC = resp.JSONRPC
	response.Result = &TxResponse{
		TxID: rawTx.Hash().ReverseString(),
	}
	return response, nil
}

// SignAndPushInvocationTx signs and pushes given script as an invocation
// transaction  using given wif to sign it and spending the amount of gas
// specified. It returns a hash of the invocation transaction and an error.
func (c *Client) SignAndPushInvocationTx(script []byte, wif *keys.WIF, gas util.Fixed8) (util.Uint256, error) {
	var txHash util.Uint256
	var err error

	gasIDB, _ := hex.DecodeString("602c79718b16e442de58778e148d0b1084e3b2dffd5de6b7b16cee7969282de7")
	gasID, _ := util.Uint256DecodeReverseBytes(gasIDB)

	tx := transaction.NewInvocationTX(script, gas)

	fromAddress := wif.PrivateKey.Address()

	if err = AddInputsAndUnspentsToTx(tx, fromAddress, gasID, gas, c); err != nil {
		return txHash, errors.Wrap(err, "failed to add inputs and unspents to transaction")
	}

	if err = SignTx(tx, wif); err != nil {
		return txHash, errors.Wrap(err, "failed to sign tx")
	}
	txHash = tx.Hash()
	resp, err := c.sendRawTransaction(tx)

	if err != nil {
		return txHash, errors.Wrap(err, "failed sendning tx")
	}
	if resp.Error != nil {
		return txHash, fmt.Errorf("remote returned %d: %s", resp.Error.Code, resp.Error.Message)
	}
	return txHash, nil
}
