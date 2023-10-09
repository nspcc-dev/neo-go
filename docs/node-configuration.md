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
| Oracle | [Oracle Configuration](#Oracle-Configuration) | | Oracle module configuration. See the [Oracle Configuration](#Oracle-Configuration) section for details. |
| P2P | [P2P Configuration](#P2P-Configuration) | | Configuration values for P2P network interaction. See the [P2P Configuration](#P2P-Configuration) section for details. |
| P2PNotary | [P2P Notary Configuration](#P2P-Notary-Configuration) | | P2P Notary module configuration. See the [P2P Notary Configuration](#P2P-Notary-Configuration) section for details. |
| Pprof | [Metrics Services Configuration](#Metrics-Services-Configuration) | | Configuration for pprof service (profiling statistics gathering). See the [Metrics Services Configuration](#Metrics-Services-Configuration) section for details. |
| Prometheus | [Metrics Services Configuration](#Metrics-Services-Configuration) | | Configuration for Prometheus (monitoring system). See the [Metrics Services Configuration](#Metrics-Services-Configuration) section for details |
| Relay | `bool` | `true` | Determines whether the server is forwarding its inventory. |
| Consensus | [Consensus Configuration](#Consensus-Configuration) |  | Describes consensus (dBFT) configuration. See the [Consensus Configuration](#Consensus-Configuration) for details. |
| RemoveUntraceableBlocks | `bool`| `false` | Denotes whether old blocks should be removed from cache and database. If enabled, then only the last `MaxTraceableBlocks` are stored and accessible to smart contracts. Old MPT data is also deleted in accordance with `GarbageCollectionPeriod` setting. If enabled along with `P2PStateExchangeExtensions` protocol extension, then old blocks and MPT states will be removed up to the second latest state synchronisation point (see `StateSyncInterval`). |
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
  to `TimePerBlock` seconds by default and is relevant only if `SessionEnabled`
  is set to `true`.
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
| GarbageCollectionPeriod | `uint32` | 10000 | Controls MPT garbage collection interval (in blocks) for configurations with `RemoveUntraceableBlocks` enabled and `KeepOnlyLatestState` disabled. In this mode the node stores a number of MPT trees (corresponding to `MaxTraceableBlocks` and `StateSyncInterval`), but the DB needs to be clean from old entries from time to time. Doing it too often will cause too much processing overhead, doing it too rarely will leave more useless data in the DB. This setting is deprecated in favor of the same setting in the ApplicationConfiguration and will be removed in future node versions. If both settings are used, ApplicationConfiguration is prioritized over this one. |
| Hardforks | `map[string]uint32` | [] | The set of incompatible changes that affect node behaviour starting from the specified height. The default value is an empty set which should be interpreted as "each known hard-fork is applied from the zero blockchain height". The list of valid hard-fork names:<br>• `Aspidochelone` represents hard-fork introduced in [#2469](https://github.com/nspcc-dev/neo-go/pull/2469) (ported from the [reference](https://github.com/neo-project/neo/pull/2712)). It adjusts the prices of `System.Contract.CreateStandardAccount` and `System.Contract.CreateMultisigAccount` interops so that the resulting prices are in accordance with `sha256` method of native `CryptoLib` contract. It also includes [#2519](https://github.com/nspcc-dev/neo-go/pull/2519) (ported from the [reference](https://github.com/neo-project/neo/pull/2749)) that adjusts the price of `System.Runtime.GetRandom` interop and fixes its vulnerability. A special NeoGo-specific change is included as well for ContractManagement's update/deploy call flags behaviour to be compatible with pre-0.99.0 behaviour that was changed because of the [3.2.0 protocol change](https://github.com/neo-project/neo/pull/2653).<br>• `Basilisk` represents hard-fork introduced in [#3056](https://github.com/nspcc-dev/neo-go/pull/3056) (ported from the [reference](https://github.com/neo-project/neo/pull/2881)). It enables strict smart contract script check against a set of JMP instructions and against method boundaries enabled on contract deploy or update. It also includes [#3080](https://github.com/nspcc-dev/neo-go/pull/3080) (ported from the [reference](https://github.com/neo-project/neo/pull/2883)) that increases `stackitem.Integer` JSON parsing precision up to the maximum value supported by the NeoVM. It also includes [#3085](https://github.com/nspcc-dev/neo-go/pull/3085) (ported from the [reference](https://github.com/neo-project/neo/pull/2810)) that enables strict check for notifications emitted by a contract to precisely match the events specified in the contract manifest. |
| KeepOnlyLatestState | `bool` | `false` | Specifies if MPT should only store the latest state (or a set of latest states, see `P2PStateExcangeExtensions` section for details). If true, DB size will be smaller, but older roots won't be accessible. This value should remain the same for the same database. | This setting is deprecated in favor of the same setting in the ApplicationConfiguration and will be removed in future node versions. If both settings are used, setting any of them to true enables the function. |
| Magic | `uint32` | `0` | Magic number which uniquely identifies Neo network. |
| MaxBlockSize | `uint32` | `262144` | Maximum block size in bytes. |
| MaxBlockSystemFee | `int64` | `900000000000` | Maximum overall transactions system fee per block. |
| MaxTraceableBlocks | `uint32` | `2102400` | Length of the chain accessible to smart contracts. | `RemoveUntraceableBlocks` should be enabled to use this setting. |
| MaxTransactionsPerBlock | `uint16` | `512` | Maximum number of transactions per block. |
| MaxValidUntilBlockIncrement | `uint32` | `5760` | Upper height increment limit for transaction's ValidUntilBlock field value relative to the current blockchain height, exceeding which a transaction will fail validation. It is set to estimated daily number of blocks with 15s interval by default. |
| MemPoolSize | `int` | `50000` | Size of the node's memory pool where transactions are stored before they are added to block. |
| NativeActivations | `map[string][]uint32` | ContractManagement: [0]<br>StdLib: [0]<br>CryptoLib: [0]<br>LedgerContract: [0]<br>NeoToken: [0]<br>GasToken: [0]<br>PolicyContract: [0]<br>RoleManagement: [0]<br>OracleContract: [0] | The list of histories of native contracts updates. Each list item shod be presented as a known native contract name with the corresponding list of chain's heights. The contract is not active until chain reaches the first height value specified in the list. | `Notary` is supported. |
| P2PNotaryRequestPayloadPoolSize | `int` | `1000` | Size of the node's P2P Notary request payloads memory pool where P2P Notary requests are stored before main or fallback transaction is completed and added to the chain.<br>This option is valid only if `P2PSigExtensions` are enabled. | Not supported by the C# node, thus may affect heterogeneous networks functionality. |
| P2PSigExtensions | `bool` | `false` | Enables following additional Notary service related logic:<br>• Transaction attribute `NotaryAssisted`<br>• Network payload of the `P2PNotaryRequest` type<br>• Native `Notary` contract<br>• Notary node module | Not supported by the C# node, thus may affect heterogeneous networks functionality. |
| P2PStateExchangeExtensions | `bool` | `false` | Enables the following P2P MPT state data exchange logic: <br>• `StateSyncInterval` protocol setting <br>• P2P commands `GetMPTDataCMD` and `MPTDataCMD` | Not supported by the C# node, thus may affect heterogeneous networks functionality. Can be supported either on MPT-complete node (`KeepOnlyLatestState`=`false`) or on light GC-enabled node (`RemoveUntraceableBlocks=true`) in which case `KeepOnlyLatestState` setting doesn't change the behavior, an appropriate set of MPTs is always stored (see `RemoveUntraceableBlocks`). |
| RemoveUntraceableBlocks | `bool`| `false` | Denotes whether old blocks should be removed from cache and database. If enabled, then only the last `MaxTraceableBlocks` are stored and accessible to smart contracts. Old MPT data is also deleted in accordance with `GarbageCollectionPeriod` setting. If enabled along with `P2PStateExchangeExtensions`, then old blocks and MPT states will be removed up to the second latest state synchronisation point (see `StateSyncInterval`). | This setting is deprecated in favor of the same setting in the ApplicationConfiguration and will be removed in future node versions. If both settings are used, setting any of them to true enables the function. |
| ReservedAttributes | `bool` | `false` | Allows to have reserved attributes range for experimental or private purposes. |
| SaveStorageBatch | `bool` | `false` | Enables storage batch saving before every persist. It is similar to StorageDump plugin for C# node. | This setting is deprecated in favor of the same setting in the ApplicationConfiguration and will be removed in future node versions. If both settings are used, setting any of them to true enables the function. |
| SeedList | `[]string` | [] | List of initial nodes addresses used to establish connectivity. |
| StandbyCommittee | `[]string` | [] | List of public keys of standby committee validators are chosen from. |
| StateRootInHeader | `bool` | `false` | Enables storing state root in block header. | Experimental protocol extension! |
| StateSyncInterval | `int` | `40000` | The number of blocks between state heights available for MPT state data synchronization. | `P2PStateExchangeExtensions` should be enabled to use this setting. |
| TimePerBlock | `Duration` | `15s` | Minimal (and targeted for) time interval between blocks. Must be an integer number of milliseconds. |
| ValidatorsCount | `uint32` | `0` | Number of validators set for the whole network lifetime, can't be set if `ValidatorsHistory` setting is used. |
| ValidatorsHistory | map[uint32]uint32 | none | Number of consensus nodes to use after given height (see `CommitteeHistory` also). Heights where the change occurs must be divisible by the number of committee members at that height. Can't be used with `ValidatorsCount` not equal to zero. |
| VerifyBlocks | `bool` | `false` | This setting is deprecated and no longer works, please use `SkipBlockVerification` in the `ApplicationConfiguration`, it will be removed in future node versions. |
| VerifyTransactions | `bool` | `false` | Denotes whether to verify transactions in the received blocks. |
