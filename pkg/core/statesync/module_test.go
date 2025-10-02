package statesync

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/nspcc-dev/neo-go/internal/fakechain"
	"github.com/nspcc-dev/neo-go/pkg/core/dao"
	"github.com/nspcc-dev/neo-go/pkg/core/mpt"
	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestModule_PR2019_discussion_r689629704(t *testing.T) {
	expectedStorage := storage.NewMemCachedStore(storage.NewMemoryStore())
	tr := mpt.NewTrie(nil, mpt.ModeLatest, expectedStorage)
	require.NoError(t, tr.Put([]byte{0x03}, []byte("leaf1")))
	require.NoError(t, tr.Put([]byte{0x01, 0xab, 0x02}, []byte("leaf2")))
	require.NoError(t, tr.Put([]byte{0x01, 0xab, 0x04}, []byte("leaf3")))
	require.NoError(t, tr.Put([]byte{0x06, 0x01, 0xde, 0x02}, []byte("leaf2"))) // <-- the same `leaf2` and `leaf3` values are put in the storage,
	require.NoError(t, tr.Put([]byte{0x06, 0x01, 0xde, 0x04}, []byte("leaf3"))) // <-- but the path should differ.
	require.NoError(t, tr.Put([]byte{0x06, 0x03}, []byte("leaf4")))

	sr := tr.StateRoot()
	tr.Flush(0)

	// Keep MPT nodes in a map in order not to repeat them. We'll use `nodes` map to ask
	// state sync module to restore the nodes.
	var (
		nodes         = make(map[util.Uint256][]byte)
		expectedItems []storage.KeyValue
	)
	expectedStorage.Seek(storage.SeekRange{Prefix: []byte{byte(storage.DataMPT)}}, func(k, v []byte) bool {
		key := bytes.Clone(k)
		value := bytes.Clone(v)
		expectedItems = append(expectedItems, storage.KeyValue{
			Key:   key,
			Value: value,
		})
		hash, err := util.Uint256DecodeBytesBE(key[1:])
		require.NoError(t, err)
		nodeBytes := value[:len(value)-4]
		nodes[hash] = nodeBytes
		return true
	})

	actualStorage := storage.NewMemCachedStore(storage.NewMemoryStore())
	// These actions are done in module.Init(), but it's not the point of the test.
	// Here we want to test only MPT restoring process.
	stateSync := &Module{
		log:          zaptest.NewLogger(t),
		syncPoint:    1000500,
		syncStage:    headersSynced,
		syncInterval: 100500,
		dao:          dao.NewSimple(actualStorage),
		mptpool:      NewPool(),
		bc:           fakechain.NewFakeChain(),
	}
	stateSync.billet = mpt.NewBillet(sr, mpt.ModeLatest,
		TemporaryPrefix(stateSync.dao.Version.StoragePrefix), actualStorage)
	stateSync.mptpool.Add(sr, []byte{})

	// The test itself: we'll ask state sync module to restore each node exactly once.
	// After that storage content (including storage items and refcounts) must
	// match exactly the one got from real MPT trie. MPT pool must be empty.
	// State sync module must have mptSynced state in the end.
	// MPT Billet root must become a collapsed hashnode (it was checked manually).
	requested := make(map[util.Uint256]struct{})
	for {
		unknownHashes := stateSync.GetUnknownMPTNodesBatch(1) // restore nodes one-by-one
		if len(unknownHashes) == 0 {
			break
		}
		h := unknownHashes[0]
		node, ok := nodes[h]
		if !ok {
			if _, ok = requested[h]; ok {
				t.Fatal("node was requested twice")
			}
			t.Fatal("unknown node was requested")
		}
		require.NotPanics(t, func() {
			err := stateSync.AddMPTNodes([][]byte{node})
			require.NoError(t, err)
		}, fmt.Errorf("hash=%s, value=%s", h.StringBE(), string(node)))
		requested[h] = struct{}{}
		delete(nodes, h)
		if len(nodes) == 0 {
			break
		}
	}
	require.Equal(t, headersSynced|mptSynced, stateSync.syncStage, "all nodes were sent exactly ones, but MPT wasn't restored")
	require.Equal(t, 0, len(nodes), "not all nodes were requested by state sync module")
	require.Equal(t, 0, stateSync.mptpool.Count(), "MPT was restored, but MPT pool still contains items")

	// Compare resulting storage items and refcounts.
	var actualItems []storage.KeyValue
	expectedStorage.Seek(storage.SeekRange{Prefix: []byte{byte(storage.DataMPT)}}, func(k, v []byte) bool {
		key := bytes.Clone(k)
		value := bytes.Clone(v)
		actualItems = append(actualItems, storage.KeyValue{
			Key:   key,
			Value: value,
		})
		return true
	})
	require.ElementsMatch(t, expectedItems, actualItems)
}
