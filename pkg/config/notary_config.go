package config

// P2PNotary stores configuration for Notary node service.
type P2PNotary struct {
	Enabled      bool   `yaml:"Enabled"`
	UnlockWallet Wallet `yaml:"UnlockWallet"`
}
