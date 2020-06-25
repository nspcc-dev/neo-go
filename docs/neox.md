# NeoGo support for neox (cross-chain Neo functionality)

NeoGo has full support for neox-2.x functionality integrated in the node, it
doesn't require a separate build or code branch and it's completely controlled
with two configuration options.

## What is neox

Neox is an extension of original Neo 2 node implemented in neox-2.x branch of
C# implementation. It includes the following main changes:
 * local state root generation for contract storages based on MPT
 * consensus updates for state root exchange between CNs and generation of
   verified (signed by CNs) state root
 * P2P protocol updates for state root distribution
 * RPC protocol updates for state status data and proofs generation
 * two new key recovery syscalls for smart contracts

Most of these changes are pure extensions to Neo 2 protocol, but consensus
changes are incompatible with regular Neo 2 nodes. The idea is that we have
now some state reference for each block that can be used by other chains
(along with proof paths for individual key-value pairs if needed) and at the
same time we're able to check non-Neo signatures using new key recovery
functionality that is available for two curves: Secp256r1 and Secp256k1.

### How local state is being generated and what it covers

Any full node processing blocks can now generate state root information
locally using Merkle Patricia Trie (MPT). It's used for any key-value pairs
stored in the database with prefix of `ST_Storage` which is used for contracts
data storage. Basically, anything contracts save using `Neo.Storage.Put`
syscall gets accounted for.

Each value gets a leaf node in MPT and the key for that value is encoded in
branch and extension nodes according to prefix data. Any node in MPT can be
hashed and the root node hash naturally depends on every other hash in the
trie, so this single hash value represents current state of the trie and is
called state root hash. Any change to the trie state
(adding/deleting/changing key-value pairs) changes state root hash.

But even though this state root data can be computed at every full node it
can't be considered authoritative until it's signed by network-trusted
entities which are consensus nodes.

### How and why consensus process was changed in neox

Consensus nodes now exchange state root information with PrepareRequest
messages, so the Primary node tells everyone its current state root hash
(along with the block index that state root corresponds to) and the hash of
the previous state root message. This data might also be versioned in case of
future updates, so there is a special field reserved for that too, but at the
moment it's always 0. Backups either confirm this data (if it matches their
local state) by proceeding with PrepareResponse or request a ChangeView if
there is some mismatch detected.

If all goes well CNs generate a signature for this state root data and
exchange it with their Commit messages (along with new block
signatures). Effectively this creates another signed chain on the network that
is always one block behind from the main chain because the process of block `N`
creation confirms the state resulting from processing of block `N - 1`. A
separate `stateroot` message is generated and sent along with the new block
broadcast.

### How P2P protocol was changed

P2P protocol was extended with `getroots`, `roots` and `stateroot`
messages for state root data exchange. Simple `stateroot` message is what
consensus nodes generate to broadcast signed state root data, it's accepted by
all nodes, they check it, verify its signature and save locally (to do that
they have to have confirmed state root for the previous block). It's somewhat
similar to block announcement, but as this message is rather small, `inv` is
not being used.

But this message might get lost or some new node may join the network and want
to get verification for its state, so there has to be some possibility for
state root requests and replies and that's what `getroots`/`roots` pair is
for. In general it's expected that the node would synchronize state roots the
same way it synchronizes blocks, always trying to be up to date with the
network. From this synchronization comes the concept of "state height" which
represents the latest verified state root known to the node.

### How RPC protocol was changed

RPC got extended with four new methods: `getproof`, `getstateheight`,
`getstateroot` and `verifyproof`.

`getstateheight` and `getstateroot` are easy, the first one allows to get
current node's block and state heights, while the second one returns state
root data for the specified (by index or by hash) block. State root data
basically mirrors the one exchanged via P2P protocol (version, previous state
root message hash and current state root hash), but also contains an
additional flag to specify if the node has a verification (signature) for this
state root. If the state is verified then the node also includes witness data
for this state root which use the same format transaction's witnesses use.

`getproof` and `verifyproof` methods are a bit more special as they allow you
to prove that some key-value pair exists in Neo state DB without having whole
state DB (like when you're operating on a different chain or when you're
working as a light node). This works via MPT path encoding from the root node
to the particular leaf (value) node you're interested in (that contains some
token balance for example). Using this path data it's easy to regenerate a
part of MPT corresponding to that key-value pair locally and recalculate
MPT hashes for that trie. If the top-level hash matches verified root hash
then you have a proof that the key-value pair is a part of the state DB shared
by all proper Neo nodes.

So `getproof` method returns this path from the root node to the given
key. It can then be used to verify the proof locally or can be used to send
this proof to some trusted RPC node to verify it using `verifyproof` method
that returns value for that key in case of success.

### What are these new neox syscalls

Two syscalls were added along with other neox changes:
"Neo.Cryptography.Secp256k1Recover" and "Neo.Cryptography.Secp256r1Recover",
they're similar in their function and interface, but using different elliptic
curves for their operation. The first one uses SEC-standardized Koblitz curve
widely known for its usage in Bitcoin and the second one operates on regular
SEC-standardized curve that is used by Neo.

Both of these syscalls allow to recover public key from the given signature
(r, s) on the given message hash with a help of a flag denoting Y's least
significant bit in decompression algorithm. The return value is a byte
array representing recovered public key (64 bytes containing 32-byte X and Y)
in case of success and zero-length byte array in case of failure.

This functionality allows you to check message signatures in smart contract,
the key recovered can be compared with an expected one or be hashed and
compared with an expected key hash (depending on what data is provided by the
other blockchain).

## How neox is supported in NeoGo

NeoGo has full support for functionality outlined above. Syscalls are
available via interop wrappers in `crypto` packages and RPC client contains
methods to work with new RPC protocol extensions. Client-side support is
always available, but NeoGo node's behavior is controlled by two configuration
options: EnableStateRoot and StateRootEnableIndex, the first one is boolean
and the second one is integer. If not specified in the configuration the first
one has a default of false and the second has a default value of 0.

EnableStateRoot controls state root generation and processing
functionality. NeoGo is able to operate both on stateroot-enabled and classic
networks, so this is the main switch between these two modes.

With EnableStateRoot set to false the node works in classic mode:
 * no local state root is being generated
 * consensus process operates using classic message formats not including
   state root data
 * stateroot-related P2P messages are ignored
 * stateroot-related RPC calls are available, but always return an error
 * recovery syscalls are unavailable to contracts
 * StateRootEnableIndex setting is ignored

With EnableStateRoot set to true things change and the node operates with full
neox support, but a StateRootEnableIndex setting may additionally affect its
P2P-processing behavior. `getroots` requests for blocks with height less than
StateRootEnableIndex are ignored, `roots` messages are only processed for
blocks higher than StateRootEnableIndex and the node doesn't actively try to
synchronize its state height until its block height reaches
StateRootEnableIndex. This setting is made for network upgrades when there are
no confirmed state roots for old blocks and they'll never be properly
confirmed.

### Things you can do

#### Running a classic network

Doesn't require changing anything, just upgrade the node and run it.

#### Running new stateroot-enabled network

Setting EnableStateRoot to true and not setting StateRootEnableIndex is a good
choice for a new private network as it gives you all the functionality from
block zero. Note that all consensus nodes must be using this settings
combination for successful operation.

#### Adding stateroot functionality to existing network

If you already have some network and you need it to continue working, but want
to upgrade it with neox functionality you need to:
 * prepare a current dump of network's blocks
 * upgrade all consensus nodes with NeoGo 0.76.0+
 * stop all of them
 * change their configuration, setting EnableStateRoot to true and
   StateRootEnableIndex to some block in the future (not far away from current
   network's height)
 * remove CNs local databases
 * import blocks from the previously generated dump on all CNs
 * start all CNs
 
This can be optimized to reduce network's downtime by doing block
dumps/restores with old CNs still running, but you have to regenerate local
databases with stateroot enabled for correct operation.

### Things you shouldn't do

#### Randomly changing EnableStateRoot setting

Switching EnableStateRoot on and off without full block resynchronization may
lead to unexpected results on any full node (independent of whether it's a
consensus node or not) because with EnableStateRoot set to true an MPT
structure is initialized using local DB and if that DB doesn't have correct
MPT state it will fail. If you're changing this setting in any way --- restore
the DB from block dump.

#### Running mixed consensus nodes set

All consensus nodes should agree on the protocol being used, either all of
them use state roots, or all of them don't. Mixing two types of nodes will
lead to consensus failures.
