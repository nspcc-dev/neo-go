package state

import (
	"testing"

	json "github.com/nspcc-dev/go-ordered-json"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/stackitem"
	"github.com/stretchr/testify/require"
)

func TestContractInvocation_MarshalUnmarshalJSON(t *testing.T) {
	t.Run("truncated", func(t *testing.T) {
		ci := NewContractInvocation(util.Uint160{}, "fakeMethodCall", nil, 1)
		testserdes.MarshalUnmarshalJSON(t, ci, new(ContractInvocation))
	})
	t.Run("not truncated", func(t *testing.T) {
		si := stackitem.NewArray([]stackitem.Item{stackitem.NewBool(false)})
		argBytes, err := stackitem.NewSerializationContext().Serialize(si, false)
		require.NoError(t, err)

		ci := NewContractInvocation(util.Uint160{}, "fakeMethodCall", argBytes, 1)
		// Marshal and Unmarshal are asymmetric, test manually
		out, err := json.Marshal(&ci)
		require.NoError(t, err)
		var ci2 ContractInvocation
		err = json.Unmarshal(out, &ci2)
		require.NoError(t, err)
		require.Equal(t, ci.Hash, ci2.Hash)
		require.Equal(t, ci.Method, ci2.Method)
		require.Equal(t, ci.Truncated, ci2.Truncated)
		require.Equal(t, ci.ArgumentsCount, ci2.ArgumentsCount)
		require.Equal(t, si, ci2.Arguments)
	})
}

func TestContractInvocation_EncodeDecodeBinary(t *testing.T) {
	t.Run("truncated", func(t *testing.T) {
		ci := NewContractInvocation(util.Uint160{}, "fakeMethodCall", nil, 1)
		testserdes.EncodeDecodeBinary(t, ci, new(ContractInvocation))
	})
	t.Run("not truncated", func(t *testing.T) {
		si := stackitem.NewArray([]stackitem.Item{stackitem.NewBool(false)})
		argBytes, err := stackitem.NewSerializationContext().Serialize(si, false)
		require.NoError(t, err)

		ci := NewContractInvocation(util.Uint160{}, "fakeMethodCall", argBytes, 1)
		testserdes.EncodeDecodeBinary(t, ci, new(ContractInvocation))
	})
}
