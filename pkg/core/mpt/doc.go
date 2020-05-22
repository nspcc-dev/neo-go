/*
Package mpt implements MPT (Merkle-Patricia Tree).

MPT stores key-value pairs and is a trie over 16-symbol alphabet. https://en.wikipedia.org/wiki/Trie
Trie is a tree where values are stored in leafs and keys are paths from root to the leaf node.
MPT consists of 4 type of nodes:
- Leaf node contains only value.
- Extension node contains both key and value.
- Branch node contains 2 or more children.
- Hash node is a compressed node and contains only actual node's hash.
  The actual node must be retrieved from storage or over the network.

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
- Extension node cannot have zero-length key
- Extension node cannot have another Extension node in it's next field

Thank to these restrictions, there is a single root hash for every set of key-value pairs
irregardless of the order they were added/removed with.
The actual trie structure can vary because of node -> HashNode compressing.

There is also one optimization which cost us almost nothing in terms of complexity but is very beneficial:
When we perform get/put/delete on a speficic path, every Hash node which was retreived from storage is
replaced by its uncompressed form, so that subsequent hits of this not don't use storage.
*/
package mpt
