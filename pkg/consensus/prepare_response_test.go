package consensus

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestPrepareResponse_Setters(t *testing.T) {
	var p = prepareResponse{
		preparationHash: util.Uint256{1, 2, 3},
	}

	require.Equal(t, util.Uint256{1, 2, 3}, p.PreparationHash())
}
