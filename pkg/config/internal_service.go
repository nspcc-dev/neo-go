package config

// InternalService stores configuration for internal services that don't have
// any network configuration, but use a wallet and can be enabled/disabled.
type InternalService struct {
	Enabled      bool   `yaml:"Enabled"`
	UnlockWallet Wallet `yaml:"UnlockWallet"`
}
