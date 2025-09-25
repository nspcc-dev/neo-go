package mpt

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/nspcc-dev/neo-go/pkg/core/storage"
	"github.com/nspcc-dev/neo-go/pkg/io"
	"github.com/nspcc-dev/neo-go/pkg/util"
)

// DummySTTempStoragePrefix is a dummy contract storage item prefix that may be
// passed to Billet constructor when Billet's restoring functionality is not
// used, i.e. for those situations when only traversal functionality is used.
// Note that using this prefix for MPT restoring is an error which will lead to
// a panic since Billet must have the ability to save contract storage items to
// the underlying DB during MPT restore process.
const DummySTTempStoragePrefix = 0x00

var (
	// ErrRestoreFailed is returned when replacing HashNode by its "unhashed"
	// candidate fails.
	ErrRestoreFailed = errors.New("failed to restore MPT node")
	errStop          = errors.New("stop condition is met")
)

// Billet is a part of an MPT trie with missing hash nodes that need to be restored.
// Billet is based on the following assumptions:
//  1. Refcount can only be incremented (we don't change the MPT structure during restore,
//     thus don't need to decrease refcount).
//  2. Each time a part of a Billet is completely restored, it is collapsed into
//     HashNode.
//  3. Any pair (node, path) must be restored only once. It's a duty of an MPT pool to manage
//     MPT paths in order to provide this assumption.
type Billet struct {
	TempStoragePrefix storage.KeyPrefix
	Store             *storage.MemCachedStore

	root Node
	mode TrieMode
}

// NewBillet returns a new billet for MPT trie restoring. It accepts a MemCachedStore
// to decouple storage errors from logic errors so that all storage errors are
// processed during `store.Persist()` at the caller. Another benefit is
// that every `Put` can be considered an atomic operation. Note that mode
// parameter must match precisely the Trie mode that is used in the underlying
// DB to store the MPT nodes. Using wrong mode will lead to improper MPT nodes
// decoding and even runtime panic.
func NewBillet(rootHash util.Uint256, mode TrieMode, prefix storage.KeyPrefix, store *storage.MemCachedStore) *Billet {
	return &Billet{
		TempStoragePrefix: prefix,
		Store:             store,
		root:              NewHashNode(rootHash),
		mode:              mode,
	}
}

// RestoreHashNode replaces HashNode located at the provided path by the specified Node
// and stores it. It also maintains the MPT as small as possible by collapsing those parts
// of the MPT that have been completely restored.
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

	// If it's a leaf, then put into temporary contract storage.
	if leaf, ok := node.(*LeafNode); ok {
		if b.TempStoragePrefix == DummySTTempStoragePrefix {
			panic("invalid storage prefix")
		}
		k := append([]byte{byte(b.TempStoragePrefix)}, fromNibbles(path)...)
		b.Store.Put(k, leaf.value)
	}
	return nil
}

// putIntoNode puts val with the provided path inside curr and returns an updated node.
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
	// Once Leaf node is restored, it will be collapsed into HashNode forever, so
	// there shouldn't be such situation when we try to restore a Leaf node.
	panic("bug: can't restore LeafNode")
}

func (b *Billet) putIntoBranch(curr *BranchNode, path []byte, val Node) (Node, error) {
	if len(path) == 0 && curr.Hash().Equals(val.Hash()) {
		// This node has already been restored, so it's an MPT pool duty to avoid
		// duplicating restore requests.
		panic("bug: can't perform restoring of BranchNode twice")
	}
	i, path := splitPath(path)
	r, err := b.putIntoNode(curr.Children[i], path, val)
	if err != nil {
		return nil, err
	}
	curr.Children[i] = r
	return b.tryCollapseBranch(curr), nil
}

func (b *Billet) putIntoExtension(curr *ExtensionNode, path []byte, val Node) (Node, error) {
	if len(path) == 0 {
		if curr.Hash() != val.Hash() {
			return nil, fmt.Errorf("%w: bad Extension node hash: expected %s, got %s", ErrRestoreFailed, curr.Hash().StringBE(), val.Hash().StringBE())
		}
		// This node has already been restored, so it's an MPT pool duty to avoid
		// duplicating restore requests.
		panic("bug: can't perform restoring of ExtensionNode twice")
	}
	if !bytes.HasPrefix(path, curr.key) {
		return nil, fmt.Errorf("%w: can't modify ExtensionNode during restore", ErrRestoreFailed)
	}

	r, err := b.putIntoNode(curr.next, path[len(curr.key):], val)
	if err != nil {
		return nil, err
	}
	curr.next = r
	return b.tryCollapseExtension(curr), nil
}

func (b *Billet) putIntoHash(curr *HashNode, path []byte, val Node) (Node, error) {
	// Once a part of the MPT Billet is completely restored, it will be collapsed forever, so
	// it's an MPT pool duty to avoid duplicating restore requests.
	if len(path) != 0 {
		return nil, fmt.Errorf("%w: node has already been collapsed", ErrRestoreFailed)
	}

	// `curr` hash node can be either of
	// 1) saved in the storage (i.g. if we've already restored a node with the same hash from the
	//    other part of the MPT), so just add it to the local in-memory MPT.
	// 2) missing from the storage. It's OK because we're syncing MPT state, and the purpose
	//    is to store missing hash nodes.
	// both cases are OK, but we still need to validate `val` against `curr`.
	if val.Hash() != curr.Hash() {
		return nil, fmt.Errorf("%w: can't restore HashNode: expected and actual hashes mismatch (%s vs %s)", ErrRestoreFailed, curr.Hash().StringBE(), val.Hash().StringBE())
	}

	if curr.Collapsed {
		// This node has already been restored and collapsed, so it's an MPT pool duty to avoid
		// duplicating restore requests.
		panic("bug: can't perform restoring of collapsed node")
	}

	// We also need to increment refcount in both cases. That's the only place where refcount
	// is changed during restore process. Also flush right now, because sync process can be
	// interrupted at any time.
	b.incrementRefAndStore(val.Hash(), val.Bytes())

	if val.Type() == LeafT {
		return b.tryCollapseLeaf(val.(*LeafNode)), nil
	}
	return val, nil
}

func (b *Billet) incrementRefAndStore(h util.Uint256, bs []byte) {
	key := makeStorageKey(h)
	if b.mode.RC() {
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
			data = append(bs, 1, 0, 0, 0, 0)
		}
		binary.LittleEndian.PutUint32(data[len(data)-4:], uint32(cnt))
		b.Store.Put(key, data)
	} else {
		b.Store.Put(key, bs)
	}
}

// Traverse traverses MPT nodes (pre-order) starting from the billet root down
// to its children calling `process` for each serialised node until true is
// returned from `process` function. It also replaces all HashNodes to their
// "unhashed" counterparts until the stop condition is satisfied.
func (b *Billet) Traverse(process func(pathToNode []byte, node Node, nodeBytes []byte) bool, ignoreStorageErr bool) error {
	r, err := b.traverse(b.root, []byte{}, []byte{}, process, ignoreStorageErr, false)
	if err != nil && !errors.Is(err, errStop) {
		return err
	}
	b.root = r
	return nil
}

func (b *Billet) traverse(curr Node, path, from []byte, process func(pathToNode []byte, node Node, nodeBytes []byte) bool, ignoreStorageErr bool, backwards bool) (Node, error) {
	if _, ok := curr.(EmptyNode); ok {
		// We're not interested in EmptyNodes, and they do not affect the
		// traversal process, thus remain them untouched.
		return curr, nil
	}
	if hn, ok := curr.(*HashNode); ok {
		r, err := b.GetFromStore(hn.Hash())
		if err != nil {
			if ignoreStorageErr && errors.Is(err, storage.ErrKeyNotFound) {
				return hn, nil
			}
			return nil, err
		}
		return b.traverse(r, path, from, process, ignoreStorageErr, backwards)
	}
	if len(from) == 0 {
		bytes := bytes.Clone(curr.Bytes())
		if process(fromNibbles(path), curr, bytes) {
			return curr, errStop
		}
	}
	switch n := curr.(type) {
	case *LeafNode:
		return b.tryCollapseLeaf(n), nil
	case *BranchNode:
		var (
			startIndex byte
			endIndex   byte = childrenCount
			cmp             = func(i int) bool {
				return i < int(endIndex)
			}
			step = 1
		)
		if backwards {
			startIndex, endIndex = lastChild, startIndex
			cmp = func(i int) bool {
				return i >= int(endIndex)
			}
			step = -1
		}
		if len(from) != 0 {
			endIndex = lastChild
			if backwards {
				endIndex = 0
			}
			startIndex, from = splitPath(from)
		}
		for i := int(startIndex); cmp(i); i += step {
			var newPath []byte
			if i == lastChild {
				newPath = path
			} else {
				newPath = append(path, byte(i))
			}
			if byte(i) != startIndex {
				from = []byte{}
			}
			r, err := b.traverse(n.Children[i], newPath, from, process, ignoreStorageErr, backwards)
			if err != nil {
				if !errors.Is(err, errStop) {
					return nil, err
				}
				n.Children[i] = r
				return b.tryCollapseBranch(n), err
			}
			n.Children[i] = r
		}
		return b.tryCollapseBranch(n), nil
	case *ExtensionNode:
		if len(from) != 0 && bytes.HasPrefix(from, n.key) {
			from = from[len(n.key):]
		} else if len(from) == 0 || bytes.Compare(n.key, from) > 0 {
			from = []byte{}
		} else {
			return b.tryCollapseExtension(n), nil
		}
		r, err := b.traverse(n.next, append(path, n.key...), from, process, ignoreStorageErr, backwards)
		if err != nil && !errors.Is(err, errStop) {
			return nil, err
		}
		n.next = r
		return b.tryCollapseExtension(n), err
	default:
		return nil, ErrNotFound
	}
}

func (b *Billet) tryCollapseLeaf(curr *LeafNode) Node {
	// Leaf can always be collapsed.
	res := NewHashNode(curr.Hash())
	res.Collapsed = true
	return res
}

func (b *Billet) tryCollapseExtension(curr *ExtensionNode) Node {
	if curr.next.Type() != HashT || !curr.next.(*HashNode).Collapsed {
		return curr
	}
	res := NewHashNode(curr.Hash())
	res.Collapsed = true
	return res
}

func (b *Billet) tryCollapseBranch(curr *BranchNode) Node {
	canCollapse := true
	for i := range childrenCount {
		if curr.Children[i].Type() == EmptyT {
			continue
		}
		if curr.Children[i].Type() == HashT && curr.Children[i].(*HashNode).Collapsed {
			continue
		}
		canCollapse = false
		break
	}
	if !canCollapse {
		return curr
	}
	res := NewHashNode(curr.Hash())
	res.Collapsed = true
	return res
}

// GetFromStore returns MPT node from the storage.
func (b *Billet) GetFromStore(h util.Uint256) (Node, error) {
	data, err := b.Store.Get(makeStorageKey(h))
	if err != nil {
		return nil, err
	}

	var n NodeObject
	r := io.NewBinReaderFromBuf(data)
	n.DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}

	if b.mode.RC() {
		data = data[:len(data)-5]
	}
	n.Node.(flushedNode).setCache(data, h)
	return n.Node, nil
}
