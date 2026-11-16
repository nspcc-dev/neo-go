package native_test

import (
	"encoding/binary"
	"math"
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/config/limits"
	"github.com/nspcc-dev/neo-go/pkg/core/block"
	"github.com/nspcc-dev/neo-go/pkg/core/native"
	"github.com/nspcc-dev/neo-go/pkg/core/state"
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
			actual := s.CalculateStoragePrice(ic, nil, tc.k, tc.v, 0, tc.lifetime)
			require.Equal(t, tc.expectedDatoshi*vm.ExecFeeFactorMultiplier /* NeoGo scaling */, actual)
		})
	}
}

// Test cases are taken from https://github.com/neo-project/neo/pull/4814.
func TestTempStorage_CalculateStoragePrice_EdgeCasesCompat(t *testing.T) {
	const (
		timestamp = 1_000_000
		ttl       = int64(365 * 24 * 60 * 60 * 1000) // max TTL, milliseconds.
	)
	s := native.NewTempStorage()
	bc, _, _ := chain.NewMulti(t)
	ic, err := bc.GetTestVM(trigger.Application, nil, &block.Block{Header: block.Header{Timestamp: timestamp}})
	require.NoError(t, err)
	price := ic.BaseStorageFee()
	require.Positive(t, price)
	key := []byte{0x00}
	// Existing record layout: 8 bytes of validTill + user value.
	makeRecord := func(validTill uint64, valueLen int) state.StorageItem {
		item := make(state.StorageItem, 8+valueLen)
		binary.BigEndian.PutUint64(item, validTill)
		return item
	}
	calc := func(record state.StorageItem, k, v []byte, oldLifetime, newLifetime int64) int64 {
		return s.CalculateStoragePrice(ic, record, k, v, oldLifetime, newLifetime)
	}
	record := makeRecord(timestamp+1, 16) // not expired, 16 bytes of value.
	value := make([]byte, 16)
	// Old part only (same value size): (16-1)/4+1 = 4 bytes charged for the whole TTL.
	require.Equal(t, 4*price, calc(record, key, value, ttl, 0))
	// New part only: key and value are charged as for a new record for the whole TTL.
	require.Equal(t, 17*price, calc(record, key, value, 0, ttl))
	// Both parts, half of TTL each.
	require.Equal(t, 4*price/2+17*price/2, calc(record, key, value, ttl/2, ttl/2))
	// Both parts, third of TTL each.
	require.Equal(t, 4*price/3+17*price/3, calc(record, key, value, ttl/3, ttl/3))
	// Minimum is the sum of the minimums of both parts.
	require.Equal(t, 4*price/10+17*price/10, calc(record, key, value, 1, 1))
	// Old part only with the minimal lifetime: 10% of the old price.
	require.Equal(t, 4*price/10, calc(record, key, value, 1, 0))
	// New part only with the minimal lifetime: 10% of the new price.
	require.Equal(t, 17*price/10, calc(record, key, value, 0, 1))
	// Both lifetimes are zero: the record is treated as newly-added and the minimal fee is charged.
	require.Equal(t, 17*price/10, calc(nil, key, value, 0, 0))
	// Both lifetimes are zero: the record is treated as existing and the minimal fee is charged.
	require.Equal(t, 4*price/10, calc(record, key, value, 0, 0))
	// Lifetimes exceeding the max TTL are capped for each part independently.
	require.Equal(t, 4*price, calc(record, key, value, ttl*2, 0))
	require.Equal(t, 17*price, calc(record, key, value, 0, ttl*2))
	require.Equal(t, 4*price+17*price, calc(record, key, value, math.MaxInt64, math.MaxInt64))
	// Value is bigger than the old one: old part is (16-1)/4+1 + (32-16) = 20 bytes, new part is 1+32 bytes.
	bigValue := make([]byte, 32)
	require.Equal(t, 20*price, calc(record, key, bigValue, ttl, 0))
	require.Equal(t, 33*price, calc(record, key, bigValue, 0, ttl))
	require.Equal(t, 53*price, calc(record, key, bigValue, ttl, ttl))
	// Value is smaller than the old one: old part is (3-1)/4+1 = 1 byte, new part is 1+3 bytes.
	smallValue := make([]byte, 3)
	require.Equal(t, 1*price, calc(record, key, smallValue, ttl, 0))
	require.Equal(t, 4*price, calc(record, key, smallValue, 0, ttl))
	// Value size is not a multiple of 4: (5-1)/4+1 = 2 bytes.
	require.Equal(t, 2*price, calc(record, key, make([]byte, 5), ttl, 0))
	// Empty value: the old part is free, the new part charges the key only.
	emptyValue := []byte{}
	require.Equal(t, int64(0), calc(record, key, emptyValue, ttl, 0))
	require.Equal(t, 1*price, calc(record, key, emptyValue, 0, ttl))
	require.Equal(t, 1*price, calc(record, key, emptyValue, ttl, ttl))
	require.Equal(t, int64(0), calc(record, key, emptyValue, 0, 0))
	require.Equal(t, 1*price/10, calc(nil, key, emptyValue, 0, 0))
	// Longer key is charged for the new part only.
	longKey := make([]byte, limits.MaxStorageKeyLen)
	require.Equal(t, 4*price, calc(record, longKey, value, ttl, 0))
	require.Equal(t, int64(len(longKey)+len(value))*price, calc(record, longKey, value, 0, ttl))
	// Existing record with the empty value: nothing is charged at the "old" rate, so the whole value is charged.
	emptyRecord := makeRecord(timestamp+1, 0)
	require.Equal(t, 16*price, calc(emptyRecord, key, value, ttl, 0))
	require.Equal(t, 17*price, calc(emptyRecord, key, value, 0, ttl))
	// The record is still traceable if validTill equals the current block timestamp.
	require.Equal(t, 4*price, calc(makeRecord(timestamp, 16), key, value, ttl, 0))
	// Expired record is treated as a missing one even for the "old" part.
	expired := makeRecord(timestamp-1, 16)
	require.Equal(t, 17*price, calc(expired, key, value, ttl, 0))
	require.Equal(t, 17*price, calc(expired, key, value, 0, ttl))
	require.Equal(t, 17*price/2, calc(expired, key, value, ttl/2, 0))
	// Missing record is treated as a new one for the "old" part as well.
	require.Equal(t, 17*price, calc(nil, key, value, ttl, 0))
	require.Equal(t, 17*price, calc(nil, key, value, 0, ttl))
	require.Equal(t, 2*17*price, calc(nil, key, value, ttl, ttl))
	// Long oldLifetime with zero extension: full old price should be charged.
	require.Equal(t, 4*price, calc(makeRecord(timestamp+uint64(ttl), len(value)), key, value, ttl, 0))
	// Long oldLifetime with short extension: full old price plus minimal new price should be charged.
	require.Equal(t, 4*price+17*price/10, calc(makeRecord(timestamp+uint64(ttl), len(value)), key, value, ttl, 1))
	// Renew: zero old period with zero extension.
	require.Equal(t, 4*price/10, calc(makeRecord(timestamp, len(value)), key, value, 0, 0))
}
