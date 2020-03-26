package transaction

import (
	"encoding/hex"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvocationZeroScript(t *testing.T) {
	// Zero-length script.
	in, err := hex.DecodeString("000000000000000000")
	require.NoError(t, err)

	inv := &InvocationTX{Version: 1}
	assert.Error(t, testserdes.DecodeBinary(in, inv))

	// PUSH1 script.
	in, err = hex.DecodeString("01510000000000000000")
	require.NoError(t, err)

	assert.NoError(t, testserdes.DecodeBinary(in, inv))
}

func TestInvocationNegativeGas(t *testing.T) {
	// Negative GAS
	in, err := hex.DecodeString("015100000000000000ff")
	require.NoError(t, err)

	inv := &InvocationTX{Version: 1}
	assert.Error(t, testserdes.DecodeBinary(in, inv))

	// Positive GAS.
	in, err = hex.DecodeString("01510100000000000000")
	require.NoError(t, err)

	assert.NoError(t, testserdes.DecodeBinary(in, inv))
	assert.Equal(t, util.Fixed8(1), inv.Gas)
}

func TestInvocationVersionZero(t *testing.T) {
	in, err := hex.DecodeString("0151")
	require.NoError(t, err)

	inv := &InvocationTX{Version: 1}
	assert.Error(t, testserdes.DecodeBinary(in, inv))

	inv = &InvocationTX{Version: 0}
	assert.NoError(t, testserdes.DecodeBinary(in, inv))
	assert.Equal(t, util.Fixed8(0), inv.Gas)
}
