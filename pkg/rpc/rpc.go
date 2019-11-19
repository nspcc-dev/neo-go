package rpc

import (
	"encoding/hex"

	"github.com/CityOfZion/neo-go/pkg/core/transaction"
	"github.com/CityOfZion/neo-go/pkg/io"
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
func (c *Client) sendRawTransaction(rawTX string) (*response, error) {
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
		buf      = io.NewBufBinWriter()
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
	rawTx.EncodeBinary(buf.BinWriter)
	if buf.Err != nil {
		return nil, errors.Wrap(buf.Err, "failed to encode raw transaction to binary for `sendtoaddress`")
	}
	rawTxStr = hex.EncodeToString(buf.Bytes())
	if resp, err = c.sendRawTransaction(rawTxStr); err != nil {
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
