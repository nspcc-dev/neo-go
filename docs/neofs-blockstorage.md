# NeoFS block storage

Using NeoFS to store chain's blocks and snapshots was proposed in
[#3463](https://github.com/neo-project/neo/issues/3463). NeoGo contains several
extensions utilizing NeoFS block storage aimed to improve node synchronization
efficiency and reduce node storage size.

## Components and functionality

### Block storage schema

A single NeoFS container is used to store blocks and index files. Each block
is stored in a binary form as a separate object with a unique OID and a set of
attributes:
 - block object identifier with block index value (`block:1`)
 - primary node index (`primary:0`)
 - block hash in the LE form (`hash:5412a781caf278c0736556c0e544c7cfdbb6e3c62ae221ef53646be89364566b`)
 - previous block hash in the LE form (`prevHash:3654a054d82a8178c7dfacecc2c57282e23468a42ee407f14506368afe22d929`)
 - millisecond-precision block timestamp (`timestamp:1627894840919`)

Each index file is an object containing a constant-sized batch of raw block object
IDs in binary form ordered by block index. Each index file is marked with the
following attributes:
 - index file identifier with consecutive file index value (`index:0`)
 - the number of OIDs included into index file (`size:128000`)

### NeoFS BlockFetcher

NeoFS BlockFetcher service is designed as an alternative to P2P synchronisation
protocol. It allows to download blocks from a trusted container in the NeoFS network
and persist them to database using standard verification flow. NeoFS BlockFetcher
service primarily used during the node's bootstrap, providing a fast alternative to
P2P blocks synchronisation.

NeoFS BlockFetcher service has two modes of operation:
- Index File Search: Search for index files, which contain batches of block object
  IDs and fetch blocks from NeoFS by retrieved OIDs.
- Direct Block Search: Search and fetch blocks directly from NeoFS container via
  built-in NeoFS object search mechanism.

Operation mode of BlockFetcher can be configured via `SkipIndexFilesSearch`
parameter.

#### Operation flow

1. **OID Fetching**:
    Depending on the mode, the service either:
   - Searches for index files by index file attribute and reads block OIDs from index
     file object-by-object.
   - Searches batches of blocks directly by block attribute (the batch size is
     configured via `OIDBatchSize` parameter).

   Once the OIDs are retrieved, they are immediately redirected to the 
   block downloading routines for further processing. The channel that 
   is used to redirect block OIDs to downloading routines is buffered 
   to provide smooth OIDs delivery without delays. The size of this channel 
   can be configured via `OIDBatchSize` parameter and equals to `2*OIDBatchSize`.
2. **Parallel Block Downloading**:
   The number of downloading routines can be configured via 
   `DownloaderWorkersCount` parameter. It's up to the user to find the 
   balance between the downloading speed and blocks persist speed for every 
   node that uses NeoFS BlockFetcher. Downloaded blocks are placed into a 
   buffered channel of size `IDBatchSize` with further redirection to the
   block queue.
3. **Block Insertion**:
   Downloaded blocks are inserted into the blockchain using the same logic
   as in the P2P synchronisation protocol. The block queue is used to order 
   downloaded blocks before they are inserted into the blockchain. The 
   size of the queue can be configured via the `BQueueSize` parameter 
   and should be larger than the `OIDBatchSize` parameter to avoid blocking
   the downloading routines.

Once all blocks available in the NeoFS container are processed, the service
shuts down automatically.
