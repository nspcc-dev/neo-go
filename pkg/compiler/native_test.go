package compiler_test

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/nameservice"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/roles"
	"github.com/stretchr/testify/require"
)

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
