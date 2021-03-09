package config

// StateRoot contains state root service configuration.
type StateRoot struct {
	Enabled      bool   `yaml:"Enabled"`
	UnlockWallet Wallet `yaml:"UnlockWallet"`
}
