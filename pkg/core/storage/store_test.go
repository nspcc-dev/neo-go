package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	prefixes = []KeyPrefix{
		DataExecutable,
		DataMPT,
		STStorage,
		IXHeaderHashList,
		SYSCurrentBlock,
		SYSCurrentHeader,
		SYSVersion,
	}

	expected = []uint8{
		0x01,
		0x03,
		0x70,
		0x80,
		0xc0,
		0xc1,
		0xf0,
	}
)

func TestAppendPrefix(t *testing.T) {
	for i := 0; i < len(expected); i++ {
		value := []byte{0x01, 0x02}
		prefix := AppendPrefix(prefixes[i], value)
		assert.Equal(t, KeyPrefix(expected[i]), KeyPrefix(prefix[0]))
	}
}

func TestAppendPrefixInt(t *testing.T) {
	for i := 0; i < len(expected); i++ {
		value := 2000
		prefix := AppendPrefixInt(prefixes[i], value)
		assert.Equal(t, KeyPrefix(expected[i]), KeyPrefix(prefix[0]))
	}
}

func TestBatchToOperations(t *testing.T) {
	b := &MemBatch{
		Put: []KeyValueExists{
			{KeyValue: KeyValue{Key: []byte{byte(STStorage), 0x01}, Value: []byte{0x01}}},
			{KeyValue: KeyValue{Key: []byte{byte(DataMPT), 0x02}, Value: []byte{0x02}}},
			{KeyValue: KeyValue{Key: []byte{byte(STStorage), 0x03}, Value: []byte{0x03}}, Exists: true},
		},
		Deleted: []KeyValueExists{
			{KeyValue: KeyValue{Key: []byte{byte(STStorage), 0x04}, Value: []byte{0x04}}},
			{KeyValue: KeyValue{Key: []byte{byte(DataMPT), 0x05}, Value: []byte{0x05}}},
			{KeyValue: KeyValue{Key: []byte{byte(STStorage), 0x06}, Value: []byte{0x06}}, Exists: true},
		},
	}
	o := []Operation{
		{State: "Added", Key: []byte{0x01}, Value: []byte{0x01}},
		{State: "Changed", Key: []byte{0x03}, Value: []byte{0x03}},
		{State: "Deleted", Key: []byte{0x06}},
	}
	require.Equal(t, o, BatchToOperations(b))
}
