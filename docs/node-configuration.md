# NeoGo node configuration file

This section contains detailed NeoGo node configuration file description
including default config values and some tips to set up configurable values.

Each config file contains two sections. `ApplicationConfiguration` describes node-related
settings and `ProtocolConfiguration` contains protocol-related settings. See the
[Application Configuration](#Application-Configuration) and
[Protocol Configuration](#Protocol-Configuration) sections for details on configurable
values.

## Application Configuration

`ApplicationConfiguration` section of `yaml` node configuration file contains
node-related settings described in the table below.

| Section | Type | Default value | Description |
| --- | --- | --- | --- |
| DBConfiguration | [DB Configuration](#DB-Configuration) |  | Describes configuration for database. See the [DB Configuration](#DB-Configuration) section for details. |
| LogLevel | `string` | "info" | Minimal logged messages level (can be "debug", "info", "warn", "error", "dpanic", "panic" or "fatal"). |
| GarbageCollectionPeriod | `uint32` | 10000 | Controls MPT garbage collection interval (in blocks) for configurations with `RemoveUntraceableBlocks` enabled and `KeepOnlyLatestState` disabled. In this mode the node stores a number of MPT trees (corresponding to `MaxTraceableBlocks` and `StateSyncInterval`), but the DB needs to be clean from old entries from time to time. Doing it too often will cause too much processing overhead, doing it too rarely will leave more useless data in the DB. |
| KeepOnlyLatestState | `bool` | `false` | Specifies if MPT should only store the latest state (or a set of latest states, see `P2PStateExchangeExtensions` section in the ProtocolConfiguration for details). If true, DB size will be smaller, but older roots won't be accessible. This value should remain the same for the same database. |  |
| LogPath | `string` | "", so only console logging | File path where to store node logs. |
| NeoFSBlockFetcher | [NeoFS BlockFetcher Configuration](#NeoFS-BlockFetcher-Configuration) | | NeoFS BlockFetcher module configuration. See the [NeoFS BlockFetcher Configuration](#NeoFS-BlockFetcher-Configuration) section for details. |
| Oracle | [Oracle Configuration](#Oracle-Configuration) | | Oracle module configuration. See the [Oracle Configuration](#Oracle-Configuration) section for details. |
| P2P | [P2P Configuration](#P2P-Configuration) | | Configuration values for P2P network interaction. See the [P2P Configuration](#P2P-Configuration) section for details. |
| P2PNotary | [P2P Notary Configuration](#P2P-Notary-Configuration) | | P2P Notary module configuration. See the [P2P Notary Configuration](#P2P-Notary-Configuration) section for details. |
| Pprof | [Metrics Services Configuration](#Metrics-Services-Configuration) | | Configuration for pprof service (profiling statistics gathering). See the [Metrics Services Configuration](#Metrics-Services-Configuration) section for details. |
| Prometheus | [Metrics Services Configuration](#Metrics-Services-Configuration) | | Configuration for Prometheus (monitoring system). See the [Metrics Services Configuration](#Metrics-Services-Configuration) section for details |
| Relay | `bool` | `true` | Determines whether the server is forwarding its inventory. |
| Consensus | [Consensus Configuration](#Consensus-Configuration) |  | Describes consensus (dBFT) configuration. See the [Consensus Configuration](#Consensus-Configuration) for details. |
| RemoveUntraceableBlocks | `bool`| `false` | Denotes whether old blocks should be removed from cache and database. If enabled, then only the last `MaxTraceableBlocks` are stored and accessible to smart contracts. Old MPT data is also deleted in accordance with `GarbageCollectionPeriod` setting. If enabled along with `P2PStateExchangeExtensions` protocol extension, then old blocks and MPT states will be removed up to the second latest state synchronisation point (see `StateSyncInterval`). |
| RemoveUntraceableHeaders | `bool`| `false` | Used only with RemoveUntraceableBlocks and makes node delete untraceable block headers as well. Notice that this is an experimental option, not recommended for production use. |
| RPC | [RPC Configuration](#RPC-Configuration) |  | Describes [RPC subsystem](rpc.md) configuration. See the [RPC Configuration](#RPC-Configuration) for details. |
| SaveStorageBatch | `bool` | `false` | Enables storage batch saving before every persist. It is similar to StorageDump plugin for C# node. |
| SkipBlockVerification | `bool` | `false` | Allows to disable verification of received/processed blocks (including cryptographic checks). |
| StateRoot | [State Root Configuration](#State-Root-Configuration) |  | State root module configuration. See the [State Root Configuration](#State-Root-Configuration) section for details. |

### P2P Configuration

`P2P` section contains configuration for peer-to-peer node communications and has
the following format:
```
P2P:
  Addresses:
    - "0.0.0.0:0" # any free port on all available addresses (in form of "[host]:[port][:announcedPort]")
  AttemptConnPeers: 20
  BroadcastFactor: 0
  DialTimeout: 0s
  MaxPeers: 100
  MinPeers: 5
  PingInterval: 30s
  PingTimeout: 90s
  ProtoTickInterval: 5s
  ExtensiblePoolSize: 20
```
where:
- `Addresses` (`[]string`) is the list of the node addresses that P2P protocol
   handler binds to. Each address has the form of `[address]:[nodePort][:announcedPort]`
   where `address` is the address itself, `nodePort` is the actual P2P port node listens at;
   `announcedPort` is the node port which should be used to announce node's port on P2P layer,
   it can differ from the `nodePort` the node is bound to if specified (for example, if your
   node is behind NAT).
- `AttemptConnPeers` (`int`) is the number of connection to try to establish when the
   connection count drops below the `MinPeers` value.
- `BroadcastFactor` (`int`) is the multiplier that is used to determine the number of
   optimal gossip fan-out peer number for broadcasted messages (0-100). By default, it's
   zero, node uses the most optimized value depending on the estimated network size
   (`2.5×log(size)`), so the node may have 20 peers and calculate that it needs to broadcast
   messages to just 10 of them. With BroadcastFactor set to 100 it will always send messages
   to all peers, any value in-between 0 and 100 is used for weighted calculation, for example
   if it's 30 then 13 neighbors will be used in the previous case.
- `DialTimeout` (`Duration`) is the maximum duration a single dial may take.
- `ExtensiblePoolSize` (`int`) is the maximum amount of the extensible payloads from a single
   sender stored in a local pool.
- `MaxPeers` (`int`) is the maximum numbers of peers that can be connected to the server.
- `MinPeers` (`int`) is the minimum number of peers for normal operation; when the node has
   less than this number of peers it tries to connect with some new ones. Note that consensus
   node won't start the consensus process until at least `MinPeers` number of peers are
   connected.
- `PingInterval` (`Duration`) is the interval used in pinging mechanism for syncing
   blocks.
- `PingTimeout` (`Duration`) is the time to wait for pong (response for sent ping request).
- `ProtoTickInterval` (`Duration`) is the duration between protocol ticks with each
   connected peer.

### DB Configuration

`DBConfiguration` section describes configuration for node database and has
the following format:
```
DBConfiguration:
  Type: leveldb
  LevelDBOptions:
    DataDirectoryPath: /chains/privnet
    ReadOnly: false
  BoltDBOptions:
    FilePath: ./chains/privnet.bolt
    ReadOnly: false
```
where:
- `Type` is the database type (string value). Supported types: `leveldb`, `boltdb` and
  `inmemory` (not recommended for production usage).
- `LevelDBOptions` are settings for LevelDB. Includes the DB files path and ReadOnly mode toggle.
  If ReadOnly mode is on, then an error will be returned on attempt to connect to unexisting or empty
  database. Database doesn't allow changes in this mode, a warning will be logged on DB persist attempts.
- `BoltDBOptions` configures BoltDB. Includes the DB files path and ReadOnly mode toggle. If ReadOnly
  mode is on, then an error will be returned on attempt to connect with unexisting or empty database.
  Database doesn't allow changes in this mode, a warning will be logged on DB persist attempts.

Only options for the specified database type will be used.

### Oracle Configuration

`Oracle` configuration section describes configuration for Oracle node module
and has the following structure:
```
Oracle:
  Enabled: false
  AllowPrivateHost: false
  MaxTaskTimeout: 3600s
  MaxConcurrentRequests: 10
  Nodes: ["172.200.0.1:30333", "172.200.0.2:30334"]
  NeoFS:
    Nodes: ["172.200.0.1:30335", "172.200.0.2:30336"]
    Timeout: 2
  RefreshInterval: 180s
  RequestTimeout: 5s
  ResponseTimeout: 5s
  UnlockWallet:
    Path: "./oracle_wallet.json"
    Password: "pass"
```

Please, refer to the [Oracle module documentation](./oracle.md#Configuration) for
details on configurable values.

### P2P Notary Configuration

`P2PNotary` configuration section describes configuration for P2P Notary node
module and has the following structure:
```
P2PNotary:
  Enabled: false
  UnlockWallet:
    Path: "/notary_wallet.json"
    Password: "pass"
```
where:
- `Enabled` denotes whether P2P Notary module is active.
- `UnlockWallet` is a Notary node wallet configuration, see the
  [Unlock Wallet Configuration](#Unlock-Wallet-Configuration) section for
  structure details.

Please, refer to the [Notary module documentation](./notary.md#Notary node module) for
details on module features.

### NeoFS BlockFetcher Configuration

`NeoFSBlockFetcher` configuration section contains settings for NeoFS
BlockFetcher module and has the following structure:
```
  NeoFSBlockFetcher:
    Enabled: true
    UnlockWallet:
      Path: "./wallet.json"
      Password: "pass"
    Addresses:
      - st1.storage.fs.neo.org:8080
      - st2.storage.fs.neo.org:8080
      - st3.storage.fs.neo.org:8080
      - st4.storage.fs.neo.org:8080
    Timeout: 10m
    DownloaderWorkersCount: 500
    OIDBatchSize: 8000
    BQueueSize: 16000
    SkipIndexFilesSearch: false
    ContainerID: "7a1cn9LNmAcHjESKWxRGG7RSZ55YHJF6z2xDLTCuTZ6c"
    BlockAttribute: "Block"
    IndexFileAttribute: "Index"
    IndexFileSize: 128000
```
where:
- `Enabled` enables NeoFS BlockFetcher module.
- `UnlockWallet` contains wallet settings to retrieve account to sign requests to
  NeoFS. Without this setting, the module will use randomly generated private key.
  For configuration details see [Unlock Wallet Configuration](#Unlock-Wallet-Configuration)
- `Addresses` is a list of NeoFS storage nodes addresses. This parameter is required.
- `Timeout` is a timeout for a single request to NeoFS storage node (10 minutes by
  default).
- `ContainerID` is a container ID to fetch blocks from. This parameter is required.
- `BlockAttribute` is an attribute name of NeoFS object that contains block
  data. It's set to `Block` by default.
- `IndexFileAttribute` is an attribute name of NeoFS index object that contains block
  object IDs. It's set to `Index` by default.
- `DownloaderWorkersCount` is a number of workers that download blocks from
  NeoFS in parallel (500 by default).
- `OIDBatchSize` is the number of blocks to search per a single request to NeoFS
  in case of disabled index files search. Also, for both modes of BlockFetcher
  operation this setting manages the buffer size of OIDs and blocks transferring
  channels. By default, it's set to a half of `BQueueSize` parameter.
- `BQueueSize` is a size of the block queue used to manage consecutive blocks
  addition to the chain. It must be larger than `OIDBatchSize` and highly recommended
  to be `2*OIDBatchSize` or `3*OIDBatchSize`. By default, it's set to 16000.
- `SkipIndexFilesSearch` is a flag that allows to skip index files search and search
  for blocks directly. It is set to `false` by default.
- `IndexFileSize` is the number of OID objects stored in the index files. This
  setting depends on the NeoFS block storage configuration and is applicable only if
  `SkipIndexFilesSearch` is set to `false`. It's set to 128000 by default.

### Metrics Services Configuration

Metrics services configuration describes options for metrics services (pprof,
Prometheus) and has the following structure:
```
Pprof:
  Enabled: false
  Addresses:
    - ":30001"
Prometheus:
  Enabled: false
  Addresses:
    - ":40001"
```
where:
- `Enabled` denotes whether the service is enabled.
- `Addresses` is a list of service addresses to be running at and listen to in
   the form of "host:port".

### RPC Configuration

`RPC` configuration section describes settings for the RPC server and has
the following structure:
```
RPC:
  Enabled: true
  Addresses:
    - ":10332"
  EnableCORSWorkaround: false
  MaxGasInvoke: 50
  MaxIteratorResultItems: 100
  MaxFindResultItems: 100
  MaxFindStoragePageSize: 50
  MaxNEP11Tokens: 100
  MaxRequestBodyBytes: 5242880
  MaxRequestHeaderBytes: 1048576
  MaxWebSocketClients: 64
  SessionEnabled: false
  SessionExpirationTime: 15
  SessionBackedByMPT: false
  SessionPoolSize: 20
  StartWhenSynchronized: false
  TLSConfig:
    Addresses:
      - ":10331"
    CertFile: serv.crt
    Enabled: true
    KeyFile: serv.key
```
where:
- `Enabled` denotes whether an RPC server should be started.
- `Addresses` is a list of RPC server addresses to be running at and listen to in
  the form of "host:port".
- `EnableCORSWorkaround` turns on a set of origin-related behaviors that make
  RPC server wide open for connections from any origins. It enables OPTIONS
  request handling for pre-flight CORS and makes the server send
  `Access-Control-Allow-Origin` and `Access-Control-Allow-Headers` headers for
  regular HTTP requests (allowing any origin which effectively makes CORS
  useless). It also makes websocket connections work for any `Origin`
  specified in the request header. This option is not recommended (reverse
  proxy can be used to have proper app-specific CORS settings), but it's an
  easy way to make RPC interface accessible from the browser.
- `MaxGasInvoke` is the maximum GAS allowed to spend during `invokefunction` and
  `invokescript` RPC-calls. `calculatenetworkfee` also can't exceed this GAS amount
  (normally the limit for it is MaxVerificationGAS from Policy, but if MaxGasInvoke
  is lower than that then this limit is respected).
- `MaxIteratorResultItems` - maximum number of elements extracted from iterator
   returned by `invoke*` call. When the `MaxIteratorResultItems` value is set to
   `n`, only `n` iterations are returned and truncated is true, indicating that
   there is still data to be returned.
- `MaxFindResultItems` - the maximum number of elements for `findstates` response.
- `MaxFindStoragePageSize` - the maximum number of elements for `findstorage` response per single page.
- `MaxNEP11Tokens` - limit for the number of tokens returned from
  `getnep11balances` call.
- `MaxRequestBodyBytes` - the maximum allowed HTTP request body size in bytes
  (5MB by default).
- `MaxRequestHeaderBytes` - the maximum allowed HTTP request header size in bytes
  (1MB by default).
- `MaxWebSocketClients` - the maximum simultaneous websocket client connection
  number (64 by default). Attempts to establish additional connections will
  lead to websocket handshake failures. Use "-1" to disable websocket
  connections (0 will lead to using the default value).
- `SessionEnabled` denotes whether session-based iterator JSON-RPC API is enabled.
  If true, then all iterators got from `invoke*` calls will be stored as sessions
  on the server side available for further traverse. `traverseiterator` and
  `terminatesession` JSON-RPC calls will be handled by the server. It is not
  recommended to enable this setting for public RPC servers due to possible DoS
  attack. Set to `false` by default. If `false`, iterators are expanded into a
  set of values (see `MaxIteratorResultItems` setting). Implementation note: when
  BoltDB storage is used as a node backend DB, then enabling iterator sessions may
  cause blockchain persist delays up to 2*`SessionExpirationTime` seconds on
  early blockchain lifetime stages with relatively small DB size. It can happen
  due to BoltDB re-mmapping behaviour traits. If regular persist is a critical
  requirement, then we recommend either to decrease `SessionExpirationTime` or to
  enable `SessionBackedByMPT`, see `SessionBackedByMPT` documentation for more
  details.
- `SessionExpirationTime` is a lifetime of iterator session in seconds. It is set
  to `TimePerBlock` seconds (but not less than 5s) by default and is relevant
  only if `SessionEnabled` is set to `true`.
- `SessionBackedByMPT` is a flag forcing JSON-RPC server into using MPT-backed
  storage for delayed iterator traversal. If `true`, then iterator resources got
  after `invoke*` calls will be released immediately. Further iterator traversing
  will be performed using MPT-backed storage by retrieving iterator via historical
  MPT-provided `invoke*` recall. `SessionBackedByMPT` set to `true` strongly affects
  the `traverseiterator` call performance and doesn't allow iterator traversing
  for outdated or removed states (see `KeepOnlyLatestState` and
  `RemoveUntraceableBlocks` settings documentation for details), thus, it is not
  recommended to enable `SessionBackedByMPT` needlessly. `SessionBackedByMPT` is
  set to `false` by default and is relevant only if `SessionEnabled` is set to
  `true`.
- `SessionPoolSize` is the maximum number of concurrent iterator sessions. It is
  set to `20` by default. If the subsequent session can't be added to the session
  pool, then invocation result will contain corresponding error inside the
  `FaultException` field.
- `StartWhenSynchronized` controls when RPC server will be started, by default
  (`false` setting) it's started immediately and RPC is available during node
  synchronization. Setting it to `true` will make the node start RPC service only
  after full synchronization.
- `TLS` section configures TLS protocol.

### State Root Configuration

`StateRoot` configuration section contains settings for state roots exchange and has
the following structure:
```
StateRoot:
  Enabled: false
  UnlockWallet:
    Path: "./wallet.json"
    Password: "pass"
```
where:
- `Enabled` enables state root module.
- `UnlockWallet` contains wallet settings, see
  [Unlock Wallet Configuration](#Unlock-Wallet-Configuration) section for
  structure details.

### Consensus Configuration

`Consensus` configuration section describes configuration for dBFT node
module and has the following structure:
```
Consensus:
  Enabled: false
  UnlockWallet:
    Path: "/consensus_node_wallet.json"
    Password: "pass"
```
where:
- `Enabled` denotes whether dBFT module is active.
- `UnlockWallet` is a consensus node wallet configuration, see the
  [Unlock Wallet Configuration](#Unlock-Wallet-Configuration) section for
  structure details.

Please, refer to the [consensus node documentation](./consensus.md) for more
details on consensus node setup.

### Unlock Wallet Configuration

`UnlockWallet` configuration section contains wallet settings and has the following
structure:
```
UnlockWallet:
  Path: "./wallet.json"
  Password: "pass"
```
where:
- `Path` is a path to wallet.
- `Password` is a wallet password.

## Protocol Configuration

`ProtocolConfiguration` section of `yaml` node configuration file contains
protocol-related settings described in the table below.

| Section | Type | Default value | Description | Notes |
| --- | --- | --- | --- | --- |
| CommitteeHistory | map[uint32]uint32 | none | Number of committee members after the given height, for example `{0: 1, 20: 4}` sets up a chain with one committee member since the genesis and then changes the setting to 4 committee members at the height of 20. `StandbyCommittee` committee setting must have the number of keys equal or exceeding the highest value in this option. Blocks numbers where the change happens must be divisible by the old and by the new values simultaneously. If not set, committee size is derived from the `StandbyCommittee` setting and never changes. |
| Genesis | [Genesis](#Genesis-Configuration) | none | The set of genesis block settings including NeoGo-specific protocol extensions that should be enabled at the genesis block or during native contracts initialisation. |
| Hardforks | `map[string]uint32` | [] | The set of incompatible changes that affect node behaviour starting from the specified height. The default value is an empty set which should be interpreted as "each known stable hard-fork is applied from the zero blockchain height". See [Hardforks](#Hardforks) section for a list of supported keys. |
| Magic | `uint32` | `0` | Magic number which uniquely identifies Neo network. |
| MaxBlockSize | `uint32` | `262144` | Maximum block size in bytes. |
| MaxBlockSystemFee | `int64` | `900000000000` | Maximum overall transactions system fee per block. |
| MaxTraceableBlocks | `uint32` | `2102400` | Length of the chain accessible to smart contracts. | `RemoveUntraceableBlocks` should be enabled to use this setting. |
| MaxTransactionsPerBlock | `uint16` | `512` | Maximum number of transactions per block. |
| MaxValidUntilBlockIncrement | `uint32` | `5760` | Upper height increment limit for transaction's ValidUntilBlock field value relative to the current blockchain height, exceeding which a transaction will fail validation. It is set to estimated daily number of blocks with 15s interval by default. |
| MemPoolSize | `int` | `50000` | Size of the node's memory pool where transactions are stored before they are added to block. |
| P2PNotaryRequestPayloadPoolSize | `int` | `1000` | Size of the node's P2P Notary request payloads memory pool where P2P Notary requests are stored before main or fallback transaction is completed and added to the chain.<br>This option is valid only if `P2PSigExtensions` are enabled. | Not supported by the C# node, thus may affect heterogeneous networks functionality. |
| P2PSigExtensions | `bool` | `false` | Enables following additional Notary service related logic:<br>• Transaction attribute `NotaryAssisted`<br>• Network payload of the `P2PNotaryRequest` type<br>• Native `Notary` contract<br>• Notary node module | Not supported by the C# node, thus may affect heterogeneous networks functionality. |
| P2PStateExchangeExtensions | `bool` | `false` | Enables the following P2P MPT state data exchange logic: <br>• `StateSyncInterval` protocol setting <br>• P2P commands `GetMPTDataCMD` and `MPTDataCMD` | Not supported by the C# node, thus may affect heterogeneous networks functionality. Can be supported either on MPT-complete node (`KeepOnlyLatestState`=`false`) or on light GC-enabled node (`RemoveUntraceableBlocks=true`) in which case `KeepOnlyLatestState` setting doesn't change the behavior, an appropriate set of MPTs is always stored (see `RemoveUntraceableBlocks`). |
| ReservedAttributes | `bool` | `false` | Allows to have reserved attributes range for experimental or private purposes. |
| SeedList | `[]string` | [] | List of initial nodes addresses used to establish connectivity. |
| StandbyCommittee | `[]string` | [] | List of public keys of standby committee validators are chosen from. | The list of keys is not required to be sorted, but it must be exactly the same within the configuration files of all the nodes in the network. |
| StateRootInHeader | `bool` | `false` | Enables storing state root in block header. | Experimental protocol extension! |
| StateSyncInterval | `int` | `40000` | The number of blocks between state heights available for MPT state data synchronization. | `P2PStateExchangeExtensions` should be enabled to use this setting. |
| TimePerBlock | `Duration` | `15s` | Minimal (and targeted for) time interval between blocks. Must be an integer number of milliseconds. |
| ValidatorsCount | `uint32` | `0` | Number of validators set for the whole network lifetime, can't be set if `ValidatorsHistory` setting is used. |
| ValidatorsHistory | map[uint32]uint32 | none | Number of consensus nodes to use after given height (see `CommitteeHistory` also). Heights where the change occurs must be divisible by the number of committee members at that height. Can't be used with `ValidatorsCount` not equal to zero. Initial validators count for genesis block must always be specified. |
| VerifyTransactions | `bool` | `false` | Denotes whether to verify transactions in the received blocks. |

### Genesis Configuration

`Genesis` subsection of protocol configuration section contains a set of settings
specific for genesis block including NeoGo node extensions that should be enabled
during genesis block persist or at the moment of native contracts initialisation.
`Genesis` has the following structure:
```
Genesis:
  Roles:
    NeoFSAlphabet:
      - 033238fa63bd08115ebf442d4af897eea2f6866e4c2001cd1f6e7656acdd91a5d3
      - 03b209fd4f53a7170ea4444e0cb0a6bb6a53c2bd016926989cf85f9b0fba17a70c
      - 02aaec38470f6aad0042c6e877cfd8087d2676b0f516fddd362801b9bd3936399e
      - 03c6aa6e12638b36e88adc1ccdceac4db9929575c3e03576c617c49cce7114a050
    Oracle:
      - 03409f31f0d66bdc2f70a9730b66fe186658f84a8018204db01c106edc36553cd0
      - 0222038884bbd1d8ff109ed3bdef3542e768eef76c1247aea8bc8171f532928c30
  Transaction:
    Script: "DCECEDp/fdAWVYWX95YNJ8UWpDlP2Wi55lFV60sBPkBAQG5BVuezJw=="
    SystemFee: 100000000
```
where:
- `Roles` is a map from node roles that should be set at the moment of native
  RoleManagement contract initialisation to the list of hex-encoded public keys
  corresponding to this role. The set of valid roles includes:
  - `StateValidator`
  - `Oracle`
  - `NeoFSAlphabet`
  - `P2PNotary`
  
  Roles designation order follows the enumeration above. Designation
  notifications will be emitted after each configured role designation.
  
  Note that Roles is a NeoGo extension that isn't supported by the NeoC# node and
  must be disabled on the public Neo N3 networks. Roles extension is compatible
  with Hardforks setting, which means that specified roles will be set
  only during native RoleManagement contract initialisation (which may be
  performed in some non-genesis hardfork). By default, no roles are designated.

- `Transaction` is a container for transaction script that should be deployed in
  the genesis block if provided. `Transaction` includes `Script` which is a
  base64-encoded transaction script and `SystemFee` which is a transaction's
  system fee value (in GAS) that will be spent during transaction execution.
  Transaction generated from the provided parameters has two signers at max with
  CalledByEntry witness scope: the first one is standby validators multisignature
  signer and the second one (if differs from the first) is committee
  multisignature signer.

  Note that `Transaction` is a NeoGo extension that isn't supported by the NeoC#
  node and must be disabled on the public Neo N3 networks.

### Hardforks

The latest stable hardfork as per 0.107.1 release is Domovoi. Echidna is still
in development and can change in an incompatible way.

| Name            | Changes | References |
| --- | --- | --- |
| `Aspidochelone` | Adjusts the price of `System.Contract.CreateStandardAccount` and `System.Contract.CreateMultisigAccount` interops so that the resulting prices are in accordance with `sha256` method of native `CryptoLib` contract. Also adjusts the price of `System.Runtime.GetRandom` interop and fixes its vulnerability. A special NeoGo-specific change is included as well for ContractManagement's update/deploy call flags behaviour to be compatible with pre-0.99.0 behaviour that was changed because of the 3.2.0 protocol change | https://github.com/nspcc-dev/neo-go/pull/2469 <br> https://github.com/neo-project/neo/pull/2712 <br> https://github.com/nspcc-dev/neo-go/pull/2519 <br> https://github.com/neo-project/neo/pull/2749 <br> https://github.com/neo-project/neo/pull/2653 |
| `Basilisk`      | Enables strict smart contract script check against a set of JMP instructions and against method boundaries enabled on contract deploy or update. Increases `stackitem.Integer` JSON parsing precision up to the maximum value supported by the NeoVM. Enables strict check for notifications emitted by a contract to precisely match the events specified in the contract manifest. | https://github.com/nspcc-dev/neo-go/pull/3056 <br> https://github.com/neo-project/neo/pull/2881 <br> https://github.com/nspcc-dev/neo-go/pull/3080 <br> https://github.com/neo-project/neo/pull/2883 <br> https://github.com/nspcc-dev/neo-go/pull/3085 <br> https://github.com/neo-project/neo/pull/2810 |
| `Cockatrice`    | Introduces the ability to update native contracts. Includes a couple of new native smart contract APIs: `keccak256` of native CryptoLib contract and `getCommitteeAddress` of native NeoToken contract. | https://github.com/nspcc-dev/neo-go/pull/3402 <br> https://github.com/neo-project/neo/pull/2942 <br> https://github.com/nspcc-dev/neo-go/pull/3301 <br> https://github.com/neo-project/neo/pull/2925 <br> https://github.com/nspcc-dev/neo-go/pull/3362 <br> https://github.com/neo-project/neo/pull/3154 |
| `Domovoi`       | Makes node use executing contract state for the contract call permissions check instead of the state stored in the native Management contract. In C# also makes System.Runtime.GetNotifications interop properly count stack references of notification parameters which prevents users from creating objects that exceed MaxStackSize constraint, but NeoGo has never had this bug, thus proper behaviour is preserved even before HFDomovoi. It results in the fact that some T5 testnet transactions have different ApplicationLogs compared to the C# node, but the node states match. | https://github.com/nspcc-dev/neo-go/pull/3476 <br> https://github.com/neo-project/neo/pull/3290 <br> https://github.com/nspcc-dev/neo-go/pull/3473 <br> https://github.com/neo-project/neo/pull/3290 <br> https://github.com/neo-project/neo/pull/3301 <br> https://github.com/nspcc-dev/neo-go/pull/3485 |
| `Echidna`       | No changes for now | https://github.com/nspcc-dev/neo-go/pull/3554 |


## DB compatibility

Real networks with large number of blocks require a substantial amount of time
to synchronize. When operating a number of node instances with similar
configurations you may want to save some resources by performing synchronization
on one node and then copying the DB over to other instances. In general, this
can be done and this is supported, but NeoGo has a lot of options that may
affect this:
- any differences in `ProtocolConfiguration` section make (or may make) databases
  incompatible, except for `MemPoolSize`, `P2PNotaryRequestPayloadPoolSize`,
  `SeedList`, `TimePerBlock`.
  Protocol configuration is expected to be the same on all nodes of the same
  network, so don't touch it unless you know what you're doing.
- DB types (Level/Bolt) must be the same
- `GarbageCollectionPeriod` must be the same
- `KeepOnlyLatestState` must be the same
- `RemoveUntraceableBlocks` must be the same

BotlDB is also known to be incompatible between machines with different
endianness. Nothing is known for LevelDB wrt this, so it's not recommended
to copy it this way too.
