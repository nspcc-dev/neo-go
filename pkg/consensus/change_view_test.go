package consensus

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChangeView_Setters(t *testing.T) {
	var c changeView

	c.SetTimestamp(123 * nanoInSec)
	require.EqualValues(t, 123*nanoInSec, c.Timestamp())

	c.SetNewViewNumber(2)
	require.EqualValues(t, 2, c.NewViewNumber())
}
