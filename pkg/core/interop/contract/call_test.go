package contract_test

import (
	"math/big"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/interop/contract"
	"github.com/nspcc-dev/neo-go/pkg/core/transaction"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func TestGetCallFlags(t *testing.T) {
	bc, _ := chain.NewSingle(t)
	ic := bc.GetTestVM(trigger.Application, &transaction.Transaction{}, &block.Block{})

	ic.VM.LoadScriptWithHash([]byte{byte(opcode.RET)}, util.Uint160{1, 2, 3}, callflag.All)
	require.NoError(t, contract.GetCallFlags(ic))
	require.Equal(t, int64(callflag.All), ic.VM.Estack().Pop().Value().(*big.Int).Int64())
}
