package config

import "time"

// OracleConfiguration is a config for the oracle module.
type OracleConfiguration struct {
	Enabled          bool          `yaml:"Enabled"`
	AllowPrivateHost bool          `yaml:"AllowPrivateHost"`
	Nodes            []string      `yaml:"Nodes"`
	RequestTimeout   time.Duration `yaml:"RequestTimeout"`
	ResponseTimeout  time.Duration `yaml:"ResponseTimeout"`
	UnlockWallet     Wallet        `yaml:"UnlockWallet"`
}
