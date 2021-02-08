package compiler_test

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/interop/native/nameservice"
	"github.com/stretchr/testify/require"
)

func TestNameServiceRecordType(t *testing.T) {
	require.EqualValues(t, native.RecordTypeA, nameservice.TypeA)
	require.EqualValues(t, native.RecordTypeCNAME, nameservice.TypeCNAME)
	require.EqualValues(t, native.RecordTypeTXT, nameservice.TypeTXT)
	require.EqualValues(t, native.RecordTypeAAAA, nameservice.TypeAAAA)
}
