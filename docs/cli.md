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
| MPTPoolResendThreshold | `int` | `2` | Threshold (in seconds) after which MPT hash nodes currently contained in the MPT pool considered to be stale and will be re-requested from the peers. |
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
| P2PStateExchangeExtensions | `bool` | `false` | Enables following P2P MPT state data exchange logic: <br>• `StateSyncInterval` protocol setting <br>• `MPTPoolResendThreshold` application setting <br>• P2P commands `GetMPTDataCMD` and `MPTDataCMD` | Not supported by the C# node, thus may affect heterogeneous networks functionality. |
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
