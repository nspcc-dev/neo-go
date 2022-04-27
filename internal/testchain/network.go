package testchain

import "github.com/nspcc-dev/neo-go/pkg/config/netmode"

// Network returns testchain network's magic number.
func Network() netmode.Magic {
	return netmode.UnitTestNet
}
