package consensus

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecoveryRequest_Setters(t *testing.T) {
	var r recoveryRequest

	r.SetTimestamp(123 * nsInMs)
	require.EqualValues(t, 123*nsInMs, r.Timestamp())
}
