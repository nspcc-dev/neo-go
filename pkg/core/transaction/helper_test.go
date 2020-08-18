package transaction

import (
	"encoding/hex"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/stretchr/testify/assert"
)

var (
	// tx from testnet 58ea0709dac398c451fd51fdf4466f5257c77927c7909834a0ef3b469cd1a2ce
	rawInvocationTX = "00be80024f7673890000000000261c130000000000e404210001f813c2cc8e18bbe4b3b87f8ef9105b50bb93918e01005d0300743ba40b0000000c14aa07cc3f2193a973904a09a6e60b87f1f96273970c14f813c2cc8e18bbe4b3b87f8ef9105b50bb93918e13c00c087472616e736665720c14bcaf41d684c7d4ad6ee0d99da9707b9d1f0c8e6641627d5b523801420c402360bbf64b9644c25f066dbd406454b07ab9f56e8e25d92d90c96c598f6c29d97eabdcf226f3575481662cfcdd064ee410978e5fae3f09a2f83129ba9cd82641290c2103caf763f91d3691cba5b5df3eb13e668fdace0295b37e2e259fd0fb152d354f900b4195440d78"
)

func decodeTransaction(rawTX string, t *testing.T) *Transaction {
	b, err1 := hex.DecodeString(rawTX)
	assert.Nil(t, err1)
	tx, err := NewTransactionFromBytes(netmode.TestNet, b)
	assert.NoError(t, err)
	return tx
}
