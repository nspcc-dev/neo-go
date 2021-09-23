package mpt

import (
	"encoding/binary"
	"errors"
	"fmt"
	"runtime"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// flushNodeGC puts node to the storage and adds generation mark to it.
// It panics if GC is disabled.
func (t *Trie) flushNodeGC(h util.Uint256) {
	if !t.GCEnabled {
		panic("`flushNodeGC` is called, but GC is disabled")
	}

	key := makeStorageKey(h.BytesBE())
	node := t.refcount[h]
	data := node.bytes
	if node.refcount > 0 {
		data = append(data, 0, 0, 0, 0)
		binary.LittleEndian.PutUint32(data[len(data)-4:], t.Generation)
		_ = t.Store.Put(key, data)
	}
}

func init() {
	runtime.SetBlockProfileRate(1)
}

func (t *Trie) traverseAndUpdateGeneration(h util.Uint256, gen uint32, st storage.Store) error {
	select {
	case <-t.gcClosed:
		return ErrInterrupted
	default:
	}

	key := makeStorageKey(h.BytesBE())
	data, err := st.Get(key)
	if err != nil {
		return err
	}

	var n NodeObject
	r := io.NewBinReaderFromBuf(data)
	n.DecodeBinary(r)
	if r.Err != nil {
		return r.Err
	}

	// Children of node retrieved from store are always hash nodes.
	switch nd := n.Node.(type) {
	case *BranchNode:
		for i := range nd.Children {
			if !isEmpty(nd.Children[i]) {
				if err := t.traverseAndUpdateGeneration(nd.Children[i].Hash(), gen, st); err != nil {
					return err
				}
			}
		}
	case *ExtensionNode:
		if !isEmpty(nd.next) {
			if err := t.traverseAndUpdateGeneration(nd.next.Hash(), gen, st); err != nil {
				return err
			}
		}
	case EmptyNode:
		panic("empty node")
	case *HashNode:
		panic(fmt.Sprintf("hash node in store: %s", nd.Hash()))
	}

	old := binary.LittleEndian.Uint32(data[len(data)-4:])
	if old <= gen-2 {
		binary.LittleEndian.PutUint32(data[len(data)-4:], gen)
		_ = st.Put(key, data)
	}
	return nil
}

const (
	gcLogBatchSize = 8
	gcBatchSize    = 1 << gcLogBatchSize
)

// ErrInterrupted is returned when GC is shutting down.
var ErrInterrupted = errors.New("GC is shutting down")

// ShutdownGC shutdowns GC if it is in process.
func (t *Trie) ShutdownGC() {
	if t.gcRunning.Load() {
		close(t.gcClosed)
		<-t.gcFinished
	}
}

const maxDepth = 10

func (t *Trie) PrepareGC() []util.Uint256 {
	return t.traverse(maxDepth, nil, t.root)
}

func (t *Trie) traverse(d int, hs []util.Uint256, node Node) []util.Uint256 {
	if d == 0 {
		return hs
	}
	switch n := node.(type) {
	case *BranchNode:
		hs = append(hs, n.Hash())
		for _, c := range n.Children {
			hs = t.traverse(d-1, hs, c)
		}
	case *ExtensionNode:
		hs = append(hs, n.Hash())
		hs = t.traverse(d-1, hs, n.next)
	case *LeafNode:
		hs = append(hs, n.Hash())
	}
	return hs
}

// PerformGC compacts storage by removing items which are no longer
func (t *Trie) PerformGC(root util.Uint256, st storage.Store) (int, error) {
	if !t.GCEnabled {
		return 0, nil
	}
	if t.gcRunning.Load() {
		return 0, errors.New("GC is already running")
	}
	if root.Equals(util.Uint256{}) {
		return 0, nil
	}
	//
	//t.gcMtx.Lock()
	//defer t.gcMtx.Unlock()

	t.gcRunning.Store(true)
	defer func() {
		t.gcRunning.Store(false)
		select {
		case <-t.gcClosed:
			close(t.gcFinished)
		default:
		}
	}()

	if mc, ok := t.Store.(*storage.MemCachedStore); ok {
		_, err := mc.Persist()
		if err != nil {
			return 0, err
		}
	}

	gen := t.Generation
	if err := t.traverseAndUpdateGeneration(root, gen, st); err != nil {
		return 0, fmt.Errorf("can't update generation: %w", err)
	}

	var n int
loop:
	for i := 0; i <= 0xFF; i += gcBatchSize {
		b := st.Batch()
		for j := 0; j < gcBatchSize; j++ {
			select {
			case <-t.gcClosed:
				break loop
			default:
			}
			st.Seek([]byte{byte(storage.DataMPT), byte(i + j)}, func(k, v []byte) {
				if len(k) == 33 && binary.LittleEndian.Uint32(v[len(v)-4:]) <= gen-2 {
					n++
					b.Delete(k)
				}
			})
		}

		err := st.PutBatch(b)
		if err != nil {
			return n, err
		}
		fmt.Println("REMOVE BATCH", i, n)
	}
	return n, nil
}
