# NeoGo CLI interface

NeoGo CLI provides all functionality from one binary, so it's used to run
node, create/compile/deploy/invoke/debug smart contracts, run vm and operate
with the wallet. The standard setup assumes that you're running a node as a
separate process and it doesn't provide any CLI of its own, instead it just
makes RPC interface available for you. To perform any actions you invoke NeoGo
as a client that connects to this RPC node and does things you want it to do
(like transferring some NEP-17 asset).

All CLI commands have corresponding help messages, use `--help` option to get
them, for example:
```
./bin/neo-go db --help
```

## Running node

Use `node` command to run a NeoGo node, it will be configured using a YAML
file that contains network parameters as well as node settings.

### Configuration

All config files are located in `./config` and they are differentiated according to the network type:
- `protocol.mainnet.yml` belongs to `--mainnet` network mode (`-m` short option)
- `protocol.privnet.yml` belongs to `--privnet` network mode (`-p` short
  option) and is used by default
- `protocol.testnet.yml` belongs to `--testnet` network mode (`-t` short option)
- `protocol.unit_testnet.yml` is used by unit tests

If you want to use some non-default configuration directory path, specify
`--config-path` flag:

`./bin/neo-go node --config-path /user/yourConfigPath`

The file loaded is chosen automatically depending on network mode flag.

Each config file contains two sections. `ApplicationConfiguration` describes node-related
settings and `ProtocolConfiguration` contains protocol-related settings. See the
[Application Configuration](#Application-Configuration) and
[Protocol Configuration](#Protocol-Configuration) sections for details on configurable
values.

#### Application Configuration

ApplicationConfiguration section of `yaml` node configuration file contains
node-related settings described in the table below.

| Section | Type | Default value | Description |
| --- | --- | --- | --- |
| Address | `string` | `127.0.0.1` | Node address that P2P protocol handler binds to. |
| AnnouncedPort | `uint16` | Same as the `NodePort` | Node port which should be used to announce node's port on P2P layer, can differ from `NodePort` node is bound to (for example, if your node is behind NAT). |
| AttemptConnPeers | `int` | `20` |  Number of connection to try to establish when the connection count drops below the `MinPeers` value.|
| DBConfiguration | [DB Configuration](#DB-Configuration) |  | Describes configuration for database. See the [DB Configuration](#DB-Configuration) section for details. |
| DialTimeout | `int64` | `0` | Maximum duration a single dial may take in seconds. |
| ExtensiblePoolSize | `int` | `20` | Maximum amount of the extensible payloads from a single sender stored in a local pool. |
| LogPath | `string` | "", so only console logging | File path where to store node logs. |
| MaxPeers | `int` | `100` | Maximum numbers of peers that can be connected to the server. |
| MinPeers | `int` | `5` | Minimum number of peers for normal operation, when the node has less than this number of peers it tries to connect with some new ones. |
| NodePort | `uint16` | `0`, which is any free port | The actual node port it is bound to. |
| Oracle | [Oracle Configuration](#Oracle-Configuration) | | Oracle module configuration. See the [Oracle Configuration](#Oracle-Configuration) section for details. |
| P2PNotary | [P2P Notary Configuration](#P2P-Notary-Configuration) | | P2P Notary module configuration. See the [P2P Notary Configuration](#P2P-Notary-Configuration) section for details. |
| PingInterval | `int64` | `30` | Interval in seconds used in pinging mechanism for syncing blocks. |
| PingTimeout | `int64` | `90` | Time to wait for pong (response for sent ping request). |
| Pprof | [Metrics Services Configuration](#Metrics-Services-Configuration) | | Configuration for pprof service (profiling statistics gathering). See the [Metrics Services Configuration](#Metrics-Services-Configuration) section for details. |
| Prometheus | [Metrics Services Configuration](#Metrics-Services-Configuration) | | Configuration for Prometheus (monitoring system). See the [Metrics Services Configuration](#Metrics-Services-Configuration) section for details |
| ProtoTickInterval | `int64` | `5` | Duration in seconds between protocol ticks with each connected peer. |
| Relay | `bool` | `true` | Determines whether the server is forwarding its inventory. |
| RPC | [RPC Configuration](#RPC-Configuration) |  | Describes [RPC subsystem](rpc.md) configuration. See the [RPC Configuration](#RPC-Configuration) for details. |
| StateRoot | [State Root Configuration](#State-Root-Configuration) |  | State root module configuration. See the [State Root Configuration](#State-Root-Configuration) section for details. |
| UnlockWallet | [Unlock Wallet Configuration](#Unlock-Wallet-Configuration) |  | Node wallet configuration used for consensus (dBFT) operation. See the [Unlock Wallet Configuration](#Unlock-Wallet-Configuration) section for details. |

##### DB Configuration

`DBConfiguration` section describes configuration for node database and has
the following format:
```
DBConfiguration:
  Type: leveldb
  LevelDBOptions:
    DataDirectoryPath: /chains/privnet
  RedisDBOptions:
    Addr: localhost:6379
    Password: ""
    DB: 0
  BoltDBOptions:
    FilePath: ./chains/privnet.bolt
  BadgerDBOptions:
    BadgerDir: ./chains/privnet.badger
```
where:
- `Type` is the database type (string value). Supported types: `levelDB`,
  `redisDB`, `boltDB`, `badgerDB`.
- `LevelDBOptions` are settings for LevelDB.
- `RedisDBOptions` are options for RedisDB.
- `BoltDBOptions` configures BoltDB.
- `BadgerDBOptions` are options for BadgerDB.

Only options for the specified database type will be used.

##### Oracle Configuration

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

##### P2P Notary Configuration

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

##### Metrics Services Configuration

Metrics services configuration describes options for metrics services (pprof,
Prometheus) and has the following structure:
```
Pprof:
  Enabled: false
  Address: ""
  Port: "30001"
Prometheus:
  Enabled: false
  Address: ""
  Port: "40001"
```
where:
- `Enabled` denotes whether the service is enabled.
- `Address` is a service address to be running at.
- `Port` is a service port to be bound to.

##### RPC Configuration

`RPC` configuration section describes settings for the RPC server and has
the following structure:
```
RPC:
  Enabled: true
  Address: ""
  EnableCORSWorkaround: false
  MaxGasInvoke: 50
  Port: 10332
  TLSConfig:
    Address: ""
    CertFile: serv.crt
    Enabled: true
    Port: 10331
    KeyFile: serv.key
```
where:
- `Enabled` denotes whether RPC server should be started.
- `Address` is an RPC server address to be running at.
- `EnableCORSWorkaround` enables Cross-Origin Resource Sharing and is useful if 
  you're accessing RPC interface from the browser.
- `MaxGasInvoke` is the maximum GAS allowed to spend during `invokefunction` and
  `invokescript` RPC-calls.
- `Port` is an RPC server port it should be bound to.
- `TLS` section configures TLS protocol.

##### State Root Configuration

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

##### Unlock Wallet Configuration

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

#### Protocol configuration

ProtocolConfiguration section of `yaml` node configuration file contains
protocol-related settings described in the table below.

| Section | Type | Default value | Description | Notes |
| --- | --- | --- | --- | --- |
| KeepOnlyLatestState | `bool` | `false` | Specifies if MPT should only store latest state. If true, DB size will be smaller, but older roots won't be accessible. This value should remain the same for the same database. |
| Magic | `uint32` | `0` | Magic number which uniquely identifies NEO network. |
| MaxBlockSize | `uint32` | `262144` | Maximum block size in bytes. |
| MaxBlockSystemFee | `int64` | `900000000000` | Maximum overall transactions system fee per block. |
| MaxTraceableBlocks | `uint32` | `2102400` |  Length of the chain accessible to smart contracts. | `RemoveUntraceableBlocks` should be enabled to use this setting. |
| MaxTransactionsPerBlock | `uint16` | `512` | Maximum number of transactions per block. |
| MemPoolSize | `int` | `50000` | Size of the node's memory pool where transactions are stored before they are added to block. |
| NativeActivations | `map[string][]uint32` | ContractManagement: [0]<br>StdLib: [0]<br>CryptoLib: [0]<br>LedgerContract: [0]<br>NeoToken: [0]<br>GasToken: [0]<br>PolicyContract: [0]<br>RoleManagement: [0]<br>OracleContract: [0] | The list of histories of native contracts updates. Each list item shod be presented as a known native contract name with the corresponding list of chain's heights. The contract is not active until chain reaches the first height value specified in the list. | `Notary` is supported. |
| P2PNotaryRequestPayloadPoolSize | `int` | `1000` | Size of the node's P2P Notary request payloads memory pool where P2P Notary requests are stored before main or fallback transaction is completed and added to the chain.<br>This option is valid only if `P2PSigExtensions` are enabled. | Not supported by the C# node, thus may affect heterogeneous networks functionality. |
| P2PSigExtensions | `bool` | `false` | Enables following additional Notary service related logic:<br>• Transaction attributes `NotValidBefore`, `Conflicts` and `NotaryAssisted`<br>• Network payload of the `P2PNotaryRequest` type<br>• Native `Notary` contract<br>• Notary node module | Not supported by the C# node, thus may affect heterogeneous networks functionality. |
| P2PStateExchangeExtensions | `bool` | `false` | Enables following P2P MPT state data exchange logic: <br>• `StateSyncInterval` protocol setting <br>• P2P commands `GetMPTDataCMD` and `MPTDataCMD` | Not supported by the C# node, thus may affect heterogeneous networks functionality. |
| RemoveUntraceableBlocks | `bool`| `false` | Denotes whether old blocks should be removed from cache and database. If enabled, then only last `MaxTraceableBlocks` are stored and accessible to smart contracts. |
| ReservedAttributes | `bool` | `false` | Allows to have reserved attributes range for experimental or private purposes. |
| SaveStorageBatch | `bool` | `false` | Enables storage batch saving before every persist. It is similar to StorageDump plugin for C# node. |
| SecondsPerBlock | `int` | `15` | Minimal time that should pass before next block is accepted. |
| SeedList | `[]string` | [] | List of initial nodes addresses used to establish connectivity. |
| StandbyCommittee | `[]string` | [] | List of public keys of standby committee validators are chosen from. |
| StateRootInHeader | `bool` | `false` | Enables storing state root in block header. | Experimental protocol extension! |
| StateSyncInterval | `int` | `40000` | The number of blocks between state heights available for MPT state data synchronization. | `P2PStateExchangeExtensions` should be enabled to use this setting.  |
| ValidatorsCount | `int` | `0` | Number of validators. |
| VerifyBlocks | `bool` | `false` | Denotes whether to verify received blocks. |
| VerifyTransactions | `bool` | `false` | Denotes whether to verify transactions in received blocks. |

### Starting a node

To start Neo node on private network use:

```
./bin/neo-go node
```

Or specify a different network with appropriate flag like this:

```
./bin/neo-go node --mainnet
```

By default the node will run in foreground using current standard output for
logging.

### Restarting node services

To restart some of the node services without full node restart, send the SIGHUP 
signal. List of the services to be restarted on SIGHUP receiving:

| Service | Action |
| --- | --- |
| RPC server | Restarting with the old configuration and updated TLS certificates |

### DB import/exports

Node operates using some database as a backend to store blockchain data. NeoGo
allows to dump chain into file from the database (when node is stopped) or to
import blocks from file into the database (also when node is stopped). Use
`db` command for that.

## Smart contracts

Use `contract` command to create/compile/deploy/invoke/debug smart contracts,
see [compiler documentation](compiler.md).

## Wallet operations

`wallet` command provides interface for all operations requiring a wallet
(except contract deployment and invocations that are done via `contract
deploy` and `contract invokefunction`). Wallet management (creating wallet,
adding addresses/keys to it) is available there as well as wallet-related
functions like NEP-17 transfers, NEO votes, multi-signature signing and other
things.

### Wallet management

#### Create wallet

Use `wallet init` command to create new wallet:
```
./bin/neo-go wallet init -w wallet.nep6

{
        "version": "3.0",
        "accounts": [],
        "scrypt": {
                "n": 16384,
                "r": 8,
                "p": 8
        },
        "extra": {
                "Tokens": null
        }
 }

wallet successfully created, file location is wallet.nep6
```

where "wallet.nep6" is a wallet file name. This wallet will be empty, to
generate a new key pair and add an account for it use `-a` option:
```
./bin/neo-go wallet init -w wallet.nep6 -a
Enter the name of the account > Name
Enter passphrase > 
Confirm passphrase > 

{
        "version": "3.0",
        "accounts": [
                {
                        "address": "NMe64G6j6nkPZby26JAgpaCNrn1Ee4wW6E",
                        "key": "6PYL2UrC11nWFJWSLiqsPKCNm9u4zr4ttX1ZbV9f2fLDqXsePioVxEsYdg",
                        "label": "Name",
                        "contract": {
                                "script": "DCEDzs1j19gSDDsZTDsogN1Kr+FHXFfkDIUoctcwVhUlgUBBdHR2qg==",
                                "parameters": [
                                        {
                                                "name": "parameter0",
                                                "type": "Signature"
                                        }
                                ],
                                "deployed": false
                        },
                        "lock": false,
                        "isDefault": false
                }
        ],
        "scrypt": {
                "n": 16384,
                "r": 8,
                "p": 8
        },
        "extra": {
                "Tokens": null
        }
 }

wallet successfully created, file location is wallet.nep6
```

or use `wallet create` command to create new account in existing wallet:
```
./bin/neo-go wallet create -w wallet.nep6
Enter the name of the account > Joe Random
Enter passphrase > 
Confirm passphrase >
```

#### Convert Neo Legacy wallets to Neo N3

Use `wallet convert` to update addresses in NEP-6 wallets used with Neo
Legacy. New wallet is specified in `-o` option, it will have the same keys
with Neo N3 addresses.
```
./bin/neo-go wallet convert -w old.nep6 -o new.nep6
```

#### Check wallet contents
`wallet dump` can be used to see wallet contents in more user-friendly way,
its output is the same NEP-6 JSON, but better formatted. You can also decrypt
keys at the same time with `-d` option (you'll be prompted for password):
```
./bin/neo-go wallet dump -w wallet.nep6 -d
Enter wallet password > 

{
        "version": "3.0",
        "accounts": [
                {
                        "address": "NMe64G6j6nkPZby26JAgpaCNrn1Ee4wW6E",
                        "key": "6PYL2UrC11nWFJWSLiqsPKCNm9u4zr4ttX1ZbV9f2fLDqXsePioVxEsYdg",
                        "label": "Name",
                        "contract": {
                                "script": "DCEDzs1j19gSDDsZTDsogN1Kr+FHXFfkDIUoctcwVhUlgUBBdHR2qg==",
                                "parameters": [
                                        {
                                                "name": "parameter0",
                                                "type": "Signature"
                                        }
                                ],
                                "deployed": false
                        },
                        "lock": false,
                        "isDefault": false
                }
        ],
        "scrypt": {
                "n": 16384,
                "r": 8,
                "p": 8
        },
        "extra": {
                "Tokens": null
        }
 }
```

You can also get public keys for addresses stored in your wallet with `wallet
dump-keys` command:
```
./bin/neo-go wallet dump-keys -w wallet.nep6
NMe64G6j6nkPZby26JAgpaCNrn1Ee4wW6E (simple signature contract):
03cecd63d7d8120c3b194c3b2880dd4aafe1475c57e40c852872d7305615258140
```

#### Private key export
`wallet export` allows you to export private key in NEP-2 encrypted or WIF
(unencrypted) form (`-d` flag).
```
$ ./bin/neo-go wallet export -w wallet.nep6 -d NMe64G6j6nkPZby26JAgpaCNrn1Ee4wW6E
Enter password > 
KyswN8r48dhsvyQJVy97RWnZmKgYLrXv9mCL81Kb4vAagZiCsePv
```

#### Private key import
You can import NEP-2 or WIF private key along with verification contract (if
it's non-standard):
```
./bin/neo-go wallet import --wif KwYgW8gcxj1JWJXhPSu4Fqwzfhp5Yfi42mdYmMa4XqK7NJxXUSK7 -w wallet.nep6
Provided WIF was unencrypted. Wallet can contain only encrypted keys.
Enter the name of the account > New Account
Enter passphrase > 
Confirm passphrase >
```

#### Special accounts
Multisignature accounts can be imported with `wallet import-multisig`, you'll
need all public keys and one private key to do that. Then you could sign
transactions for this multisignature account with imported key.

`wallet import-deployed` can be used to create wallet accounts for deployed
contracts. They also can have WIF keys associated with them (in case your
contract's `verify` method needs some signature).

### Neo voting
`wallet candidate` provides commands to register or unregister a committee
(and therefore validator) candidate key:
```
./bin/neo-go wallet candidate register -a NMe64G6j6nkPZby26JAgpaCNrn1Ee4wW6E -w wallet.json -r http://localhost:20332
```

You can also vote for candidates if you own NEO:
```
./bin/neo-go wallet candidate vote -a NMe64G6j6nkPZby26JAgpaCNrn1Ee4wW6E -w wallet.json -r http://localhost:20332 -c 03cecd63d7d8120c3b194c3b2880dd4aafe1475c57e40c852872d7305615258140
```

### Getting data from chain

#### Node height/validated height
`query height` returns the latest block and validated state height:
```
$ ./bin/neo-go query height -r http://localhost:20332
Latest block: 11926
Validated state: 11926
```

#### Transaction status
`query tx` provides convenient wrapper over RPC calls to query transaction status.
```
./bin/neo-go query tx --rpc-endpoint http://localhost:20332 aaf87628851e0c03ee086ff88596bc24de87082e9e5c73d75bb1c740d1d68088
Hash:			aaf87628851e0c03ee086ff88596bc24de87082e9e5c73d75bb1c740d1d68088
OnChain:		true
BlockHash:		fabcd46e93b8f4e1bc5689e3e0cc59704320494f7a0265b91ae78b4d747ee93b
Success:		true
```
`OnChain` is true if transaction was included in block and `Success` is true
if it was executed successfully.

#### Committee members
`query commitee` returns a list of current committee members:
```
$ ./bin/neo-go query committee -r http://localhost:20332
03009b7540e10f2562e5fd8fac9eaec25166a58b26e412348ff5a86927bfac22a2
030205e9cefaea5a1dfc580af20c8d5aa2468bb0148f1a5e4605fc622c80e604ba
0207da870cedb777fceff948641021714ec815110ca111ccc7a54c168e065bda70
02147c1b1d5728e1954958daff2f88ee2fa50a06890a8a9db3fa9e972b66ae559f
0214baf0ceea3a66f17e7e1e839ea25fd8bed6cd82e6bb6e68250189065f44ff01
03184b018d6b2bc093e535519732b3fd3f7551c8cffaf4621dd5a0b89482ca66c9
0231edee3978d46c335e851c76059166eb8878516f459e085c0dd092f0f1d51c21
023e9b32ea89b94d066e649b124fd50e396ee91369e8e2a6ae1b11c170d022256d
03408dcd416396f64783ac587ea1e1593c57d9fea880c8a6a1920e92a259477806
035056669864feea401d8c31e447fb82dd29f342a9476cfd449584ce2a6165e4d7
025831cee3708e87d78211bec0d1bfee9f4c85ae784762f042e7f31c0d40c329b8
026328aae34f149853430f526ecaa9cf9c8d78a4ea82d08bdf63dd03c4d0693be6
0370c75c54445565df62cfe2e76fbec4ba00d1298867972213530cae6d418da636
03840415b0a0fcf066bcc3dc92d8349ebd33a6ab1402ef649bae00e5d9f5840828
03957af9e77282ae3263544b7b2458903624adc3f5dee303957cb6570524a5f254
02a7834be9b32e2981d157cb5bbd3acb42cfd11ea5c3b10224d7a44e98c5910f1b
02ba2c70f5996f357a43198705859fae2cfea13e1172962800772b3d588a9d4abd
03c609bea5a4825908027e4ab217e7efc06e311f19ecad9d417089f14927a173d5
02c69a8d084ee7319cfecf5161ff257aa2d1f53e79bf6c6f164cff5d94675c38b3
02cf9dc6e85d581480d91e88e8cbeaa0c153a046e89ded08b4cefd851e1d7325b5
03d84d22b8753cf225d263a3a782a4e16ca72ef323cfde04977c74f14873ab1e4c
```

#### Candidate/voting data
`query candidates` returns all current candidates, number of votes for them
and their committee/consensus status:
```
$ ./bin/neo-go query candidates -r http://localhost:20332
Key                                                                 Votes    Committee  Consensus
03009b7540e10f2562e5fd8fac9eaec25166a58b26e412348ff5a86927bfac22a2  2000000  true       true
030205e9cefaea5a1dfc580af20c8d5aa2468bb0148f1a5e4605fc622c80e604ba  2000000  true       true
0214baf0ceea3a66f17e7e1e839ea25fd8bed6cd82e6bb6e68250189065f44ff01  2000000  true       true
023e9b32ea89b94d066e649b124fd50e396ee91369e8e2a6ae1b11c170d022256d  2000000  true       true
03408dcd416396f64783ac587ea1e1593c57d9fea880c8a6a1920e92a259477806  2000000  true       true
02a7834be9b32e2981d157cb5bbd3acb42cfd11ea5c3b10224d7a44e98c5910f1b  2000000  true       true
02ba2c70f5996f357a43198705859fae2cfea13e1172962800772b3d588a9d4abd  2000000  true       true
025664cef0abcba7787ad5fb12f3af31c5cdc7a479068aa2ad8ee78804768bffe9  1000000  false      false
03650a684461a64bf46bee561d9981a4c57adc6ccbd3a9512b83701480b30218ab  1000000  false      false
026a10aa2b4d7639c5deafa4ff081467db10b5d00432749a2a5ee1d2bfed23e1c0  1000000  false      false
02d5786a9214a8a3f1757d7596fd10f5241205e2c0d68362f4766579bac6189249  1000000  false      false
033d8e35f8cd9a33852280b6d93093c7292ed5ce90d90f149fa2da50ba6168dfce  100000   false      false
0349c7ef0b4aaf181f0a3e1350c527b136cc5b42498cb83ab8880c05ed95167e1c  100000   false      false
035b4f9be2b853e06eb5a09c167e038b96b4804235961510423252f2ee3dbba583  100000   false      false
027e459b264b6f7e325ab4b0bb0fa641081fb68517fd613ebd7a94cb79d3081e4f  100000   false      false
0288cad442a877960c76b4f688f4be30f768256d9a3da2492b0180b91243918b4f  100000   false      false
02a40c552798f79636095817ec88924fc6cb7094e5a3cb059a9b3bc91ea3bf0d3d  100000   false      false
02db79e69c518ae9254e314b6f5f4b63e914cdd4b2574dc2f9236c01c1fc1d8973  100000   false      false
02ec143f00b88524caf36a0121c2de09eef0519ddbe1c710a00f0e2663201ee4c0  100000   false      false
03d8d58d2257ca6cb14522b76513d4783f7d481801695893794c2186515c6de76f  0        false      false
```

#### Voter data
`query voter` returns additional data about NEO holder: amount of NEO he has,
candidate he voted for (if any) and block number of the last transactions
involving NEO on this account:
```
$ ./bin/neo-go query voter -r http://localhost:20332 Nj91C8TxQSxW1jCE1ytFre6mg5qxTypg1Y
        Voted: 0214baf0ceea3a66f17e7e1e839ea25fd8bed6cd82e6bb6e68250189065f44ff01 (Nj91C8TxQSxW1jCE1ytFre6mg5qxTypg1Y)
        Amount : 2000000
        Block: 3970
```

### NEP-17 token functions

`wallet nep17` contains a set of commands to use for NEP-17 tokens.

#### Token metadata

NEP-17 commands are designed to work with any NEP-17 tokens, but NeoGo needs
some metadata for these tokens to function properly. Native NEO or GAS are
known to NeoGo by default, but other tokens are not. NeoGo can get this
metadata from the specified RPC server, but that's an additional request to
make, so if you care about command processing delay you can import token
metadata into the wallet with `wallet nep17 import` command. It'll be stored
in the `extra` section of the wallet.
```
./bin/neo-go wallet nep17 import -w wallet.nep6 -r http://localhost:20332 -t abcdefc189f30098b0ba6a2eb90b3a925800ffff
```

You can later see what token data you have in your wallet with `wallet nep17
info` command and remove tokens you don't need with `wallet nep17 remove`.

#### Balance
Getting balance is easy:
```
./bin/neo-go wallet nep17 balance -w /etc/neo-go/wallet.json -r http://localhost:20332
```

By default you'll get data for all tokens for the default wallet's
address. You can select non-default address with `-a` flag and/or select token
with `--token` flag (token hash or name can be used as parameter)

#### Transfers

`wallet nep17 transfer` creates a token transfer transaction and pushes it to
the RPC server (or saves to file if it needs to be signed by multiple
parties). For example, transferring 100 GAS looks like this:

```
./bin/neo-go wallet nep17 transfer -w wallet.nep6 -r http://localhost:20332 --to NjEQfanGEXihz85eTnacQuhqhNnA6LxpLp --from NMe64G6j6nkPZby26JAgpaCNrn1Ee4wW6E --token GAS --amount 100
```

You can omit `--from` parameter (default wallet's address will be used in this
case), you can add `--gas` for extra network fee (raising priority of your
transaction). And you can save transaction to file with `--out` instead of
sending it to the network if it needs to be signed by multiple parties.

One `transfer` invocation creates one transaction, but in case you need to do
many transfers you can save on network fees by doing multiple token moves with
one transaction by using `wallet nep17 multitransfer` command. It can transfer
things from one account to many, its syntax differs from `transfer` in that
you don't have `--token`, `--to` and `--amount` options, but instead you can
specify multiple "token:addr:amount" sets after all other options. The same
transfer as above can be done with `multitransfer` by doing this:
```
./bin/neo-go wallet nep17 multitransfer -w wallet.nep6 -r http://localhost:20332 --from NMe64G6j6nkPZby26JAgpaCNrn1Ee4wW6E GAS:NjEQfanGEXihz85eTnacQuhqhNnA6LxpLp:100
```

#### GAS claims

While Neo N3 doesn't have any notion of "claim transaction" and has GAS
automatically distributed with every NEO transfer for NEO owners you still
won't get GAS if you don't do any actions. So the old `wallet claim` command
was updated to be an easier way to do NEO "flipping" when you send a
transaction that transfers all of your NEO to yourself thereby triggering GAS
distribution.

## Conversion utility

NeoGo provides conversion utility command to reverse data, convert script
hashes to/from address, convert data to/from hexadecimal or base64
representation. All of this is done by a single `util convert` command like
this:
```
$ ./bin/neo-go util convert deee79c189f30098b0ba6a2eb90b3a9258a6c7ff
BE ScriptHash to Address        NgEisvCqr2h8wpRxQb7bVPWUZdbVCY8Uo6
LE ScriptHash to Address        NjEQfanGEXihz85eTnacQuhqhNnA6LxpLp
Hex to String                           "\xde\xeey\xc1\x89\xf3\x00\x98\xb0\xbaj.\xb9\v:\x92X\xa6\xc7\xff"
Hex to Integer                          -1256651697634605895065630637163547727407485218
Swap Endianness                         ffc7a658923a0bb92e6abab09800f389c179eede
Base64 to String                        "u\xe7\x9e\xef\xd75\xf3\xd7\xf7\xd3O|oF\xda魞o\xdd\x1bݯv\xe7ƺs\xb7\xdf"
Base64 to BigInteger            -222811771454869584930239486728381018152491835874567723544539443409000587
String to Hex                           64656565373963313839663330303938623062613661326562393062336139323538613663376666
String to Base64                        ZGVlZTc5YzE4OWYzMDA5OGIwYmE2YTJlYjkwYjNhOTI1OGE2YzdmZg==
```

## VM CLI
There is a VM CLI that you can use to load/analyze/run/step through some code:

```
./bin/neo-go vm
```

Some basic commands available there:

- `loadgo` -- loads smart contract `NEO-GO-VM > loadgo TestContract/main.go`
- `ops` -- show the opcodes of currently loaded contract
- `run` -- executes currently loaded contract

Use `help` command to get more detailed information on all possibilities and
particular commands. Note that this VM is completely disconnected from the
blockchain, so you won't have all interop functionality available for smart
contracts (use test invocations via RPC for that).
