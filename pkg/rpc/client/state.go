package client

import (
	"encoding/hex"

	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
	"github.com/nspcc-dev/neo-go/pkg/rpc/response/result"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// GetStateRootByHeight returns state root for the given height.
func (c *Client) GetStateRootByHeight(h uint32) (*state.MPTRootState, error) {
	return c.getStateRoot(request.NewRawParams(h))
}

// GetStateRootByHash returns state root for the given block hash.
func (c *Client) GetStateRootByHash(h util.Uint256) (*state.MPTRootState, error) {
	return c.getStateRoot(request.NewRawParams(h))
}

func (c *Client) getStateRoot(p request.RawParams) (*state.MPTRootState, error) {
	var resp state.MPTRootState
	err := c.performRequest("getstateroot", p, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetStateHeight returns current block and state height.
func (c *Client) GetStateHeight() (*result.StateHeight, error) {
	resp := new(result.StateHeight)
	err := c.performRequest("getstateheight", request.NewRawParams(), resp)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// GetProof returns proof that key belongs to a contract sc state rooted at root.
func (c *Client) GetProof(root util.Uint256, sc util.Uint160, key []byte) (*result.GetProof, error) {
	var resp result.GetProof
	ps := request.NewRawParams(root, sc, hex.EncodeToString(key))
	err := c.performRequest("getproof", ps, &resp)
	if err != nil {
		return nil, err
	}
	return &resp, nil
}
