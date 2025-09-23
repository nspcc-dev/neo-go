package native

import (
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/compiler"
	"github.com/nspcc-dev/neo-go/pkg/config"
	"github.com/nspcc-dev/neo-go/pkg/core/native/nativehashes"
	"github.com/nspcc-dev/neo-go/pkg/neotest"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
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

func TestStd_HexEncodeDecodeInteropAPI(t *testing.T) {
	bc, acc := chain.NewSingleWithCustomConfig(t, func(c *config.Blockchain) {
		c.Hardforks = map[string]uint32{
			config.HFFaun.String(): 0,
		}
	})
	e := neotest.NewExecutor(t, bc, acc, acc)

	src := `package teststd
		import (
			"github.com/nspcc-dev/neo-go/pkg/interop/native/std"
		)
		func Encode(args []any) string {
			return std.HexEncode(args[0].([]byte))
		}
		func Decode(args []any) []byte {
			return std.HexDecode(args[0].(string))
		}`

	ctr := neotest.CompileSource(t, e.Validator.ScriptHash(), strings.NewReader(src), &compiler.Options{
		Name: "teststd_contract",
	})
	e.DeployContract(t, ctr, nil)

	expectedBytes := []byte{0x00, 0x01, 0x02, 0x03}
	expectedString := "00010203"

	ctrInvoker := e.NewInvoker(ctr.Hash, e.Committee)

	ctrInvoker.Invoke(t, stackitem.Make(expectedString), "encode", []any{expectedBytes})
	ctrInvoker.Invoke(t, stackitem.Make(expectedBytes), "decode", []any{expectedString})
}
