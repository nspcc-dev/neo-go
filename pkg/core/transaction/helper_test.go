package transaction

import (
	"encoding/hex"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/netmode"
	"github.com/stretchr/testify/assert"
)

var (
	//TODO NEO3.0: Update binary
	// https://neotracker.io/tx/fe4b3af60677204c57e573a57bdc97bc5059b05ad85b1474f84431f88d910f64
	rawInvocationTX = "d101590400b33f7114839c33710da24cf8e7d536b8d244f3991cf565c8146063795d3b9b3cd55aef026eae992b91063db0db53c1087472616e7366657267c5cc1cb5392019e2cc4e6d6b5ea54c8d4b6d11acf166cb072961424c54f6000000000000000001206063795d3b9b3cd55aef026eae992b91063db0db0000014140c6a131c55ca38995402dff8e92ac55d89cbed4b98dfebbcb01acbc01bd78fa2ce2061be921b8999a9ab79c2958875bccfafe7ce1bbbaf1f56580815ea3a4feed232102d41ddce2c97be4c9aa571b8a32cbc305aa29afffbcae71b0ef568db0e93929aaac"
)

func decodeTransaction(rawTX string, t *testing.T) *Transaction {
	b, err1 := hex.DecodeString(rawTX)
	assert.Nil(t, err1)
	tx, err := NewTransactionFromBytes(netmode.UnitTestNet, b)
	assert.NoError(t, err)
	return tx
}
