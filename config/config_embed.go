// Package config contains embedded YAML configuration files for different network modes
// of the Neo N3 blockchain and for NeoFS mainnet and testnet networks.
package config

import (
	_ "embed"
)

// MainNet is the Neo N3 mainnet configuration.
//
//go:embed protocol.mainnet.yml
var MainNet []byte

// TestNet is the Neo N3 testnet configuration.
//
//go:embed protocol.testnet.yml
var TestNet []byte

// PrivNet is the private network configuration.
//
//go:embed protocol.privnet.yml
var PrivNet []byte

// MainNetNeoFS is the mainnet NeoFS configuration.
//
//go:embed protocol.mainnet.neofs.yml
var MainNetNeoFS []byte

// TestNetNeoFS is the testnet NeoFS configuration.
//
//go:embed protocol.testnet.neofs.yml
var TestNetNeoFS []byte
