package rpc

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
