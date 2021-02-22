package native

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest/standard"
	"github.com/stretchr/testify/require"
)

func TestNonfungibleNEP11(t *testing.T) {
	n := newNonFungible("NFToken", -100, "SYM", 1)
	require.NoError(t, standard.Check(&n.ContractMD.Manifest, manifest.NEP11StandardName))
}
