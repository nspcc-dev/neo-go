package rpc

import (
	"bytes"
	"encoding/hex"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/smartcontract"
	"github.com/CityOfZion/neo-go/pkg/util"
	"github.com/pkg/errors"
)

// GetBlock returns a block by its hash or index/height. If verbose is true
// the response will contain a pretty Block object instead of the raw hex string.
func (c *Client) GetBlock(indexOrHash interface{}, verbose bool) (*response, error) {
	var (
		params = newParams(indexOrHash)
		resp   = &response{}
	)
	if verbose {
		params = newParams(indexOrHash, 1)
	}
	if err := c.performRequest("getblock", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// GetAccountState will return detailed information about a NEO account.
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

// InvokeFunction return the results after calling a the smart contract scripthash
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

// Invoke returns the results after calling a the smart contract scripthash
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

// GetRawTransaction queries a transaction by hash.
func (c *Client) GetRawTransaction(hash string, verbose bool) (*response, error) {
	var (
		params = newParams(hash, verbose)
		resp   = &response{}
	)
	if err := c.performRequest("getrawtransaction", params, resp); err != nil {
		return nil, err
	}
	return resp, nil
}

// SendRawTransaction broadcasts a transaction over the NEO network.
// The given hex string needs to be signed with a keypair.
// When the result of the response object is true, the TX has successfully
// been broadcasted to the network.
func (c *Client) SendRawTransaction(rawTX string) (*response, error) {
	var (
		params = newParams(rawTX)
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
		buf      = &bytes.Buffer{}
		rawTx    *transaction.Transaction
		rawTxStr string
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
	if err = rawTx.EncodeBinary(buf); err != nil {
		return nil, errors.Wrap(err, "failed to encode raw transaction to binary for `sendtoaddress`")
	}
	rawTxStr = hex.EncodeToString(buf.Bytes())
	if resp, err = c.SendRawTransaction(rawTxStr); err != nil {
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
