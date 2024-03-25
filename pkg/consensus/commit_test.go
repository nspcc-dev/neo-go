package consensus

import (
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/stretchr/testify/require"
)

func TestCommit_Getters(t *testing.T) {
	var sign [signatureSize]byte
	random.Fill(sign[:])

	var c = &commit{
		signature: sign,
	}
	require.Equal(t, sign[:], c.Signature())
}
