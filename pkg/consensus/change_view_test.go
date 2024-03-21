package consensus

import (
	"testing"

	"github.com/nspcc-dev/dbft"
	"github.com/stretchr/testify/require"
)

func TestChangeView_Getters(t *testing.T) {
	var c = &changeView{
		newViewNumber: 2,
		reason:        dbft.CVTimeout,
	}

	require.EqualValues(t, 2, c.NewViewNumber())
	require.EqualValues(t, dbft.CVTimeout, c.Reason())
}
