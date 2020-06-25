package client

import (
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/rpc/request"
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
