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

// Trie is an MPT trie storing all key-value pairs.
type Trie struct {
	Store *storage.MemCachedStore

	root            Node
	refcountEnabled bool
	refcount        map[util.Uint256]*cachedNode
}

type cachedNode struct {
	bytes    []byte
	initial  int32
	refcount int32
}

// ErrNotFound is returned when requested trie item is missing.
var ErrNotFound = errors.New("item not found")

// NewTrie returns new MPT trie. It accepts a MemCachedStore to decouple storage errors from logic errors
// so that all storage errors are processed during `store.Persist()` at the caller.
// This also has the benefit, that every `Put` can be considered an atomic operation.
func NewTrie(root Node, enableRefCount bool, store *storage.MemCachedStore) *Trie {
	if root == nil {
		root = EmptyNode{}
	}

	return &Trie{
		Store: store,
		root:  root,

		refcountEnabled: enableRefCount,
		refcount:        make(map[util.Uint256]*cachedNode),
	}
}

// Get returns value for the provided key in t.
func (t *Trie) Get(key []byte) ([]byte, error) {
	if len(key) > MaxKeyLength {
		return nil, errors.New("key is too big")
	}
	path := toNibbles(key)
	r, leaf, _, err := t.getWithPath(t.root, path, true)
	if err != nil {
		return nil, err
	}
	t.root = r
	return slice.Copy(leaf.(*LeafNode).value), nil
}

// getWithPath returns a current node with all hash nodes along the path replaced
// to their "unhashed" counterparts. It also returns node the provided path in a
// subtrie rooting in curr points to. In case of `strict` set to `false` the
// provided path can be incomplete, so it also returns full path that points to
// the node found at the specified incomplete path. In case of `strict` set to `true`
// the resulting path matches the provided one.
func (t *Trie) getWithPath(curr Node, path []byte, strict bool) (Node, Node, []byte, error) {
	switch n := curr.(type) {
	case *LeafNode:
		if len(path) == 0 {
			return curr, n, []byte{}, nil
		}
	case *BranchNode:
		i, path := splitPath(path)
		if i == lastChild && !strict {
			return curr, n, []byte{}, nil
		}
		r, res, prefix, err := t.getWithPath(n.Children[i], path, strict)
		if err != nil {
			return nil, nil, nil, err
		}
		n.Children[i] = r
		return n, res, append([]byte{i}, prefix...), nil
	case EmptyNode:
	case *HashNode:
		if r, err := t.getFromStore(n.hash); err == nil {
			return t.getWithPath(r, path, strict)
		}
	case *ExtensionNode:
		if len(path) == 0 && !strict {
			return curr, n.next, n.key, nil
		}
		if bytes.HasPrefix(path, n.key) {
			r, res, prefix, err := t.getWithPath(n.next, path[len(n.key):], strict)
			if err != nil {
				return nil, nil, nil, err
			}
			n.next = r
			return curr, res, append(n.key, prefix...), err
		}
		if !strict && bytes.HasPrefix(n.key, path) {
			// path is shorter than prefix, stop seeking
			return curr, n.next, n.key, nil
		}
	default:
		panic("invalid MPT node type")
	}
	return curr, nil, nil, ErrNotFound
}

// Put puts key-value pair in t.
func (t *Trie) Put(key, value []byte) error {
	if len(key) == 0 {
		return errors.New("key is empty")
	} else if len(key) > MaxKeyLength {
		return errors.New("key is too big")
	} else if len(value) > MaxValueLength {
		return errors.New("value is too big")
	} else if value == nil {
		// (t *Trie).Delete should be used to remove value
		return errors.New("value is nil")
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
		t.removeRef(curr.Hash(), curr.bytes)
		t.addRef(val.Hash(), val.Bytes())
		return v, nil
	}

	b := NewBranchNode()
	b.Children[path[0]] = t.newSubTrie(path[1:], v, true)
	b.Children[lastChild] = curr
	t.addRef(b.Hash(), b.bytes)
	return b, nil
}

// putIntoBranch puts val to trie if current node is a Branch.
// It returns Node if curr needs to be replaced and error if any.
func (t *Trie) putIntoBranch(curr *BranchNode, path []byte, val Node) (Node, error) {
	i, path := splitPath(path)
	t.removeRef(curr.Hash(), curr.bytes)
	r, err := t.putIntoNode(curr.Children[i], path, val)
	if err != nil {
		return nil, err
	}
	curr.Children[i] = r
	curr.invalidateCache()
	t.addRef(curr.Hash(), curr.bytes)
	return curr, nil
}

// putIntoExtension puts val to trie if current node is an Extension.
// It returns Node if curr needs to be replaced and error if any.
func (t *Trie) putIntoExtension(curr *ExtensionNode, path []byte, val Node) (Node, error) {
	t.removeRef(curr.Hash(), curr.bytes)
	if bytes.HasPrefix(path, curr.key) {
		r, err := t.putIntoNode(curr.next, path[len(curr.key):], val)
		if err != nil {
			return nil, err
		}
		curr.next = r
		curr.invalidateCache()
		t.addRef(curr.Hash(), curr.bytes)
		return curr, nil
	}

	pref := lcp(curr.key, path)
	lp := len(pref)
	keyTail := curr.key[lp:]
	pathTail := path[lp:]

	s1 := t.newSubTrie(keyTail[1:], curr.next, false)
	b := NewBranchNode()
	b.Children[keyTail[0]] = s1

	i, pathTail := splitPath(pathTail)
	s2 := t.newSubTrie(pathTail, val, true)
	b.Children[i] = s2

	t.addRef(b.Hash(), b.bytes)
	if lp > 0 {
		e := NewExtensionNode(slice.Copy(pref), b)
		t.addRef(e.Hash(), e.bytes)
		return e, nil
	}
	return b, nil
}

func (t *Trie) putIntoEmpty(path []byte, val Node) (Node, error) {
	return t.newSubTrie(path, val, true), nil
}

// putIntoHash puts val to trie if current node is a HashNode.
// It returns Node if curr needs to be replaced and error if any.
func (t *Trie) putIntoHash(curr *HashNode, path []byte, val Node) (Node, error) {
	result, err := t.getFromStore(curr.hash)
	if err != nil {
		return nil, err
	}
	return t.putIntoNode(result, path, val)
}

// newSubTrie create new trie containing node at provided path.
func (t *Trie) newSubTrie(path []byte, val Node, newVal bool) Node {
	if newVal {
		t.addRef(val.Hash(), val.Bytes())
	}
	if len(path) == 0 {
		return val
	}
	e := NewExtensionNode(path, val)
	t.addRef(e.Hash(), e.bytes)
	return e
}

// putIntoNode puts val with provided path inside curr and returns updated node.
// Reference counters are updated for both curr and returned value.
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
	case EmptyNode:
		return t.putIntoEmpty(path, val)
	default:
		panic("invalid MPT node type")
	}
}

// Delete removes key from trie.
// It returns no error on missing key.
func (t *Trie) Delete(key []byte) error {
	if len(key) > MaxKeyLength {
		return errors.New("key is too big")
	}
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
	h := b.Hash()
	bs := b.bytes
	r, err := t.deleteFromNode(b.Children[i], path)
	if err != nil {
		return nil, err
	}
	t.removeRef(h, bs)
	b.Children[i] = r
	b.invalidateCache()
	var count, index int
	for i := range b.Children {
		if !isEmpty(b.Children[i]) {
			index = i
			count++
		}
	}
	// count is >= 1 because branch node had at least 2 children before deletion.
	if count > 1 {
		t.addRef(b.Hash(), b.bytes)
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
		t.removeRef(e.Hash(), e.bytes)
		e.key = append([]byte{byte(index)}, e.key...)
		e.invalidateCache()
		t.addRef(e.Hash(), e.bytes)
		return e, nil
	}

	e := NewExtensionNode([]byte{byte(index)}, c)
	t.addRef(e.Hash(), e.bytes)
	return e, nil
}

func (t *Trie) deleteFromExtension(n *ExtensionNode, path []byte) (Node, error) {
	if !bytes.HasPrefix(path, n.key) {
		return n, nil
	}
	h := n.Hash()
	bs := n.bytes
	r, err := t.deleteFromNode(n.next, path[len(n.key):])
	if err != nil {
		return nil, err
	}
	t.removeRef(h, bs)
	switch nxt := r.(type) {
	case *ExtensionNode:
		t.removeRef(nxt.Hash(), nxt.bytes)
		n.key = append(n.key, nxt.key...)
		n.next = nxt.next
	case EmptyNode:
		return nxt, nil
	case *HashNode:
		n.next = nxt
	default:
		n.next = r
	}
	n.invalidateCache()
	t.addRef(n.Hash(), n.bytes)
	return n, nil
}

// deleteFromNode removes value with provided path from curr and returns an updated node.
// Reference counters are updated for both curr and returned value.
func (t *Trie) deleteFromNode(curr Node, path []byte) (Node, error) {
	switch n := curr.(type) {
	case *LeafNode:
		if len(path) == 0 {
			t.removeRef(curr.Hash(), curr.Bytes())
			return EmptyNode{}, nil
		}
		return curr, nil
	case *BranchNode:
		return t.deleteFromBranch(n, path)
	case *ExtensionNode:
		return t.deleteFromExtension(n, path)
	case EmptyNode:
		return n, nil
	case *HashNode:
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
	if isEmpty(t.root) {
		return util.Uint256{}
	}
	return t.root.Hash()
}

func makeStorageKey(mptKey []byte) []byte {
	return append([]byte{byte(storage.DataMPT)}, mptKey...)
}

// Flush puts every node in the trie except Hash ones to the storage.
// Because we care only about block-level changes, there is no need to put every
// new node to storage. Normally, flush should be called with every StateRoot persist, i.e.
// after every block.
func (t *Trie) Flush() {
	for h, node := range t.refcount {
		if node.refcount != 0 {
			if node.bytes == nil {
				panic("item not in trie")
			}
			if t.refcountEnabled {
				node.initial = t.updateRefCount(h)
				if node.initial == 0 {
					delete(t.refcount, h)
				}
			} else if node.refcount > 0 {
				_ = t.Store.Put(makeStorageKey(h.BytesBE()), node.bytes)
			}
			node.refcount = 0
		} else {
			delete(t.refcount, h)
		}
	}
}

// updateRefCount should be called only when refcounting is enabled.
func (t *Trie) updateRefCount(h util.Uint256) int32 {
	if !t.refcountEnabled {
		panic("`updateRefCount` is called, but GC is disabled")
	}
	var data []byte
	key := makeStorageKey(h.BytesBE())
	node := t.refcount[h]
	cnt := node.initial
	if cnt == 0 {
		// A newly created item which may be in store.
		var err error
		data, err = t.Store.Get(key)
		if err == nil {
			cnt = int32(binary.LittleEndian.Uint32(data[len(data)-4:]))
		}
	}
	if len(data) == 0 {
		data = append(node.bytes, 0, 0, 0, 0)
	}
	cnt += node.refcount
	switch {
	case cnt < 0:
		// BUG: negative reference count
		panic(fmt.Sprintf("negative reference count: %s new %d, upd %d", h.StringBE(), cnt, t.refcount[h]))
	case cnt == 0:
		_ = t.Store.Delete(key)
	default:
		binary.LittleEndian.PutUint32(data[len(data)-4:], uint32(cnt))
		_ = t.Store.Put(key, data)
	}
	return cnt
}

func (t *Trie) addRef(h util.Uint256, bs []byte) {
	node := t.refcount[h]
	if node == nil {
		t.refcount[h] = &cachedNode{
			refcount: 1,
			bytes:    bs,
		}
		return
	}
	node.refcount++
	if node.bytes == nil {
		node.bytes = bs
	}
}

func (t *Trie) removeRef(h util.Uint256, bs []byte) {
	node := t.refcount[h]
	if node == nil {
		t.refcount[h] = &cachedNode{
			refcount: -1,
			bytes:    bs,
		}
		return
	}
	node.refcount--
	if node.bytes == nil {
		node.bytes = bs
	}
}

func (t *Trie) getFromStore(h util.Uint256) (Node, error) {
	data, err := t.Store.Get(makeStorageKey(h.BytesBE()))
	if err != nil {
		return nil, err
	}

	var n NodeObject
	r := io.NewBinReaderFromBuf(data)
	n.DecodeBinary(r)
	if r.Err != nil {
		return nil, r.Err
	}

	if t.refcountEnabled {
		data = data[:len(data)-4]
		node := t.refcount[h]
		if node != nil {
			node.bytes = data
			node.initial = int32(r.ReadU32LE())
		}
	}
	n.Node.(flushedNode).setCache(data, h)
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
	t.refcount = make(map[util.Uint256]*cachedNode)
}

func collapse(depth int, node Node) Node {
	switch node.(type) {
	case *HashNode, EmptyNode:
		return node
	}
	if depth == 0 {
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

// Find returns list of storage key-value pairs whose key is prefixed by the specified
// prefix starting from the specified `prefix`+`from` path (not including the item at
// the specified `prefix`+`from` path if so). The `max` number of elements is returned at max.
func (t *Trie) Find(prefix, from []byte, max int) ([]storage.KeyValue, error) {
	if len(prefix) > MaxKeyLength {
		return nil, errors.New("invalid prefix length")
	}
	if len(from) > MaxKeyLength-len(prefix) {
		return nil, errors.New("invalid from length")
	}
	prefixP := toNibbles(prefix)
	fromP := []byte{}
	if len(from) > 0 {
		fromP = toNibbles(from)
	}
	_, start, path, err := t.getWithPath(t.root, prefixP, false)
	if err != nil {
		return nil, fmt.Errorf("failed to determine the start node: %w", err)
	}
	path = path[len(prefixP):]

	if len(fromP) > 0 {
		if len(path) <= len(fromP) && bytes.HasPrefix(fromP, path) {
			fromP = fromP[len(path):]
		} else if len(path) > len(fromP) && bytes.HasPrefix(path, fromP) {
			fromP = []byte{}
		} else {
			cmp := bytes.Compare(path, fromP)
			switch {
			case cmp < 0:
				return []storage.KeyValue{}, nil
			case cmp > 0:
				fromP = []byte{}
			}
		}
	}

	var (
		res   []storage.KeyValue
		count int
	)
	b := NewBillet(t.root.Hash(), false, 0, t.Store)
	process := func(pathToNode []byte, node Node, _ []byte) bool {
		if leaf, ok := node.(*LeafNode); ok {
			if from == nil || !bytes.Equal(pathToNode, from) { // (*Billet).traverse includes `from` path into result if so. Need to filter out manually.
				res = append(res, storage.KeyValue{
					Key:   append(slice.Copy(prefix), pathToNode...),
					Value: slice.Copy(leaf.value),
				})
				count++
			}
		}
		return count >= max
	}
	_, err = b.traverse(start, path, fromP, process, false)
	if err != nil && !errors.Is(err, errStop) {
		return nil, err
	}
	return res, nil
}
