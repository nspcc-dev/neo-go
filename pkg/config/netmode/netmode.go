package netmode

import "strconv"

const (
	// MainNet contains magic code used in the Neo main official network.
	MainNet Magic = 0x334f454e // NEO3
	// TestNet contains magic code used in the Neo testing network.
	TestNet Magic = 0x3554334e // N3T5
	// PrivNet contains magic code usually used for Neo private networks.
	PrivNet Magic = 56753 // docker privnet
	// UnitTestNet is a stub magic code used for testing purposes.
	UnitTestNet Magic = 42
	//MainNetNeoFS contains magic code used in the NeoFS main network.
	MainNetNeoFS Magic = 0x572dfa5 // NeoFS mainnet
	//TestNetNeoFS contains magic code used in the NeoFS test network.
	TestNetNeoFS Magic = 0x2bdb2b5f // NeoFS testnet
)

// Magic describes the network the blockchain will operate on.
type Magic uint32

// String implements the stringer interface.
func (n Magic) String() string {
	switch n {
	case PrivNet:
		return "privnet"
	case TestNet:
		return "testnet"
	case MainNet:
		return "mainnet"
	case UnitTestNet:
		return "unit_testnet"
	case MainNetNeoFS:
		return "mainnet.neofs"
	case TestNetNeoFS:
		return "testnet.neofs"
	default:
		return "net 0x" + strconv.FormatUint(uint64(n), 16)
	}
}
