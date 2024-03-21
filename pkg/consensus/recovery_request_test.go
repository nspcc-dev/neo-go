package consensus

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRecoveryRequest_Getters(t *testing.T) {
	var r = &recoveryRequest{
		timestamp: 123,
	}

	require.EqualValues(t, 123*nsInMs, r.Timestamp())
}
