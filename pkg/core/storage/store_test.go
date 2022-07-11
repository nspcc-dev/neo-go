package storage

import (
	"testing"

	"github.com/nspcc-dev/neo-go/pkg/core/storage/dboper"
	"github.com/stretchr/testify/require"
)

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
	o := []dboper.Operation{
		{State: "Added", Key: []byte{0x01}, Value: []byte{0x01}},
		{State: "Changed", Key: []byte{0x03}, Value: []byte{0x03}},
		{State: "Deleted", Key: []byte{0x06}},
	}
	require.Equal(t, o, BatchToOperations(b))
}
