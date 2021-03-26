package native

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/interop"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/manifest"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/nef"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/vm/opcode"
	"github.com/stretchr/testify/require"
)

func TestDeployGetUpdateDestroyContract(t *testing.T) {
	mgmt := newManagement()
	d := dao.NewCached(dao.NewSimple(storage.NewMemoryStore(), false))
	mgmt.Initialize(&interop.Context{DAO: d})
	script := []byte{byte(opcode.RET)}
	sender := util.Uint160{1, 2, 3}
	ne, err := nef.NewFile(script)
	require.NoError(t, err)
	manif := manifest.NewManifest("Test")
	manif.ABI.Methods = append(manif.ABI.Methods, manifest.Method{
		Name:       "dummy",
		ReturnType: smartcontract.VoidType,
		Parameters: []manifest.Parameter{},
	})

	h := state.CreateContractHash(sender, ne.Checksum, manif.Name)

	contract, err := mgmt.Deploy(d, sender, ne, manif)
	require.NoError(t, err)
	require.Equal(t, int32(1), contract.ID)
	require.Equal(t, uint16(0), contract.UpdateCounter)
	require.Equal(t, h, contract.Hash)
	require.Equal(t, ne, &contract.NEF)
	require.Equal(t, *manif, contract.Manifest)

	// Double deploy.
	_, err = mgmt.Deploy(d, sender, ne, manif)
	require.Error(t, err)

	// Different sender.
	sender2 := util.Uint160{3, 2, 1}
	contract2, err := mgmt.Deploy(d, sender2, ne, manif)
	require.NoError(t, err)
	require.Equal(t, int32(2), contract2.ID)
	require.Equal(t, uint16(0), contract2.UpdateCounter)
	require.Equal(t, state.CreateContractHash(sender2, ne.Checksum, manif.Name), contract2.Hash)
	require.Equal(t, ne, &contract2.NEF)
	require.Equal(t, *manif, contract2.Manifest)

	refContract, err := mgmt.GetContract(d, h)
	require.NoError(t, err)
	require.Equal(t, contract, refContract)

	upContract, err := mgmt.Update(d, h, ne, manif)
	refContract.UpdateCounter++
	require.NoError(t, err)
	require.Equal(t, refContract, upContract)

	err = mgmt.Destroy(d, h)
	require.NoError(t, err)
	_, err = mgmt.GetContract(d, h)
	require.Error(t, err)
}

func TestManagement_Initialize(t *testing.T) {
	t.Run("good", func(t *testing.T) {
		d := dao.NewSimple(storage.NewMemoryStore(), false)
		mgmt := newManagement()
		require.NoError(t, mgmt.InitializeCache(d))
	})
	t.Run("invalid contract state", func(t *testing.T) {
		d := dao.NewSimple(storage.NewMemoryStore(), false)
		mgmt := newManagement()
		require.NoError(t, d.PutStorageItem(mgmt.ID, []byte{prefixContract}, state.StorageItem{0xFF}))
		require.Error(t, mgmt.InitializeCache(d))
	})
}
