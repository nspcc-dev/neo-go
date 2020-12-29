package callflag

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCallFlag_Has(t *testing.T) {
	require.True(t, AllowCall.Has(AllowCall))
	require.True(t, (AllowCall | AllowNotify).Has(AllowCall))
	require.False(t, (AllowCall).Has(AllowCall|AllowNotify))
	require.True(t, All.Has(ReadOnly))
}
