package transaction

import (
	"encoding/hex"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/internal/testserdes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInvocationZeroScript(t *testing.T) {
	// Zero-length script.
	in, err := hex.DecodeString("000000000000000000")
	require.NoError(t, err)

	inv := &InvocationTX{}
	assert.Error(t, testserdes.DecodeBinary(in, inv))

	// PUSH1 script.
	in, err = hex.DecodeString("01510000000000000000")
	require.NoError(t, err)

	assert.NoError(t, testserdes.DecodeBinary(in, inv))
}
