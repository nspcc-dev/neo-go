package consensus

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCommit_Setters(t *testing.T) {
	var sign [signatureSize]byte
	fillRandom(t, sign[:])

	var c commit
	c.SetSignature(sign[:])
	require.Equal(t, sign[:], c.Signature())
}
