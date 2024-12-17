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
   - Searches blocks one by one directly by block attribute.

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

### NeoFS Upload Command
The `upload-bin` command is designed to fetch blocks from the RPC node and upload 
them to the NeoFS container. It also creates and uploads index files. Below is an
example usage of the command:

```shell
./bin/neo-go util upload-bin --cid 9iVfUg8aDHKjPC4LhQXEkVUM4HDkR7UCXYLs8NQwYfSG --wallet-config ./wallet-config.yml --block-attribute Block --index-attribute Index --rpc-endpoint https://rpc.t5.n3.nspcc.ru:20331 -fsr st1.t5.fs.neo.org:8080 -fsr st2.t5.fs.neo.org:8080 -fsr st3.t5.fs.neo.org:8080
```

Run `./bin/neo-go util upload-bin --help` to see the full list of supported options.

This command works as follows:
1. Fetches the current block height from the RPC node.
2. Searches for the index files stored in NeoFS.
3. Searches for the stored blocks from the latest incomplete index file. 
4. Fetches missing blocks from the RPC node and uploads them to the NeoFS container.
5. After uploading the blocks, it creates index file based on the uploaded block OIDs. 
6. Uploads the created index file to the NeoFS container.
7. Repeats steps 4-6 until the current block height is reached.

If the command is interrupted, it can be resumed. It starts the uploading process
from the last uploaded index file.

For a given block sequence, only one type of index file is supported. If new index 
files are needed (different `index-file-size` or `index-attribute`), `upload-bin`
will upload the entire block sequence starting from genesis since no migration is
supported yet by this command. Please, add a comment to the
[#3744](https://github.com/nspcc-dev/neo-go/issues/3744) issue if you need this
functionality.