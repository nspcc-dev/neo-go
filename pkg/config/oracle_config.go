package config

import "time"

// OracleConfiguration is a config for the oracle module.
type OracleConfiguration struct {
	Enabled               bool               `yaml:"Enabled"`
	AllowPrivateHost      bool               `yaml:"AllowPrivateHost"`
	Nodes                 []string           `yaml:"Nodes"`
	NeoFS                 NeoFSConfiguration `yaml:"NeoFS"`
	MaxTaskTimeout        time.Duration      `yaml:"MaxTaskTimeout"`
	RefreshInterval       time.Duration      `yaml:"RefreshInterval"`
	MaxConcurrentRequests int                `yaml:"MaxConcurrentRequests"`
	RequestTimeout        time.Duration      `yaml:"RequestTimeout"`
	ResponseTimeout       time.Duration      `yaml:"ResponseTimeout"`
	UnlockWallet          Wallet             `yaml:"UnlockWallet"`
}

// NeoFSConfiguration is a config for the NeoFS service.
type NeoFSConfiguration struct {
	Nodes   []string `yaml:"Nodes"`
	Timeout int      `yaml:"Timeout"`
}
