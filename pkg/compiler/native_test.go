package compiler_test

import (
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/gas"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/ledger"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/nameservice"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/neo"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/notary"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/oracle"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/policy"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/roles"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
)

func TestContractHashes(t *testing.T) {
	cs := native.NewContracts(true)
	require.Equal(t, []byte(neo.Hash), cs.NEO.Hash.BytesBE())
	require.Equal(t, []byte(gas.Hash), cs.GAS.Hash.BytesBE())
	require.Equal(t, []byte(oracle.Hash), cs.Oracle.Hash.BytesBE())
	require.Equal(t, []byte(roles.Hash), cs.Designate.Hash.BytesBE())
	require.Equal(t, []byte(policy.Hash), cs.Policy.Hash.BytesBE())
	require.Equal(t, []byte(nameservice.Hash), cs.NameService.Hash.BytesBE())
	require.Equal(t, []byte(ledger.Hash), cs.Ledger.Hash.BytesBE())
	require.Equal(t, []byte(management.Hash), cs.Management.Hash.BytesBE())
	require.Equal(t, []byte(notary.Hash), cs.Notary.Hash.BytesBE())
}

// testPrintHash is a helper for updating contract hashes.
func testPrintHash(u util.Uint160) {
	fmt.Print(`"`)
	for _, b := range u.BytesBE() {
		fmt.Printf("\\x%02x", b)
	}
	fmt.Println(`"`)
}

func TestRoleManagementRole(t *testing.T) {
	require.EqualValues(t, native.RoleOracle, roles.Oracle)
	require.EqualValues(t, native.RoleStateValidator, roles.StateValidator)
	require.EqualValues(t, native.RoleP2PNotary, roles.P2PNotary)
}

func TestNameServiceRecordType(t *testing.T) {
	require.EqualValues(t, native.RecordTypeA, nameservice.TypeA)
	require.EqualValues(t, native.RecordTypeCNAME, nameservice.TypeCNAME)
	require.EqualValues(t, native.RecordTypeTXT, nameservice.TypeTXT)
	require.EqualValues(t, native.RecordTypeAAAA, nameservice.TypeAAAA)
}
