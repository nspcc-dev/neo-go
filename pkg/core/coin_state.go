package core

// CoinState represents the state of a coin.
type CoinState uint8

// Viable CoinState constants.
const (
	CoinStateConfirmed CoinState = 0
	CoinStateSpent     CoinState = 1 << 1
	CoinStateClaimed   CoinState = 1 << 2
	CoinStateFrozen    CoinState = 1 << 5
)
