package rpc

import "github.com/CityOfZion/neo-go/pkg/smartcontract"

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

// InvokeScipt returns the result of the given script after running it true the VM.
// NOTE: This is a test invoke and will not affect the blokchain.
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
