package native

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativehashes"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
)

func TestStd_HexEncodeDecodeCompat(t *testing.T) {
	bc, acc := chain.NewSingleWithCustomConfig(t, func(c *config.Blockchain) {
		c.Hardforks = map[string]uint32{
			config.HFFaun.String(): 2,
		}
	})
	e := neotest.NewExecutor(t, bc, acc, acc)
	p := e.CommitteeInvoker(nativehashes.StdLib)

	expectedBytes := []byte{0x00, 0x01, 0x02, 0x03}
	expectedString := "00010203"

	p.InvokeFail(t, "method not found: hexEncode/1", "hexEncode", expectedBytes)

	e.AddNewBlock(t)

	p.Invoke(t, expectedString, "hexEncode", expectedBytes)
	p.Invoke(t, expectedBytes, "hexDecode", expectedString)
}
