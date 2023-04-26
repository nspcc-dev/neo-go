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
	mgmt.Policy = newPolicy()
	d := dao.NewSimple(storage.NewMemoryStore(), false, false)
	err := mgmt.Initialize(&interop.Context{DAO: d})
	require.NoError(t, err)
	require.NoError(t, mgmt.Policy.Initialize(&interop.Context{DAO: d}))
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

	refContract, err := GetContract(d, h)
	require.NoError(t, err)
	require.Equal(t, contract, refContract)

	refContract, err = GetContractByID(d, contract.ID)
	require.NoError(t, err)
	require.Equal(t, contract, refContract)

	upContract, err := mgmt.Update(d, h, ne, manif)
	refContract.UpdateCounter++
	require.NoError(t, err)
	require.Equal(t, refContract, upContract)

	err = mgmt.Destroy(d, h)
	require.NoError(t, err)
	_, err = GetContract(d, h)
	require.Error(t, err)
	_, err = GetContractByID(d, contract.ID)
	require.Error(t, err)
}

func TestManagement_Initialize(t *testing.T) {
	t.Run("good", func(t *testing.T) {
		d := dao.NewSimple(storage.NewMemoryStore(), false, false)
		mgmt := newManagement()
		require.NoError(t, mgmt.InitializeCache(0, d))
	})
	/* See #2801
	t.Run("invalid contract state", func(t *testing.T) {
		d := dao.NewSimple(storage.NewMemoryStore(), false, false)
		mgmt := newManagement()
		d.PutStorageItem(mgmt.ID, []byte{PrefixContract}, state.StorageItem{0xFF})
		require.Error(t, mgmt.InitializeCache(d))
	})
	*/
}

func TestManagement_GetNEP17Contracts(t *testing.T) {
	mgmt := newManagement()
	mgmt.Policy = newPolicy()
	d := dao.NewSimple(storage.NewMemoryStore(), false, false)
	err := mgmt.Initialize(&interop.Context{DAO: d})
	require.NoError(t, err)
	require.NoError(t, mgmt.Policy.Initialize(&interop.Context{DAO: d}))
	err = mgmt.InitializeCache(0, d)
	require.NoError(t, err)

	require.Empty(t, mgmt.GetNEP17Contracts(d))
	private := d.GetPrivate()

	// Deploy NEP-17 contract
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
	manif.SupportedStandards = []string{manifest.NEP17StandardName}
	c1, err := mgmt.Deploy(private, sender, ne, manif)
	require.NoError(t, err)

	// c1 contract hash should be returned, as private DAO already contains changed cache.
	require.Equal(t, []util.Uint160{c1.Hash}, mgmt.GetNEP17Contracts(private))

	// Lower DAO still shouldn't contain c1, as no Persist was called.
	require.Empty(t, mgmt.GetNEP17Contracts(d))

	// Call Persist, check c1 contract hash is returned
	_, err = private.Persist()
	require.NoError(t, err)
	require.Equal(t, []util.Uint160{c1.Hash}, mgmt.GetNEP17Contracts(d))

	// Update contract
	private = d.GetPrivate()
	manif.ABI.Methods = append(manif.ABI.Methods, manifest.Method{
		Name:       "dummy2",
		ReturnType: smartcontract.VoidType,
		Parameters: []manifest.Parameter{},
	})
	c1Updated, err := mgmt.Update(private, c1.Hash, ne, manif)
	require.NoError(t, err)
	require.Equal(t, c1.Hash, c1Updated.Hash)

	// No changes expected in lower store.
	require.Equal(t, []util.Uint160{c1.Hash}, mgmt.GetNEP17Contracts(d))
	c1Lower, err := GetContract(d, c1.Hash)
	require.NoError(t, err)
	require.Equal(t, 1, len(c1Lower.Manifest.ABI.Methods))
	require.Equal(t, []util.Uint160{c1Updated.Hash}, mgmt.GetNEP17Contracts(private))
	c1Upper, err := GetContract(private, c1Updated.Hash)
	require.NoError(t, err)
	require.Equal(t, 2, len(c1Upper.Manifest.ABI.Methods))

	// Call Persist, check c1Updated state is returned from lower.
	_, err = private.Persist()
	require.NoError(t, err)
	require.Equal(t, []util.Uint160{c1.Hash}, mgmt.GetNEP17Contracts(d))
	c1Lower, err = GetContract(d, c1.Hash)
	require.NoError(t, err)
	require.Equal(t, 2, len(c1Lower.Manifest.ABI.Methods))
}
