package mpt

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"go.uber.org/atomic"
)

// Trie is an MPT trie storing all key-value pairs.
type Trie struct {
	Store *storage.MemCachedStore

	root Node

	gcRunning  atomic.Bool
	gcMtx      sync.Mutex
	generation uint32
	gcClose    chan struct{}
	gcFinished chan struct{}
}

// GenerationSpan is an amount of blocks in a single generation.
// It must be >= 65536 so we can store it in 2 bytes.
const GenerationSpan = 200000

// ErrNotFound is returned when requested trie item is missing.
var ErrNotFound = errors.New("item not found")

// NewTrie returns new MPT trie. It accepts a MemCachedStore to decouple storage errors from logic errors
// so that all storage errors are processed during `store.Persist()` at the caller.
// This also has the benefit, that every `Put` can be considered an atomic operation.
func NewTrie(root Node, store *storage.MemCachedStore) *Trie {
	if root == nil {
		root = new(HashNode)
	}

	return &Trie{
		Store: store,
		root:  root,

		gcClose:    make(chan struct{}),
		gcFinished: make(chan struct{}),
	}
}

// Get returns value for the provided key in t.
func (t *Trie) Get(key []byte) ([]byte, error) {
	path := toNibbles(key)
	r, bs, err := t.getWithPath(t.root, path)
	if err != nil {
		return nil, err
	}
	t.root = r
	return bs, nil
}

// getWithPath returns value the provided path in a subtrie rooting in curr.
// It also returns a current node with all hash nodes along the path
// replaced to their "unhashed" counterparts.
func (t *Trie) getWithPath(curr Node, path []byte) (Node, []byte, error) {
	switch n := curr.(type) {
	case *LeafNode:
		if len(path) == 0 {
			return curr, copySlice(n.value), nil
		}
	case *BranchNode:
		i, path := splitPath(path)
		r, bs, err := t.getWithPath(n.Children[i], path)
		if err != nil {
			return nil, nil, err
		}
		n.Children[i] = r
		return n, bs, nil
	case *HashNode:
		if !n.IsEmpty() {
			if r, err := t.getFromStore(n.hash); err == nil {
				return t.getWithPath(r, path)
			}
		}
	case *ExtensionNode:
		if bytes.HasPrefix(path, n.key) {
			r, bs, err := t.getWithPath(n.next, path[len(n.key):])
			if err != nil {
				return nil, nil, err
			}
			n.next = r
			return curr, bs, err
		}
	default:
		panic("invalid MPT node type")
	}
	return curr, nil, ErrNotFound
}

// Put puts key-value pair in t.
func (t *Trie) Put(key, value []byte) error {
	if len(key) > MaxKeyLength {
		return errors.New("key is too big")
	} else if len(value) > MaxValueLength {
		return errors.New("value is too big")
	}
	if len(value) == 0 {
		return t.Delete(key)
	}
	path := toNibbles(key)
	n := NewLeafNode(value)
	r, err := t.putIntoNode(t.root, path, n)
	if err != nil {
		return err
	}
	t.root = r
	return nil
}

// putIntoLeaf puts val to trie if current node is a Leaf.
// It returns Node if curr needs to be replaced and error if any.
func (t *Trie) putIntoLeaf(curr *LeafNode, path []byte, val Node) (Node, error) {
	v := val.(*LeafNode)
	if len(path) == 0 {
		return v, nil
	}

	b := NewBranchNode()
	b.Children[path[0]] = newSubTrie(path[1:], v)
	b.Children[lastChild] = curr
	return b, nil
}

// putIntoBranch puts val to trie if current node is a Branch.
// It returns Node if curr needs to be replaced and error if any.
func (t *Trie) putIntoBranch(curr *BranchNode, path []byte, val Node) (Node, error) {
	i, path := splitPath(path)
	r, err := t.putIntoNode(curr.Children[i], path, val)
	if err != nil {
		return nil, err
	}
	curr.Children[i] = r
	curr.invalidateCache()
	return curr, nil
}

// putIntoExtension puts val to trie if current node is an Extension.
// It returns Node if curr needs to be replaced and error if any.
func (t *Trie) putIntoExtension(curr *ExtensionNode, path []byte, val Node) (Node, error) {
	if bytes.HasPrefix(path, curr.key) {
		r, err := t.putIntoNode(curr.next, path[len(curr.key):], val)
		if err != nil {
			return nil, err
		}
		curr.next = r
		curr.invalidateCache()
		return curr, nil
	}

	pref := lcp(curr.key, path)
	lp := len(pref)
	keyTail := curr.key[lp:]
	pathTail := path[lp:]

	s1 := newSubTrie(keyTail[1:], curr.next)
	b := NewBranchNode()
	b.Children[keyTail[0]] = s1

	i, pathTail := splitPath(pathTail)
	s2 := newSubTrie(pathTail, val)
	b.Children[i] = s2

	if lp > 0 {
		return NewExtensionNode(copySlice(pref), b), nil
	}
	return b, nil
}

// putIntoHash puts val to trie if current node is a HashNode.
// It returns Node if curr needs to be replaced and error if any.
func (t *Trie) putIntoHash(curr *HashNode, path []byte, val Node) (Node, error) {
	if curr.IsEmpty() {
		return newSubTrie(path, val), nil
	}

	result, err := t.getFromStore(curr.hash)
	if err != nil {
		return nil, err
	}
	return t.putIntoNode(result, path, val)
}

// newSubTrie create new trie containing node at provided path.
func newSubTrie(path []byte, val Node) Node {
	if len(path) == 0 {
		return val
	}
	return NewExtensionNode(path, val)
}

func (t *Trie) putIntoNode(curr Node, path []byte, val Node) (Node, error) {
	switch n := curr.(type) {
	case *LeafNode:
		return t.putIntoLeaf(n, path, val)
	case *BranchNode:
		return t.putIntoBranch(n, path, val)
	case *ExtensionNode:
		return t.putIntoExtension(n, path, val)
	case *HashNode:
		return t.putIntoHash(n, path, val)
	default:
		panic("invalid MPT node type")
	}
}

// Delete removes key from trie.
// It returns no error on missing key.
func (t *Trie) Delete(key []byte) error {
	path := toNibbles(key)
	r, err := t.deleteFromNode(t.root, path)
	if err != nil {
		return err
	}
	t.root = r
	return nil
}

func (t *Trie) deleteFromBranch(b *BranchNode, path []byte) (Node, error) {
	i, path := splitPath(path)
	r, err := t.deleteFromNode(b.Children[i], path)
	if err != nil {
		return nil, err
	}
	b.Children[i] = r
	b.invalidateCache()
	var count, index int
	for i := range b.Children {
		h, ok := b.Children[i].(*HashNode)
		if !ok || !h.IsEmpty() {
			index = i
			count++
		}
	}
	// count is >= 1 because branch node had at least 2 children before deletion.
	if count > 1 {
		return b, nil
	}
	c := b.Children[index]
	if index == lastChild {
		return c, nil
	}
	if h, ok := c.(*HashNode); ok {
		c, err = t.getFromStore(h.Hash())
		if err != nil {
			return nil, err
		}
	}
	if e, ok := c.(*ExtensionNode); ok {
		e.key = append([]byte{byte(index)}, e.key...)
		e.invalidateCache()
		return e, nil
	}

	return NewExtensionNode([]byte{byte(index)}, c), nil
}

func (t *Trie) deleteFromExtension(n *ExtensionNode, path []byte) (Node, error) {
	if !bytes.HasPrefix(path, n.key) {
		return nil, ErrNotFound
	}
	r, err := t.deleteFromNode(n.next, path[len(n.key):])
	if err != nil {
		return nil, err
	}
	switch nxt := r.(type) {
	case *ExtensionNode:
		n.key = append(n.key, nxt.key...)
		n.next = nxt.next
	case *HashNode:
		if nxt.IsEmpty() {
			return nxt, nil
		}
	default:
		n.next = r
	}
	n.invalidateCache()
	return n, nil
}

func (t *Trie) deleteFromNode(curr Node, path []byte) (Node, error) {
	switch n := curr.(type) {
	case *LeafNode:
		if len(path) == 0 {
			return new(HashNode), nil
		}
		return nil, ErrNotFound
	case *BranchNode:
		return t.deleteFromBranch(n, path)
	case *ExtensionNode:
		return t.deleteFromExtension(n, path)
	case *HashNode:
		if n.IsEmpty() {
			return nil, ErrNotFound
		}
		newNode, err := t.getFromStore(n.Hash())
		if err != nil {
			return nil, err
		}
		return t.deleteFromNode(newNode, path)
	default:
		panic("invalid MPT node type")
	}
}

// StateRoot returns root hash of t.
func (t *Trie) StateRoot() util.Uint256 {
	if hn, ok := t.root.(*HashNode); ok && hn.IsEmpty() {
		return util.Uint256{}
	}
	return t.root.Hash()
}

func makeStorageKey(mptKey []byte) []byte {
	return append([]byte{byte(storage.DataMPT)}, mptKey...)
}

// SetGeneration sets current generation of MPT,
// Generation is used for garbage collecting.
func (t *Trie) SetGeneration(gen uint32) {
	t.generation = gen
}

// updateGeneration updates generation for a single node and returns true
// if it was updated.
func updateGenerationSingle(h util.Uint256, gen uint32, st storage.Store) error {
	key := makeStorageKey(h.BytesBE())
	data, err := st.Get(key)
	if err != nil {
		return fmt.Errorf("%w: %s", err, h.BytesBE())
	}
	old := binary.LittleEndian.Uint32(data)
	if old <= gen-2 {
		binary.LittleEndian.PutUint32(data, gen)
		_ = st.Put(key, data)
	}
	return nil
}

func (t *Trie) updateGeneration(h util.Uint256, gen uint32, st storage.Store) error {
	select {
	case <-t.gcClose:
		return ErrInterrupted
	default:
	}

	node, err := getFromStore(h, st)
	if err != nil {
		return err
	}
	// Children of node retrieved from store are always hash nodes.
	switch n := node.(type) {
	case *BranchNode:
		for i := range n.Children {
			if !n.Children[i].(*HashNode).IsEmpty() {
				if err := t.updateGeneration(n.Children[i].Hash(), gen, st); err != nil {
					return err
				}
			}
		}
	case *ExtensionNode:
		if !n.next.(*HashNode).IsEmpty() {
			if err := t.updateGeneration(n.next.Hash(), gen, st); err != nil {
				return err
			}
		}
	case *HashNode:
		panic(fmt.Sprintf("hash node in store: %s", n.hash))
	}
	return updateGenerationSingle(h, gen, st)
}

const (
	gcLogBatchSize = 5
	gcBatchSize    = 1 << gcLogBatchSize
)

// ErrInterrupted is returned when GC is shutting down.
var ErrInterrupted = errors.New("GC is shutting down")

// ShutdownGC shutdowns GC if it is in process.
func (t *Trie) ShutdownGC() {
	if t.gcRunning.Load() {
		close(t.gcClose)
		<-t.gcFinished
	}
}

// PerformGC compacts storage by removing items which are no longer
// belong to trie.
func (t *Trie) PerformGC(root util.Uint256, st storage.Store) (int, error) {
	if t.gcRunning.Load() {
		return 0, errors.New("GC is already running")
	}
	if root.Equals(util.Uint256{}) {
		return 0, nil
	}
	t.gcRunning.Store(true)
	defer func() {
		t.gcRunning.Store(false)
		select {
		case <-t.gcClose:
			close(t.gcFinished)
		default:
		}
	}()
	gen := t.generation
	if err := t.updateGeneration(root, gen, st); err != nil {
		panic(err)
	}
	var n int
	for i := 0; i <= 0xFF; i += gcBatchSize {
		t.gcMtx.Lock()
		_, err := t.Store.Persist()
		if err != nil {
			t.gcMtx.Unlock()
			return 0, err
		}
		b := st.Batch()
		for j := 0; j < gcBatchSize; j++ {
			st.Seek([]byte{byte(storage.DataMPT), byte(i + j)}, func(k, v []byte) {
				if len(k) == 33 && binary.LittleEndian.Uint32(v) <= gen-2 {
					n++
					b.Delete(k)
				}
			})
		}
		if err := st.PutBatch(b); err != nil {
			t.gcMtx.Unlock()
			return n, err
		}
		t.gcMtx.Unlock()
	}
	return n, nil
}

// Flush puts every node in the trie except Hash ones to the storage.
// Because we care only about block-level changes, there is no need to put every
// new node to storage. Normally, flush should be called with every StateRoot persist, i.e.
// after every block.
func (t *Trie) Flush() {
	t.gcMtx.Lock()
	defer t.gcMtx.Unlock()
	t.flush(t.root)
}

func (t *Trie) flush(node Node) {
	if node.IsFlushed() {
		return
	}
	switch n := node.(type) {
	case *BranchNode:
		for i := range n.Children {
			t.flush(n.Children[i])
		}
	case *ExtensionNode:
		t.flush(n.next)
	case *HashNode:
		return
	}
	t.putToStore(node)
}

func (t *Trie) putToStore(n Node) {
	if n.Type() == HashT {
		panic("can't put hash node in trie")
	}
	no := NodeObject{Node: n, Generation: t.generation}
	w := io.NewBufBinWriter()
	no.EncodeBinary(w.BinWriter)
	_ = t.Store.Put(makeStorageKey(n.Hash().BytesBE()), w.Bytes())
	n.SetFlushed()
}

func (t *Trie) getFromStore(h util.Uint256) (Node, error) {
	return getFromStore(h, t.Store)
}

func getFromStore(h util.Uint256, st storage.Store) (Node, error) {
	data, err := st.Get(makeStorageKey(h.BytesBE()))
	if err != nil {
		return nil, err
	}

	var n NodeObject
	r := io.NewBinReaderFromBuf(data)
	n.DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}
	n.Node.(flushedNode).setCache(data[4:], h)
	return n.Node, nil
}

// Collapse compresses all nodes at depth n to the hash nodes.
// Note: this function does not perform any kind of storage flushing so
// `Flush()` should be called explicitly before invoking function.
func (t *Trie) Collapse(depth int) {
	if depth < 0 {
		panic("negative depth")
	}
	t.root = collapse(depth, t.root)
}

func collapse(depth int, node Node) Node {
	if _, ok := node.(*HashNode); ok {
		return node
	} else if depth == 0 {
		return NewHashNode(node.Hash())
	}

	switch n := node.(type) {
	case *BranchNode:
		for i := range n.Children {
			n.Children[i] = collapse(depth-1, n.Children[i])
		}
	case *ExtensionNode:
		n.next = collapse(depth-1, n.next)
	case *LeafNode:
	case *HashNode:
	default:
		panic("invalid MPT node type")
	}
	return node
}
