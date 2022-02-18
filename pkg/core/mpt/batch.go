package mpt

import (
	"bytes"
	"sort"
)

// Batch is batch of storage changes.
// It stores key-value pairs in a sorted state.
type Batch struct {
	kv []keyValue
}

type keyValue struct {
	key   []byte
	value []byte
}

// MapToMPTBatch makes a Batch from unordered set of storage changes.
func MapToMPTBatch(m map[string][]byte) Batch {
	var b Batch

	b.kv = make([]keyValue, 0, len(m))

	for k, v := range m {
		b.kv = append(b.kv, keyValue{strToNibbles(k), v}) // Strip storage prefix.
	}
	sort.Slice(b.kv, func(i, j int) bool {
		return bytes.Compare(b.kv[i].key, b.kv[j].key) < 0
	})
	return b
}

// PutBatch puts batch to trie.
// It is not atomic (and probably cannot be without substantial slow-down)
// and returns number of elements processed.
// If an error is returned, the trie may be in the inconsistent state in case of storage failures.
// This is due to the fact that we can remove multiple children from the branch node simultaneously
// and won't strip the resulting branch node.
// However it is used mostly after the block processing to update MPT and error is not expected.
func (t *Trie) PutBatch(b Batch) (int, error) {
	if len(b.kv) == 0 {
		return 0, nil
	}
	r, n, err := t.putBatch(b.kv)
	t.root = r
	return n, err
}

func (t *Trie) putBatch(kv []keyValue) (Node, int, error) {
	return t.putBatchIntoNode(t.root, kv)
}

func (t *Trie) putBatchIntoNode(curr Node, kv []keyValue) (Node, int, error) {
	switch n := curr.(type) {
	case *LeafNode:
		return t.putBatchIntoLeaf(n, kv)
	case *BranchNode:
		return t.putBatchIntoBranch(n, kv)
	case *ExtensionNode:
		return t.putBatchIntoExtension(n, kv)
	case *HashNode:
		return t.putBatchIntoHash(n, kv)
	case EmptyNode:
		return t.putBatchIntoEmpty(kv)
	default:
		panic("invalid MPT node type")
	}
}

func (t *Trie) putBatchIntoLeaf(curr *LeafNode, kv []keyValue) (Node, int, error) {
	t.removeRef(curr.Hash(), curr.Bytes())
	return t.newSubTrieMany(nil, kv, curr.value)
}

func (t *Trie) putBatchIntoBranch(curr *BranchNode, kv []keyValue) (Node, int, error) {
	return t.addToBranch(curr, kv, true)
}

func (t *Trie) mergeExtension(prefix []byte, sub Node) (Node, error) {
	switch sn := sub.(type) {
	case *ExtensionNode:
		t.removeRef(sn.Hash(), sn.bytes)
		sn.key = append(prefix, sn.key...)
		sn.invalidateCache()
		t.addRef(sn.Hash(), sn.bytes)
		return sn, nil
	case EmptyNode:
		return sn, nil
	case *HashNode:
		n, err := t.getFromStore(sn.Hash())
		if err != nil {
			return sn, err
		}
		return t.mergeExtension(prefix, n)
	default:
		if len(prefix) != 0 {
			e := NewExtensionNode(prefix, sub)
			t.addRef(e.Hash(), e.bytes)
			return e, nil
		}
		return sub, nil
	}
}

func (t *Trie) putBatchIntoExtension(curr *ExtensionNode, kv []keyValue) (Node, int, error) {
	t.removeRef(curr.Hash(), curr.bytes)

	common := lcpMany(kv)
	pref := lcp(common, curr.key)
	if len(pref) == len(curr.key) {
		// Extension must be split into new nodes.
		stripPrefix(len(curr.key), kv)
		sub, n, err := t.putBatchIntoNode(curr.next, kv)
		if err == nil {
			sub, err = t.mergeExtension(pref, sub)
		}
		return sub, n, err
	}

	if len(pref) != 0 {
		stripPrefix(len(pref), kv)
		sub, n, err := t.putBatchIntoExtensionNoPrefix(curr.key[len(pref):], curr.next, kv)
		if err == nil {
			sub, err = t.mergeExtension(pref, sub)
		}
		return sub, n, err
	}
	return t.putBatchIntoExtensionNoPrefix(curr.key, curr.next, kv)
}

func (t *Trie) putBatchIntoExtensionNoPrefix(key []byte, next Node, kv []keyValue) (Node, int, error) {
	b := NewBranchNode()
	if len(key) > 1 {
		b.Children[key[0]] = t.newSubTrie(key[1:], next, false)
	} else {
		b.Children[key[0]] = next
	}
	return t.addToBranch(b, kv, false)
}

func isEmpty(n Node) bool {
	_, ok := n.(EmptyNode)
	return ok
}

// addToBranch puts items into the branch node assuming b is not yet in trie.
func (t *Trie) addToBranch(b *BranchNode, kv []keyValue, inTrie bool) (Node, int, error) {
	if inTrie {
		t.removeRef(b.Hash(), b.bytes)
	}

	// Error during iterate means some storage failure (i.e. some hash node cannot be
	// retrieved from storage). This can leave trie in inconsistent state, because
	// it can be impossible to strip branch node after it has been changed.
	// Consider a branch with 10 children, first 9 of which are deleted and the remaining one
	// is a leaf node replaced by a hash node missing from storage.
	// This can't be fixed easily because we need to _revert_ changes in reference counts
	// for children which were updated successfully. But storage access errors means we are
	// in a bad state anyway.
	n, err := t.iterateBatch(kv, func(c byte, kv []keyValue) (int, error) {
		child, n, err := t.putBatchIntoNode(b.Children[c], kv)
		b.Children[c] = child
		return n, err
	})
	if inTrie && n != 0 {
		b.invalidateCache()
	}

	// Even if some of the children can't be put, we need to try to strip branch
	// and possibly update refcounts.
	nd, bErr := t.stripBranch(b)
	if err == nil {
		err = bErr
	}
	return nd, n, err
}

// stripsBranch strips branch node after incomplete batch put.
// It assumes there is no reference to b in trie.
func (t *Trie) stripBranch(b *BranchNode) (Node, error) {
	var n int
	var lastIndex byte
	for i := range b.Children {
		if !isEmpty(b.Children[i]) {
			n++
			lastIndex = byte(i)
		}
	}
	switch {
	case n == 0:
		return EmptyNode{}, nil
	case n == 1:
		if lastIndex != lastChild {
			return t.mergeExtension([]byte{lastIndex}, b.Children[lastIndex])
		}
		return b.Children[lastIndex], nil
	default:
		t.addRef(b.Hash(), b.bytes)
		return b, nil
	}
}

func (t *Trie) iterateBatch(kv []keyValue, f func(c byte, kv []keyValue) (int, error)) (int, error) {
	var n int
	for len(kv) != 0 {
		c, i := getLastIndex(kv)
		if c != lastChild {
			stripPrefix(1, kv[:i])
		}
		sub, err := f(c, kv[:i])
		n += sub
		if err != nil {
			return n, err
		}
		kv = kv[i:]
	}
	return n, nil
}

func (t *Trie) putBatchIntoEmpty(kv []keyValue) (Node, int, error) {
	common := lcpMany(kv)
	stripPrefix(len(common), kv)
	return t.newSubTrieMany(common, kv, nil)
}

func (t *Trie) putBatchIntoHash(curr *HashNode, kv []keyValue) (Node, int, error) {
	result, err := t.getFromStore(curr.hash)
	if err != nil {
		return curr, 0, err
	}
	return t.putBatchIntoNode(result, kv)
}

// Creates new subtrie from provided key-value pairs.
// Items in kv must have no common prefix.
// If there are any deletions in kv, return error.
// kv is not empty.
// kv is sorted by key.
// value is current value stored by prefix.
func (t *Trie) newSubTrieMany(prefix []byte, kv []keyValue, value []byte) (Node, int, error) {
	if len(kv[0].key) == 0 {
		if kv[0].value == nil {
			if len(kv) == 1 {
				return EmptyNode{}, 1, nil
			}
			node, n, err := t.newSubTrieMany(prefix, kv[1:], nil)
			return node, n + 1, err
		}
		if len(kv) == 1 {
			return t.newSubTrie(prefix, NewLeafNode(kv[0].value), true), 1, nil
		}
		value = kv[0].value
	}

	// Prefix is empty and we have at least 2 children.
	b := NewBranchNode()
	if value != nil {
		// Empty key is always first.
		leaf := NewLeafNode(value)
		t.addRef(leaf.Hash(), leaf.bytes)
		b.Children[lastChild] = leaf
	}
	nd, n, err := t.addToBranch(b, kv, false)
	if err == nil {
		nd, err = t.mergeExtension(prefix, nd)
	}
	return nd, n, err
}

func stripPrefix(n int, kv []keyValue) {
	for i := range kv {
		kv[i].key = kv[i].key[n:]
	}
}

func getLastIndex(kv []keyValue) (byte, int) {
	if len(kv[0].key) == 0 {
		return lastChild, 1
	}
	c := kv[0].key[0]
	for i := range kv[1:] {
		if kv[i+1].key[0] != c {
			return c, i + 1
		}
	}
	return c, len(kv)
}
