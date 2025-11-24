# NeoFS block storage

Using NeoFS to store chain's blocks and snapshots was proposed in
[#3463](https://github.com/neo-project/neo/issues/3463). NeoGo contains several
extensions utilizing NeoFS block storage aimed to improve node synchronization
efficiency and reduce node storage size.

## Components and functionality

### Block storage schema

A single NeoFS container is used to store blocks and index files. Each container
has network magic attribute (`Magic:56753`). Each block is stored in a binary 
form as a separate object with a unique OID and a set of attributes:
 - block object identifier with block index value (`Block:1`)
 - primary node index (`Primary:0`)
 - block hash in the LE form (`Hash:5412a781caf278c0736556c0e544c7cfdbb6e3c62ae221ef53646be89364566b`)
 - previous block hash in the LE form (`PrevHash:3654a054d82a8178c7dfacecc2c57282e23468a42ee407f14506368afe22d929`)
 - millisecond-precision block creation timestamp (`BlockTime:1627894840919`)
 - second-precision block uploading timestamp (`Timestamp:1627894840`)

Each index file is an object containing a constant-sized batch of raw block object
IDs in binary form ordered by block index. Each index file is marked with the
following attributes:
 - index file identifier with consecutive file index value (`Index:0`)
 - the number of OIDs included into index file (`IndexSize:128000`)
 - second-precision index file uploading timestamp (`Timestamp:1627894840`)

### Contract state storage schema

A single NeoFS container is used to store contract state objects. Each container
has network magic attribute (`Magic:56753`). Each contract state object is stored 
in a binary form as a separate object with a unique OID and a set of attributes:
- state object identifier where the state index number corresponds to the block 
  height, starting from 0 (`State:0`)
- contract state object interval of uploading (`StateSyncInterval:5000`)
- state root hash in the LE form (`StateRoot:58a5157b7e99eeabf631291f1747ec8eb12ab89461cda888492b17a301de81e8`)
- millisecond-precision block creation timestamp (`BlockTime:1468595301000`)
- second-precision contract state object uploading timestamp (`Timestamp:1742957073`)
- hex-encoded binary representation of state root witness (for validated
  stateroots only) (`Witness:c60c408b7c6f48320eb201ed20cce53188a4c6eb989cc71fa49991380d1c53f6527f4b1e014c2f77d7893f372a2f8e1fdf49f4513720a25256196d6531b6c18e4f5d440c4096e6bdeed500011a20f784444d757fe2b3a372d2c63369d0c759dfc5e852b2ae63b41d721aae44eb22284db1e16a032107f1c36986c94e41696f2bcd659827210c40f6d84e945e1f632a2e43b730c728c5f18741868c19f41aa46a151f8f94ea5c880d67f681bd14baa45cfcaa8b1d2c7553f493299e34cd941ba3d6e93044d2392493130c210345e2bbda8d3d9e24d1e9ee61df15d4f435f69a44fe012d86e9cf9377baaa42cd0c210353663d8da8d6c344aade0168c1cfb651db859175a60c48c6bd4000c9e682d0f50c210392fbd1d809a3c62f7dcde8f25454a1570830a21e4b014b3f362a79baf413e1150c21039b45040cc529966165ef5dff3d046a4960520ce616ae170e265d669e0e2de7f414419ed0dc3a`)

The binary form of the contract state object includes serialized sequence of 
contract storage item key-value pairs for every deployed contract including 
native contracts excluding native LedgerContract contract state. The binary 
form of the state object has the following format:

```
1 byte: Version of the contract state object (uint32)
4 bytes: Network magic (uint32)
4 bytes: Block height (uint32)
32 bytes: State root (Uint256)
Variable-length sequence of contract storage key-value pairs, each encoded as:
    Variable-length key (varbytes)
    Variable-length value (varbytes)
```

### NeoFS BlockFetcher

NeoFS BlockFetcher service is designed as an alternative to P2P synchronisation
protocol. It allows to download blocks from a trusted container in the NeoFS network
and persist them to database using standard verification flow. NeoFS BlockFetcher
service primarily used during the node's bootstrap, providing a fast alternative to
P2P blocks synchronisation.

NeoFS BlockFetcher service search and fetch blocks directly from NeoFS container via
built-in NeoFS object search mechanism.

#### Operation flow

1. **OID Fetching**:
   Searches blocks one by one directly by block attribute.
   Once the OIDs are retrieved, they are immediately redirected to the 
   block downloading routines for further processing. The channel that 
   is used to redirect block OIDs to downloading routines is buffered 
   to provide smooth OIDs delivery without delays. The size of this channel 
   can be configured via `OIDBatchSize` parameter and equals to `2*OIDBatchSize`.
2. **Parallel Block Downloading**:
   The number of downloading routines can be configured via 
   `DownloaderWorkersCount` parameter. It's up to the user to find the 
   balance between the downloading speed and blocks persist speed for every 
   node that uses NeoFS BlockFetcher. Downloaded blocks are placed to the
   block queue directly.
3. **Block Insertion**:
   Downloaded blocks are inserted into the blockchain using the same logic
   as in the P2P synchronisation protocol. The block queue is used to order 
   downloaded blocks before they are inserted into the blockchain. The 
   size of the queue can be configured via the `BQueueSize` parameter 
   and should be larger than the `OIDBatchSize` parameter to avoid blocking
   the downloading routines.

Once all blocks available in the NeoFS container are processed, the service
shuts down automatically.

### NeoFS block uploading command
The `util upload-bin` command is designed to fetch blocks from the RPC node and upload 
them to the NeoFS container. The batch size should be consistent from run to run, 
otherwise gaps in the uploaded blocks may occur. Below is an example usage of the command:

```shell
./bin/neo-go util upload-bin --cid 9iVfUg8aDHKjPC4LhQXEkVUM4HDkR7UCXYLs8NQwYfSG --wallet-config ./wallet-config.yml --block-attribute Block --rpc-endpoint https://rpc.t5.n3.nspcc.ru:20331 -fsr st1.t5.fs.neo.org:8080 -fsr st2.t5.fs.neo.org:8080 -fsr st3.t5.fs.neo.org:8080
```

Run `./bin/neo-go util upload-bin --help` to see the full list of supported options.

This command works as follows:
1. Fetches the current block height from the RPC node.
2. Searches in batches for the blocks stored in NeoFS.
3. In batches fetches missing blocks from the RPC node and uploads them to the NeoFS container.

If the command is interrupted, it can be resumed. It starts the uploading process
from the last fully uploaded batch.

For a given block sequence, it is strictly prohibited to change the batch size. 
The batch size can be changed only if uploading has successfully completed up to 
the current chain block height, otherwise it can lead to gaps in the sequence. 

### NeoFS StateFetcher

NeoFS StateFetcher service enables contract state synchronization for non-archival nodes
by fetching contract storage key-value pairs from a trusted NeoFS container. 
It serves as an alternative to P2P-based MPT state synchronization ([P2PStateExchngeExtensions](https://github.com/nspcc-dev/neo-go/blob/f7080f28d7088517de1d624dfdaf247f914486d2/docs/node-configuration.md?plain=1#L421)).
It allows to download contract state objects from a trusted container in the NeoFS 
network and persist them to database. 
NeoFS StateFetcher service primarily used during the node's bootstrap, providing a 
fast alternative to P2P synchronization.

#### Operation flow

1. **State Object Search**:
   Searches the NeoFS container for objects with the configured `StateAttribute`
   (e.g., `State`), filtering by block height aligned with `StateSyncInterval`
   (e.g., heights 0, 40000, 80000).
2. **Storage Data Fetching**:
   Fetches contract storage key-value pairs from the identified state object.
3. **State Synchronization**:
   Passes key-value pairs to the `statesync.Module`, along with the synchronization
   height and expected state root. The `statesync.Module` builds a local MPT trie,
   flushing batches of up to `KeyValueBatchSize` pairs to storage. If the trie’s 
   state root matches the expected root at height P, synchronization process 
   considered to be completed.
4. **Atomic State Jump**:
   If the state root matches the expected root, the `statesync.Module` 
   performs an atomic state jump. The local MPT trie built during 
   synchronization becomes the working trie for normal chain processing.

Once state object fetched from the NeoFS container is processed, the service
shuts down automatically.

### NeoFS state uploading command
The `util upload-state` command is used to start a node, traverse the MPT over the 
smart contract storage, and upload MPT nodes to a NeoFS container at every 
`StateSyncInterval` number of blocks. Below is an example usage of the command:

```shell
./bin/neo-go util upload-state --cid 9iVfUg8aDHKjPC4LhQXEkVUM4HDkR7UCXYLs8NQwYfSG --wallet-config ./wallet-config.yml --state-attribute State -m -fsr st1.t5.fs.neo.org:8080 -fsr st2.t5.fs.neo.org:8080 -fsr st3.t5.fs.neo.org:8080
```

Run `./bin/neo-go util upload-state --help` to see the full list of supported options.

This command works as follows:
1. Searches for the state objects stored in NeoFS to find the latest uploaded object.
2. Checks if new state objects could be uploaded given the current local state height. 
3. Traverses the MPT nodes (pre-order) starting from the stateroot at the height of the 
   latest uploaded state object down to its children.
4. Uploads the MPT nodes to the NeoFS container.
5. Repeats steps 3-4 with a step equal to the `StateSyncInterval` number of blocks.

If the command is interrupted, it can be resumed. It starts the uploading process
from the last uploaded state object.