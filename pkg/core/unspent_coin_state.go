package core

import (
	"github.com/CityOfZion/neo-go/pkg/util"
)

// UnspentCoinStates is mapping of Uin256 and UnspentCoinState.
type UnspentCoinStates map[util.Uint256]*UnspentCoinState

// UnspentCoinState ..
type UnspentCoinState struct {
	states []CoinState
}

// NewUnspentCoinState returns ne UnspentCoinState object.
func NewUnspentCoinState(states []CoinState) *UnspentCoinState {
	return &UnspentCoinState{
		states: states,
	}
}
