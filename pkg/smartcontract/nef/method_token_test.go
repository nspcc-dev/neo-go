package nef

import (
	"strings"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/random"
	"github.com/nspcc-dev/neo-go/internal/testserdes"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/callflag"
	"github.com/stretchr/testify/require"
)

func TestMethodToken_Serializable(t *testing.T) {
	getToken := func() *MethodToken {
		return &MethodToken{
			Hash:       random.Uint160(),
			Method:     "MethodName",
			ParamCount: 2,
			HasReturn:  true,
			CallFlag:   callflag.ReadStates,
		}
	}
	t.Run("good", func(t *testing.T) {
		testserdes.EncodeDecodeBinary(t, getToken(), new(MethodToken))
	})
	t.Run("too long name", func(t *testing.T) {
		tok := getToken()
		tok.Method = strings.Repeat("s", maxMethodLength+1)
		data, err := testserdes.EncodeBinary(tok)
		require.NoError(t, err)
		require.Error(t, testserdes.DecodeBinary(data, new(MethodToken)))
	})
	t.Run("start with '_'", func(t *testing.T) {
		tok := getToken()
		tok.Method = "_method"
		data, err := testserdes.EncodeBinary(tok)
		require.NoError(t, err)
		err = testserdes.DecodeBinary(data, new(MethodToken))
		require.ErrorIs(t, err, errInvalidMethodName)
	})
	t.Run("invalid call flag", func(t *testing.T) {
		tok := getToken()
		tok.CallFlag = ^callflag.All
		data, err := testserdes.EncodeBinary(tok)
		require.NoError(t, err)
		err = testserdes.DecodeBinary(data, new(MethodToken))
		require.ErrorIs(t, err, errInvalidCallFlag)
	})
}
