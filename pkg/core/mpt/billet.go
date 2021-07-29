package mpt

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
	"github.com/nspcc-dev/neo-go/pkg/util/slice"
)

var (
	// ErrRestoreFailed is returned when replacing HashNode by its "unhashed"
	// candidate fails.
	ErrRestoreFailed = errors.New("failed to restore MPT node")
	errStop          = errors.New("stop condition is met")
)

// Billet is a part of MPT trie with missing hash nodes that need to be restored.
// Billet is based on the following assumptions:
// 1. Refcount can only be incremented (we don't change MPT structure during restore,
//    thus don't need to decrease refcount).
// 2. TODO: Each time the part of Billet is completely restored, it is collapsed into HashNode.
type Billet struct {
	Store *storage.MemCachedStore

	root            Node
	refcountEnabled bool
}

// NewBillet returns new billet for MPT trie restoring. It accepts a MemCachedStore
// to decouple storage errors from logic errors so that all storage errors are
// processed during `store.Persist()` at the caller. This also has the benefit,
// that every `Put` can be considered an atomic operation.
func NewBillet(rootHash util.Uint256, enableRefCount bool, store *storage.MemCachedStore) *Billet {
	return &Billet{
		Store:           store,
		root:            NewHashNode(rootHash),
		refcountEnabled: enableRefCount,
	}
}

// RestoreHashNode replaces HashNode located at the provided path by the specified Node
// and stores it.
// TODO: It also maintains MPT as small as possible by collapsing those parts
// of MPT that have been completely restored.
func (b *Billet) RestoreHashNode(path []byte, node Node) error {
	if _, ok := node.(*HashNode); ok {
		return fmt.Errorf("%w: unable to restore node into HashNode", ErrRestoreFailed)
	}
	if _, ok := node.(EmptyNode); ok {
		return fmt.Errorf("%w: unable to restore node into EmptyNode", ErrRestoreFailed)
	}
	r, err := b.putIntoNode(b.root, path, node)
	if err != nil {
		return err
	}
	b.root = r

	// If it's a leaf, then put into contract storage.
	if leaf, ok := node.(*LeafNode); ok {
		k := append([]byte{byte(storage.STStorage)}, fromNibbles(path)...)
		_ = b.Store.Put(k, leaf.value)
	}
	return nil
}

// putIntoNode puts val with provided path inside curr and returns updated node.
// Reference counters are updated for both curr and returned value.
func (b *Billet) putIntoNode(curr Node, path []byte, val Node) (Node, error) {
	switch n := curr.(type) {
	case *LeafNode:
		return b.putIntoLeaf(n, path, val)
	case *BranchNode:
		return b.putIntoBranch(n, path, val)
	case *ExtensionNode:
		return b.putIntoExtension(n, path, val)
	case *HashNode:
		return b.putIntoHash(n, path, val)
	case EmptyNode:
		return nil, fmt.Errorf("%w: can't modify EmptyNode during restore", ErrRestoreFailed)
	default:
		panic("invalid MPT node type")
	}
}

func (b *Billet) putIntoLeaf(curr *LeafNode, path []byte, val Node) (Node, error) {
	if len(path) != 0 {
		return nil, fmt.Errorf("%w: can't modify LeafNode during restore", ErrRestoreFailed)
	}
	if curr.Hash() != val.Hash() {
		return nil, fmt.Errorf("%w: bad Leaf node hash: expected %s, got %s", ErrRestoreFailed, curr.Hash().StringBE(), val.Hash().StringBE())
	}
	// this node has already been restored, no refcount changes required
	return curr, nil
}

func (b *Billet) putIntoBranch(curr *BranchNode, path []byte, val Node) (Node, error) {
	if len(path) == 0 && curr.Hash().Equals(val.Hash()) {
		// this node has already been restored, no refcount changes required
		return curr, nil
	}
	i, path := splitPath(path)
	r, err := b.putIntoNode(curr.Children[i], path, val)
	if err != nil {
		return nil, err
	}
	curr.Children[i] = r
	return curr, nil
}

func (b *Billet) putIntoExtension(curr *ExtensionNode, path []byte, val Node) (Node, error) {
	if len(path) == 0 {
		if curr.Hash() != val.Hash() {
			return nil, fmt.Errorf("%w: bad Extension node hash: expected %s, got %s", ErrRestoreFailed, curr.Hash().StringBE(), val.Hash().StringBE())
		}
		// this node has already been restored, no refcount changes required
		return curr, nil
	}
	if !bytes.HasPrefix(path, curr.key) {
		return nil, fmt.Errorf("%w: can't modify ExtensionNode during restore", ErrRestoreFailed)
	}

	r, err := b.putIntoNode(curr.next, path[len(curr.key):], val)
	if err != nil {
		return nil, err
	}
	curr.next = r
	return curr, nil
}

func (b *Billet) putIntoHash(curr *HashNode, path []byte, val Node) (Node, error) {
	// Once the part of MPT Billet is completely restored, it will be collapsed forever, so
	// it's an MPT pool duty to avoid duplicating restore requests.
	if len(path) != 0 {
		return nil, fmt.Errorf("%w: node has already been collapsed", ErrRestoreFailed)
	}

	// `curr` hash node can be either of
	// 1) saved in storage (i.g. if we've already restored node with the same hash from the
	//    other part of MPT), so just add it to local in-memory MPT.
	// 2) missing from the storage. It's OK because we're syncing MPT state, and the purpose
	//    is to store missing hash nodes.
	// both cases are OK, but we still need to validate `val` against `curr`.
	if val.Hash() != curr.Hash() {
		return nil, fmt.Errorf("%w: can't restore HashNode: expected and actual hashes mismatch (%s vs %s)", ErrRestoreFailed, curr.Hash().StringBE(), val.Hash().StringBE())
	}
	// We also need to increment refcount in both cases. That's the only place where refcount
	// is changed during restore process. Also flush right now, because sync process can be
	// interrupted at any time.
	b.incrementRefAndStore(val.Hash(), val.Bytes())
	return val, nil
}

func (b *Billet) incrementRefAndStore(h util.Uint256, bs []byte) {
	key := makeStorageKey(h.BytesBE())
	if b.refcountEnabled {
		var (
			err  error
			data []byte
			cnt  int32
		)
		// An item may already be in store.
		data, err = b.Store.Get(key)
		if err == nil {
			cnt = int32(binary.LittleEndian.Uint32(data[len(data)-4:]))
		}
		cnt++
		if len(data) == 0 {
			data = append(bs, 0, 0, 0, 0)
		}
		binary.LittleEndian.PutUint32(data[len(data)-4:], uint32(cnt))
		_ = b.Store.Put(key, data)
	} else {
		_ = b.Store.Put(key, bs)
	}
}

// Traverse traverses MPT nodes (pre-order) starting from the billet root down
// to its children calling `process` for each serialised node until true is
// returned from `process` function. It also replaces all HashNodes to their
// "unhashed" counterparts until the stop condition is satisfied.
func (b *Billet) Traverse(process func(node Node, nodeBytes []byte) bool, ignoreStorageErr bool) error {
	r, err := b.traverse(b.root, process, ignoreStorageErr)
	if err != nil && !errors.Is(err, errStop) {
		return err
	}
	b.root = r
	return nil
}

func (b *Billet) traverse(curr Node, process func(node Node, nodeBytes []byte) bool, ignoreStorageErr bool) (Node, error) {
	if _, ok := curr.(EmptyNode); ok {
		// We're not interested in EmptyNodes, and they do not affect the
		// traversal process, thus remain them untouched.
		return curr, nil
	}
	if hn, ok := curr.(*HashNode); ok {
		r, err := b.getFromStore(hn.Hash())
		if err != nil {
			if ignoreStorageErr && errors.Is(err, storage.ErrKeyNotFound) {
				return hn, nil
			}
			return nil, err
		}
		return b.traverse(r, process, ignoreStorageErr)
	}
	bytes := slice.Copy(curr.Bytes())
	if process(curr, bytes) {
		return curr, errStop
	}
	switch n := curr.(type) {
	case *LeafNode:
		return n, nil
	case *BranchNode:
		for i := range n.Children {
			r, err := b.traverse(n.Children[i], process, ignoreStorageErr)
			if err != nil {
				if !errors.Is(err, errStop) {
					return nil, err
				}
				n.Children[i] = r
				return n, err
			}
			n.Children[i] = r
		}
		return n, nil
	case *ExtensionNode:
		r, err := b.traverse(n.next, process, ignoreStorageErr)
		if err != nil && !errors.Is(err, errStop) {
			return nil, err
		}
		n.next = r
		return n, err
	default:
		return nil, ErrNotFound
	}
}

func (b *Billet) getFromStore(h util.Uint256) (Node, error) {
	data, err := b.Store.Get(makeStorageKey(h.BytesBE()))
	if err != nil {
		return nil, err
	}

	var n NodeObject
	r := io.NewBinReaderFromBuf(data)
	n.DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}

	if b.refcountEnabled {
		data = data[:len(data)-4]
	}
	n.Node.(flushedNode).setCache(data, h)
	return n.Node, nil
}
