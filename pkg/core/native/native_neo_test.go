package native

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCandidate_Bytes(t *testing.T) {
	expected := &candidate{
		Registered: true,
		Votes:      *big.NewInt(0x0F),
	}
	data := expected.Bytes()
	actual := new(candidate).FromBytes(data)
	require.Equal(t, expected, actual)
}
