package consensus

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/stretchr/testify/require"
)

func TestCommit_Setters(t *testing.T) {
	var sign [signatureSize]byte
	random.Fill(sign[:])

	var c commit
	c.SetSignature(sign[:])
	require.Equal(t, sign[:], c.Signature())
}
