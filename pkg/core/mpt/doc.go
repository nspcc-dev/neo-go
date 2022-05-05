/*
Package mpt implements MPT (Merkle-Patricia Trie).

An MPT stores key-value pairs and is a trie over 16-symbol alphabet. https://en.wikipedia.org/wiki/Trie
A trie is a tree where values are stored in leafs and keys are paths from the root to the leaf node.
An MPT consists of 4 types of nodes:
- Leaf node only contains a value.
- Extension node contains both a key and a value.
- Branch node contains 2 or more children.
- Hash node is a compressed node and only contains the actual node's hash.
  The actual node must be retrieved from the storage or over the network.

As an example here is a trie containing 3 pairs:
- 0x1201 -> val1
- 0x1203 -> val2
- 0x1224 -> val3
- 0x12 -> val4

ExtensionNode(0x0102), Next
 _______________________|
 |
BranchNode [0, 1, 2, ...], Last -> Leaf(val4)
            |     |
            |     ExtensionNode [0x04], Next -> Leaf(val3)
            |
            BranchNode [0, 1, 2, 3, ...], Last -> HashNode(nil)
                           |     |
                           |     Leaf(val2)
                           |
                           Leaf(val1)

There are 3 invariants that this implementation has:
- Branch node cannot have <= 1 children
- Extension node cannot have a zero-length key
- Extension node cannot have another Extension node in its next field

Thanks to these restrictions, there is a single root hash for every set of key-value pairs
irregardless of the order they were added/removed in.
The actual trie structure can vary because of node -> HashNode compressing.

There is also one optimization which cost us almost nothing in terms of complexity but is quite beneficial:
When we perform get/put/delete on a specific path, every Hash node which was retrieved from the storage is
replaced by its uncompressed form, so that subsequent hits of this don't need to access the storage.
*/
package mpt
