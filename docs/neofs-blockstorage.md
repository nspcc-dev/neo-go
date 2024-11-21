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
 - millisecond-precision block timestamp (`Timestamp:1627894840919`)

Each index file is an object containing a constant-sized batch of raw block object
IDs in binary form ordered by block index. Each index file is marked with the
following attributes:
 - index file identifier with consecutive file index value (`Index:0`)
 - the number of OIDs included into index file (`IndexSize:128000`)

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

### NeoFS Upload Command
The `upload-bin` command is designed to fetch blocks from the RPC node and upload 
them to the NeoFS container.
It also creates and uploads index files. Below is an example usage of the command:

```shell
./bin/neo-go util upload-bin --cid 9iVfUg8aDHKjPC4LhQXEkVUM4HDkR7UCXYLs8NQwYfSG --wallet-config ./wallet-config.yml --block-attribute Block --index-attribute Index --rpc-endpoint https://rpc.t5.n3.nspcc.ru:20331 -fsr st1.t5.fs.neo.org:8080 -fsr st2.t5.fs.neo.org:8080 -fsr st3.t5.fs.neo.org:8080
```
The command supports the following options:
```
NAME:
neo-go util upload-bin - Fetch blocks from RPC node and upload them to the NeoFS container

USAGE:
neo-go util upload-bin --fs-rpc-endpoint <address1>[,<address2>[...]] --container <cid> --block-attribute block --index-attribute index --rpc-endpoint <node> [--timeout <time>] --wallet <wallet> [--wallet-config <config>] [--address <address>] [--workers <num>] [--searchers <num>] [--index-file-size <size>] [--skip-blocks-uploading] [--retries <num>] [--debug]

OPTIONS:
--fs-rpc-endpoint value, --fsr value [ --fs-rpc-endpoint value, --fsr value ]  List of NeoFS storage node RPC addresses (comma-separated or multiple --fs-rpc-endpoint flags)
--container value, --cid value                                                 NeoFS container ID to upload blocks to
--block-attribute value                                                        Attribute key of the block object
--index-attribute value                                                        Attribute key of the index file object
--address value                                                                Address to use for signing the uploading and searching transactions in NeoFS
--index-file-size value                                                        Size of index file (default: 128000)
--workers value                                                                Number of workers to fetch, upload and search blocks concurrently (default: 50)
--searchers value                                                              Number of concurrent searches for blocks (default: 20)
--skip-blocks-uploading                                                        Skip blocks uploading and upload only index files (default: false)
--retries value                                                                Maximum number of Neo/NeoFS node request retries (default: 5)
--debug, -d                                                                    Enable debug logging (LOTS of output, overrides configuration) (default: false)
--rpc-endpoint value, -r value                                                 RPC node address
--timeout value, -s value                                                      Timeout for the operation (default: 10s)
--wallet value, -w value                                                       Wallet to use to get the key for transaction signing; conflicts with --wallet-config flag
--wallet-config value                                                          Path to wallet config to use to get the key for transaction signing; conflicts with --wallet flag
--help, -h                                                                     show help
```

This command works as follows:
1. Fetches the current block height from the RPC node.
2. Searches for the oldest half-filled batch of block objects stored in NeoFS. 
3. Fetches missing blocks from the RPC node and uploads them to the NeoFS container 
starting from the oldest half-filled batch.
4. After uploading the blocks, it creates index files for the newly uploaded blocks. 
5. Uploads the created index files to the NeoFS container.

If the command is interrupted, it can be resumed. It starts the uploading process
from the oldest half-filled batch of blocks.