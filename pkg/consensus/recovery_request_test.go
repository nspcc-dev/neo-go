package consensus

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecoveryRequest_Setters(t *testing.T) {
	var r recoveryRequest

	r.SetTimestamp(123 * nanoInSec)
	require.EqualValues(t, 123*nanoInSec, r.Timestamp())
}
