package state

// Coin represents the state of a coin.
type Coin uint8

// Viable Coin constants.
const (
	CoinConfirmed Coin = 0
	CoinSpent     Coin = 1 << 1
	CoinClaimed   Coin = 1 << 2
	CoinFrozen    Coin = 1 << 5
)
