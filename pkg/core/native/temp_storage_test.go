package native_test

import (
	"math"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/limits"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/neotest/chain"
	"github.com/nspcc-dev/neo-go/pkg/smartcontract/trigger"
	"github.com/nspcc-dev/neo-go/pkg/vm"
	"github.com/stretchr/testify/require"
)

// Compatibility tests are taken from https://github.com/neo-project/neo/pull/4767.
func TestTempStorage_CalculateStoragePrice_Compat(t *testing.T) {
	const (
		year                   = 365 * 24 * 60 * 60 * 1000 // milliseconds.
		permanentDatoshi int64 = 100_000                   // compatible with raw C# data.
	)
	s := native.NewTempStorage()
	bc, _, _ := chain.NewMulti(t)
	ic, err := bc.GetTestVM(trigger.Application, nil, &block.Block{Header: block.Header{Timestamp: 1}})
	require.NoError(t, err)

	require.Equal(t, permanentDatoshi*vm.ExecFeeFactorMultiplier, ic.BaseStorageFee()) // C# test setup compatibility check; don't change `permanent` if this check fails -- adjust test chain setup instead.
	minimal := permanentDatoshi / 10
	require.Positive(t, minimal)

	minKey := []byte{0x00}
	minValue := []byte{}

	for _, tc := range []struct {
		name            string
		k, v            []byte
		lifetime        int64
		expectedDatoshi int64
	}{
		{"TTL: 0, size: 1 byte", minKey, minValue, 0, minimal},
		{"TTL: max, size: 1 byte", minKey, minValue, math.MaxInt64, permanentDatoshi},
		{"TTL: 1/2 of year, size: 1 byte", minKey, minValue, year / 2, permanentDatoshi / 2},
		{"TTL: 10% of year, size: 1 byte", minKey, minValue, year / 10, permanentDatoshi / 10},
		{"TTL: <10% of year, size: 1 byte", minKey, minValue, year/10 - 1, permanentDatoshi / 10},
		{"TTL: 0, size: 5 bytes", []byte{0, 1}, []byte{2, 3, 4}, 0, 5 * minimal},
		{"TTL: max, size: 5 bytes", []byte{0, 1}, []byte{2, 3, 4}, math.MaxInt64, 5 * permanentDatoshi},
		{"TTL: 1/2 of year, size: 5 bytes", []byte{0, 1}, []byte{2, 3, 4}, year / 2, 5 * permanentDatoshi / 2},
		{"TTL: 10% of year, size: 5 bytes", []byte{0, 1}, []byte{2, 3, 4}, year / 10, 5 * permanentDatoshi / 10},
		{"TTL: <10% of year, size: 5 bytes", []byte{0, 1}, []byte{2, 3, 4}, year/10 - 1, 5 * permanentDatoshi / 10},
		{"TTL: 10% of year, size: 6 bytes", []byte{0, 1}, []byte{2, 3, 4, 5}, year / 10, 6 * permanentDatoshi / 10},
		{"TTL: 0, size: max", make([]byte, 1+4+limits.MaxStorageKeyLen), make([]byte, 8+limits.MaxStorageValueLen), 0, 656_120_000},
		{"TTL: 10% of year, size: max", make([]byte, 1+4+limits.MaxStorageKeyLen), make([]byte, 8+limits.MaxStorageValueLen), year / 10, 656_120_000},
		{"TTL: 1/2 of year, size: max", make([]byte, 1+4+limits.MaxStorageKeyLen), make([]byte, 8+limits.MaxStorageValueLen), year / 2, 3_280_600_000},
		{"TTL: year, size: max", make([]byte, 1+4+limits.MaxStorageKeyLen), make([]byte, 8+limits.MaxStorageValueLen), year, 6_561_200_000},
		{"TTL: >year, size: max", make([]byte, 1+4+limits.MaxStorageKeyLen), make([]byte, 8+limits.MaxStorageValueLen), year + 1, 6_561_200_000},
		{"TTL: max, size: max", make([]byte, 1+4+limits.MaxStorageKeyLen), make([]byte, 8+limits.MaxStorageValueLen), math.MaxInt64, 6_561_200_000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actual := s.CalculateStoragePrice(ic, tc.k, tc.v, tc.lifetime)
			require.Equal(t, tc.expectedDatoshi*vm.ExecFeeFactorMultiplier /* NeoGo scaling */, actual)
		})
	}
}
