package wallet

import (
	"encoding/json"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestToken_MarshalJSON(t *testing.T) {
	// From the https://neo-python.readthedocs.io/en/latest/prompt.html#import-nep5-compliant-token
	h, err := util.Uint160DecodeStringLE("f8d448b227991cf07cb96a6f9c0322437f1599b9")
	require.NoError(t, err)

	tok := NewToken(h, "NEP17 Standard", "NEP17", 8, manifest.NEP17StandardName)
	require.Equal(t, "NEP17 Standard", tok.Name)
	require.Equal(t, "NEP17", tok.Symbol)
	require.EqualValues(t, 8, tok.Decimals)
	require.Equal(t, h, tok.Hash)
	require.Equal(t, "NcqKahsZ93ZyYS5bep8G2TY1zRB7tfUPdK", tok.Address())

	data, err := json.Marshal(tok)
	require.NoError(t, err)

	actual := new(Token)
	require.NoError(t, json.Unmarshal(data, actual))
	require.Equal(t, tok, actual)
}
