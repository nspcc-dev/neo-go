# Changelog

This document outlines major changes between releases.

## 0.112.0 "Hibernation" (29 Aug 2025)

This version fixes state difference at block 8813651 of testnet caused by
improper `ABORTMSG` and `AASSERTMSG` arguments handling. It also introduces
support for a set of newly-added NEPs and ability to customize the list of
native contracts.

DB resynchronisation (or state reset to block 8813650 for full nodes) is
required on testnet nodes upgrade. `SessionExpirationTime` RPC server setting is
deprecated and replaced by `SessionLifetime` of `Duration` type, consider
upgrading the node's config. `math.Min` and `math.Max` interop utilities are
deprecated and replaced by Go built-in `min` and `max` functions, consider
upgrading affected smart contracts. Also, some deprecated functionality has been
removed according to the schedule.

New features:
 * customizable native contracts (#3966)
 * NEP-22/NEP-31 (contract update and destroy functionality) support in CLI,
   `smartcontract` package and compiler (#3968, #3971)
 * NEP-29/NEP-30 (contract `_deploy` and `verify` methods) support in
   `smartcontract` package and compiler (#3978)
 * NEP-32 support for `db restore` CLI command (unified chain dump format)
   (#3974)
 * mempool events RPC web-socket subscription (#3967)

Behavior changes:
 * `wallet candidate register` CLI command is migrated onto GAS transfer (#3973)
 * `SessionExpirationTime` RPC server setting is deprecated and replaced by
   `SessionLifetime` (#3953)
 * `math.Min` and `math.Max` interop helpers are deprecated and replaced with
   Go built-in min/max (#3984)
 * deprecated functionality removal (`GetBlockHeader` and
   `GetBlockHeaderVerbose` methods of RPC client) (#3995)
 * RPC client's `Waiter.Wait()` is extended with context parameter (#3959)

Improvements:
 * `storage.KeyValue` interop type (#3982)
 * `util convert` CLI command is extended with Base64 public key convertor
 * stackitem conversion is supported for `rolemgmt.DesignationEvent` native
   RoleManagement RPC binding (#3956)
 * `getversion` RPC response is extended with `SaveInvocations`,
   `KeepOnlyLatestState` and `RemoveUntraceableBlocks` settings (#3954)
 * Go built-in min/max support in compiler (#3984)
 * web-socket client connection errors improvement (#3975)

Bugs fixed:
 * panic on public key comparison with infinite operand (#3961)
 * missing strict UTF-8 check for `ABORTMSG` and `ASSERTMSG` arguments (#3988)

## 0.111.0 "Facilitation" (18 Jul 2025)

We've decided to release one more v3.8.0-compatible version since the current
set of changes includes adaptive block time extension allowing to vary block
generation time from `TimePerBlock` to `MaxTimePerBlock`. In addition to that,
this version includes new audit module for NeoFS block storage, an optimisation
of light node synchronisation process, enhanced version of BoltDB and more.

Some deprecated functionality has been removed according to the schedule, see
more details in the `Behaviour changes` section. Also, for those node operators
who would like to test the dynamic block time extension, new `MaxTimePerBlock`
option is added to the node's protocol configuration. Finally, configuration
changes have been introduced: starting from this version,
`RemoveUntraceableHeaders` mode is a part of `RemoveUntraceableBlocks` and
hence, should be removed from the node's configuration file in favour of
`RemoveUntraceableBlocks`. Also, `SessionExpirationTime` RPC server
configuration setting is replaced with `SessionLifetime` setting of type
`Duration`; `SkipIndexFilesSearch`, `IndexFileSize` and `IndexFileAttribute`
settings of NeoFSBlockFetcher service were dropped since index files are not
supported anymore. Please, upgrade your configuration accordingly.

New features:
 * `util audit-bin` CLI command for NeoFS block storage audit (#3943, #3945,
   #3946)
 * support of `GetBlockHeaderByIndex[Verbose]` RPC client API (#3939)
 * `MaxTimePerblock` protocol configuration setting enabling dynamic block
  acceptance interval (#3948)

Behavior changes:
 * index files support is removed from `util upload-bin` CLI command and
   NeoFSBlockFetcher service (#3938)
 * `RemoveUntraceableHeaders` application configuration setting is merged into
   `RemoveUntraceableBlocks` (#3922)
 * `GetBlockHeaderVerbose` RPC client API is deprecated and replaced by
   `GetBlockHeaderByHashVerbose` (#3939)
 * deprecated `NEP11Payable` and `NEP17Payable` aliases are removed from the
   smartcontract package in favour of `NEP26StandardName` and
   `NEP27StandardName` (#3955)
 * `SessionExpirationTime` RPC server configuration setting is deprecated and
   replaced by `SessionLifetime` setting of `Duration` type (#3953)

Improvements:
 * light node synchronisation doesn't require full headers sync anymore (#3922)
 * BoltDB dependency is upgraded to custom (and faster!) version (#3936, #3937)
 * dBFT dependency upgrade empowering dynamic block time extension (#3948)

Bugs fixed:
 * misses in blocks uploaded by `util upload-bin` CLI command (#3942)
 * panic on attempt to close local RPC client (#3947)

## 0.110.0 "Utilization" (10 Jun 2025)

This version fixes state difference at block 6701925 of testnet. It also introduces
NeoFSStateFetcher service that allows to synchronize contract storage state at
particular height based on checkpoint data stored in NeoFS. Moreover, for smooth
RPC client upgrade experience, the next Faun hardfork is added in a preview mode
(although it doesn't contain any changes yet). Also, new `BroadcastTxsBatchDelay`
P2P application configuration setting is introduced preliminary for custom networks
setup to be able to customize the maximum transaction batch broadcast delay.

T5 nodes require DB resynchronisation (or state reset to 6701924 block for full nodes),
mainnet nodes DB resync is not required at the moment of release. No configuration
changes required.

New features:
 * ability to synchronize contract storage from NeoFS snapshot (#3844)
 * Faun hardfork is introduced in a preview mode, but doesn't include
   any changes yet (#3931)

Improvements:
 * migration to NeoFS SearchV2 API (#3913)
 * support for graceful stop at given height for NeoFSBlockFetcher service (#3901)
 * support slice expressions for string in compiler (#3916)
 * integrate ArchivalNode capability (#3899)
 * refactor code to use Go built-ins (#3897, #3923)
 * update documentation (#3924)
 * `BroadcastTxsBatchDelay` application setting that allows to customize the time
   limit for transaction batch collection before its broadcast (#3930)

Bugs fixed:
 * race in cache persistence logic (#3931)
 * whitespaces in argument of StdLib's `base64Decode` and `base64UrlDecode`
   methods lead to decoding error (#3928)
 * multiple simultaneous connections to a single peer (#3911)
 * node peer state unambiguity (#3911)
 * failed MPT initialization after node restart (#3919)
 * bugs in seed discovery logic (#3912)
 * intermediate state synchronisation stages are not persisted to the DB (#3900)
 * intermediate state jump stages are not persisted to the DB (#3895)
 * excessive logging in Notary service (#3896)
 * missing block height initialization for statesync module (#3933) 

## 0.109.1 "Transformation" (14 May 2025)

An urgent patch version that fixes state difference at block 5894663 of testnet.
It also contains a couple of other bug fixes including CommitteeChanged events 
emission (that affects application execution results compatibility with C# node,
but likely not critical for anyone).

DB resynchronisation for testnet nodes is required (operators of full node may
use `neo-go db reset` command to reset the node's state to height 5894662 and
avoid full chain resynchronisation). Mainnet DB resynchronisation is not
needed.

Behavior changes:
 * `subscribe` and `unsubscribe` requests will be served sequentially by web-socket
   RPC client (#3893)

Bugs fixed:
 * CommitteeChanged event of native NeoToken is emitted irrespective of
   Cockatrice fork (#3892)
 * GasPerBlock update is applied starting from the next block (#3891)
 * panic in subscriptions module of web-socket RPC client (#3893)

## 0.109.0 "Separation" (01 May 2025)

Long-awaited version that is fully compatible with the C# node 3.8.0. It extends
and enables Echidna hardfork, introduces a set of RPC extensions and new node
configuration settings and fixes a couple of critical bugs. 

Node operators should review and update node configuration. For N3 chains Echidna
hardfork is enabled at block `7300000` of Mainnet and `5870000` block of Testnet;
NeoFS Mainnet hardforks schedule is changed, see the updated node configuration
for reference; for NeoFS Testnet Echidna is enabled starting from the genesis
block. N3 Mainnet/Testnet nodes DB resynchronization is not needed, NeoFS chains
resynchronization is required.

New features:
 * block generation time is a part of native Policy contract
   (`setMillisecondsPerBlock` and `getMillisecondsPerBlock` APIs) starting
   from Echidna HF (#3835)
 * `util upload-state` CLI command allowing to upload contract storage items
   to NeoFS (#3808, #3845)
 * `verifyWithEd25519` method support in CryptoLib native contract starting
   from Echidna HF (#3852, #3853)
 * `MaxWebSocketFeeds` node parameter allowing to configure maximum number
   of RPC server subscriptions (#3828)
 * `invokecontainedscript` RPC API allowing to invoke script in a
   customizable execution environment (#3839)
 * `LogEncoding` and `LogTimestamp` node parameters allowing to customize logs
   output format and timestamps (#3846)
 * `MaxValidUntilBlockIncrement` setting is a part of native Policy contract
   (`setMaxValidUntilBlockIncrement` and `getMaxValidUntilBlockIncrement` APIs)
   starting from Echidna HF (#3849)
 * `MaxTraceableBlocks` setting is a part of native Policy contract
   (`setMaxTraceableBlocks` and `getMaxTraceableBlocks` APIs) starting from
   Echidna HF (#3858)
 * `recoverSecp256K1` method support in CryptoLib native contract starting
   from Echidna HF (#3863)
 * `isContract` method support in ContractManagement native contract starting
   from Echidna HF (#3867)
 * `DisableCompression` node parameter allowing to disable P2P payloads
   compression (#3881)
 * `NotaryAssisted` attribute support is moved from `P2PSignatureExtensions`
   under Echidna HF (#3854)
 * Notary native contract activation is moved from `P2PSignatureExtensions`
   under Echidna HF (#3478)
 * Echidna HF is stable now (#3851)

Behavior changes:
 * restricted number of allowed contract notifications starting from Echidna
   HF (#3640)
 * NeoFS networks configuration update and forks reschedule (#3833, #3851)

Improvements:
 * RPC session iterator expansion extension (#3827)
 * optimize VM script execution in case of disabled coverage (#3855)
 * replace timers with tickers (#3861)
 * extend NotaryAssisted attribute verification logic (#3865)
 * `ErrConnClosedByUser` error designated to distinguish case when connection
   is closed by RPC client user (#3868)
 * Go 1.23 upgrade (#3847)
 * NeoFS SDK Go update to RC13+ (#3847, #3870)

Bugs fixed:
 * NPE on `getpeers` RPC request serving (#3880)
 * inability to decode genesis block header fetched from NeoFS (#3819)
 * insufficient nesting depth restriction of transaction witness condition
   (#3815)
 * improper handling of empty filter for `getblocknotifications` RPC API
   on the RPC client side (#3820)
 * native contracts can't be initialized from non-genesis block (#3837)
 * invalid signature of `base64UrlEncode` and `base64UrlDecode` (#3862)
 * NPE on NeoFS BlockFetcher service shutdown (#3870)

## 0.108.1 "Revalidation" (13 Feb 2025)

An urgent fix for a very old behavior difference with C# node in Rules witness
condition parsing. It suddenly affected testnet compatibility at block 5450030
and made the chain unprocessable by NeoGo. Please upgrade to fix it, DB is
compatible, no resynchronization required.

Bugs fixed:
 * incorrect rule depth limit for Rules witness conditions (#3810)

## 0.108.0 "Participation" (11 Feb 2025)

This version is compatible with the C# node 3.7.6, but also contains some
Echidna changes preview. Additional RPC extensions are introduced with this
release as well as some important fixes and improvements.

We recommend to check your configurations wrt NeoFS synchronization options,
a new set of containers was introduced recently, old ones will eventually be
deleted (which won't break syncrhonization, but can delay it a bit). No DB
resync is required unless you want to use the new "SaveInvocations" option.

New features:
 * "Designation" event in RoleManagement native contract starting from Echidna
   HF (#3761)
 * base64Url encoding and decoding support in StdLib native contract starting
   from Echidna HF (#3761)
 * NEO candidate registration via NEP-27 payment starting from Echidna HF (#3700)
 * ArchivalNode P2P capability (#3778)
 * NEP-26 and NEP-27 support everywhere (#3792)
 * "SaveInvocations" node parameter that allows to store more detailed
   contract invocation data and retrieve it via RPC (#3569)
 * "getblocknotifications" RPC API allowing to fetch filtered notifications
   from all block execution contexts (#3805)

Behavior changes:
 * updated "upload-bin" command defaults (#3760)
 * updated NeoFS containers for all networks (#3759)
 * additional AllowNotify call flag for some NEO methods starting from Echidna
   HF (#3761)
 * Dump*Slot methods removed from vm.Context (#3806)

Improvements:
 * golang.org/x/crypto update from 0.26.0 to 0.31.0 (#3765)
 * neotest can load contracts from NEF/manifest files now (#3771)
 * more accurate memory management in persisting/processing cycles preventing
   OOM conditions in most cases (#3787)
 * RPC bindings now have ToStackItem and ToSCParameter methods for structures
   (#3794, #3796, #3804)
 * dBFT 0.3.2 with improved timers (#3799)
 * github.com/consensys/gnark update from 0.11.0 to 0.12.0 (#3800)
 * ability to fetch headers from NeoFS (#3789)

Bugs fixed:
 * potentially incorrect handling of misconfigured NeoFS endpoints (#3758)
 * duplicate index objects are no longer an error for "upload-bin" (#3763)
 * old transfer data removal problem in RemoveUntraceableBlocks configuration,
   0.107.2 regression (#3787)
 * zkpbinding module producing code that can't be compiled, 0.107.0 regression
   (#3802)

## 0.107.2 "Obliteration" (13 Dec 2024)

One more compatible patch-release that introduces `RemoveUntraceableHeaders`
application-level extension allowing to remove untraceable block headers from the DB.
This feature significantly reduces the database size, but for now it is supported in
an experimental mode, use it with care. Other than that, this release includes a fix
of BlockFetcher service that may hang on retry of NeoFS requests preventing the node
from syncing. Also, an improved algorithm of blocks uploading and extended list of
block and index file attributes are supported for `upload-bin` CLI command.

No configuration update or DB resync is required. However, starting from this release
`NeoFSBlockFetcher` application configuration section is backed by default values for
every parameter except `Addresses` and `ContainerID`, hence if you don't like too
chatty configuration files, feel free to remove all optional parameters.

New features:
 * untraceable headers removal (#3750)

Behavior changes:
 * add `BlockTime` attribute to block objects stored in NeoFS block storage (#3749)
 * use `Timestamp` attribute to hold object creation time for block and index objects
   stored in NeoFS block storage (#3749)
 * extended debug logs for `upload-bin` CLI command (#3751)

Improvements:
 * embed default UnitTestNet node configuration (#3696)
 * NeoFS SDK dependency upgrade (#3725, #3756)
 * dependent packages updates (#3746, #3747, #3748)
 * refactor and speed up `upload-bin` CLI command (#3735)
 * backup NeoFS BlockFetcher configuration with default values (#3742)
 * reuse more of built-in NeoFS SDK functionality in `upload-bin` CLI handler (#3749)

Bugs fixed:
 * NeoFS BlockFetcher sometimes is hanging during the node startup (#3736)
 * RPC server timers are improperly drained (#3737)
 * basic unit test chain restore configuration (#3696)

## 0.107.1 "Narrativization" (06 Dec 2024)

An urgent version that fixes the problem of intensive CPU usage caused by improper
NeoFS BlockFetcher shutdown on the node's sync process completion and magnified by
additional bug at the peer discovery level.

No configuration changes or DB resync is required. It is highly recommended to update
from 0.107.0 as soon as possible since described problems affect the speed of blocks
processing and the overall node functionality.

Behavior changes:
 * explicitly enable the list of stable hardforks in default NeoFS testnet
   configuration (#3722)

Improvements:
 * decrease NeoFS storage nodes deal timeout for NeoFS BlockFetcher (#3723)
 * adjust optimal number of peers for networks with small peer count (#3727)
 * don't enable unstable hardforks by default (#3724)

Bugs fixed:
 * "unexpected empty payload: CMDVersion" error on peer disconnection (#3726)
 * NeoFS BlockFetcher shutdown (#3728)
 * connected peers count is not respected on attempt to gather more node addresses
   (#3730)

## 0.107.0 "Mongrelization" (03 Dec 2024)

A large update that introduces a major node extension: NeoFS BlockFetcher service and
`util upload-bin` CLI command implemented as a part of [NeoFS snapshot storage
proposal](https://github.com/neo-project/neo/issues/3463). BlockFetcher service, as
an alternative to P2P synchronization mechanism, allows to download blocks and sync
chain from block dumps stored in NeoFS. Starting from this release, NeoSPCC team
maintains chain dumps in NeoFS for N3 and NeoFS public networks. The default NeoGo
node configuration for these networks has been changed to use NeoFS BlockFetcher as
a synchronization mechanism prior to P2P synchronization. Other than that, this
release includes NEP-24 standard support at both compiler and SC bindings generator
levels. A large number of tiny user-facing enhancements is rolled out for RPC
actor/ivoker, `neotest` and `unwrap` packages as far as for CLI utilities. Also,
this release contains a set of NeoGo VM bug fixes inspired by differential VM fuzzing
study conducted by our external contributor @Slava0135 and a set of tiny VM CLI
enhancements introduced by @ixje.

Some deprecated functionality has been removed according to the schedule, see more
details in the `Behaviour changes` section. Also, for those node operators who would
like to check out our new NeoFS BlockFetcher node extension, we'd recommend to add
corresponding section to the node's configuration (see the default node configuration
for N3/NeoFS networks for example).

This release is fully compatible with 3.7.5 version of C# node, no DB resync is
needed.

New features:
 * new NeoFS BlockFetcher service that allows to sync node from NeoFS chain dump
   (#3515, #3636, #3637, #3632, #3691, #3706, #3668, #3713)
 * new `util upload-bin` CLI command that allows to upload blocks from Neo chains to
   NeoFS (#3578, #3625, #3626, #3633, #3637, #3638, #3643, #3650, #3662, #3684,
   #3686, #3691)
 * new `delete` and `ib` VM CLI commands for brealpoints management (#3674)
 * NEP-24 standard support (#3560)
 * Echidna hardfork introduced, but not yet enabled (please, note that this is only
   a preview that includes a part of scheduled changes and will be changed in an
   incompatible way before the full support, hence, this hardfork may be enabled for
   experimental purpose only) (#3554)

Behavior changes:
 * neotest's AddSystemFee and TestInvoke are bound to Executor state (#3551)
 * getversion RPC response is extended with seed list and standby committee (#3540)
 * support only two latest versions of Go instead of three (#3567)
 * some deprecated functionality is dropped: unmarshalling code for the old
   `getpeers` RPC response, `NEOBalance` stackitem deserializer compatibility code,
   `serv_node_version` Prometheus gauge metric, outdated RPC error codes support,
   block-based web-socket transaction awaiting (#3690)

Improvements:
 * documentation updates (#3545, #3663, #3666, #3678, #3683, #3708)
 * netmode package is extended with public NeoFS chain IDs (#3539)
 * dBFT library upgrades (#3541, #3711)
 * migration to Docker Compose V2 (#3547)
 * system fee required for contract deployment in neotest is precisely calculated
   (#3551)
 * ability to customize awaiting options for PollingBased RPC waiter (#3556)
 * Go 1.23 support, bump minimum required Go version up to Go 1.22 (#3567)
 * a set of dependent libraries upgrades (#3570)
 * extend the list of supported SC parameters for RPC actor/invoker (#3583)
 * Null stackitem result handling for autogenerated RPC bindings (#3584)
 * macos-12 support is removed, macos-14 support is added (#3657)
 * allow to get NEP-11/NEP-17 balances via CLI using account address only (#3659)
 * expose VM slot getters (#3677)
 * add `unwrap.ErrNull` error to handle Null stackitem returned (#3695)
 * extend web-socket notification subsystem with notification parameter filters
   (#3689)
 * explicitly prohibit unknown configuration fields for `contract generate-*` CLI
   commands (#3708)
 * extend compiled smart contract identifier in neotest cache (#3709)
 * move `neogo_version` metric out of `network` package (#3712)

Bugs fixed:
 * fees of ditched transaction are not cleared from mempool (#3537)
 * extension of SC permission descriptor doesn't clear wildcard status (#3544)
 * example of NeoGo VM script fails VM execution (#3593)
 * a call to `getunclaimedGas` of native Neo contract for account with zero balance
   results in VM failure (#3589)
 * `MODMUL` VM opcode handler returns wrong results for negative arguments (#3599)
 * neotest coverage extension panics on attempt to collect coverage for contract with
   missing debug information (#3600)
 * `MODPOW`VM opcode handler returns wrong results for negative base (#3649)
 * node panic on SIGHUP (#3661)
 * HTTP return code of RPC requests diverges from C# RPC server's behaviour (#3665)
 * a set of bugs in unit tests (#3442, #3680)
 * `POPITEM` VM opcode handler counts stack references improperly (#3688)
 * `PACKMAP` VM opcode handler ignores duplicating map keys (#3685)
 * chain restore from genesis block fails with StateRootInHeader extension enabled
   (#3697)
 * hardfork-dependent methods are not included into native contract metadata starting
   from the hardfork height (#3704)
 * outdated keyword usage in Dockerfiles (#3710)

## 0.106.3 "Lyophilization" (29 Jul 2024)

This 3.7.5-compatible version includes a number of important fixes, so please
upgrade your nodes. Some minor extensions were also added.

Resynchronization (or state reset) is required for testnet (because of a bug
leading to state difference since 4368840), but not required for mainnet.

New features:
 * embedded mainnet/testnet/NeoFS node configuration files (#3477, #3504)

Behavior changes:
 * CLI no longer panics if error occurs (#3495)
 * MaxTraceableBlocks is 17280 now for NeoFS networks (#3518)
 * minimal default RPC `SessionExpirationTime` is 5s now (#3529)

Improvements:
 * RPC actor interface extension with WaitSuccess method (#3491)
 * Signers() API for RPC invokers (#3492)
 * SignerAccounts() API for RPC actors (#3492)
 * getpeers RPC extension with the user agent and last known block height data
   (#3481)
 * OnExecHook() API for VM (#3460)
 * more details in witness verification error message (#3508)
 * CLI help and error string format unification (#3495, #3520)
 * CLI library (github.com/urfave/cli) upgrade to 2.27.2 (from v1 API, #3495)
 * microoptimization of extensible sender list calculation (#3500)
 * microoptimization of chain dump code (#3514)
 * documentation and error messages (#3526, #3527)

Bugs fixed:
 * RPC `SessionExpirationTime` could be zero in some configurations (#3529)
 * panic in WSClient unsubscription code in some multithreaded cases (#3532)
 * missing PrimaryIndex in Ledger's getBlock() result (#3534)
 * contract manifests with null groups were accepted (#3523)
 * contract manifests with invalid features were accepted (#3523)
 * contract manifests with null trusts were accepted (#3523)
 * WSClient deadlock in some disconnection cases (#3535)

## 0.106.2 "Keratinization" (13 Jun 2024)

A 3.7.5-compatible version introducing new Domovoi hardfork that brings two fixes to
the protocol: using executing contract state to check contract call permissions
(included into this NeoGo release) and proper VM items refcounting for
System.Runtime.GetNotifications handler (not included into this NeoGo release because
we've never had this bug). Since the second bug is C#-specific and does not lead to
the state differences in mainnet/testnet, we've decided not to break the NeoGo node
to follow pre-Domovoi C# node implementation. Thus, differences in application logs
for several T5 transactions before Domovoi hardfork are expected and won't be fixed.

Please, ensure your node configuration includes the Domovoi hardfork. No DB
resynchronisation is required.

New features:
 * Domovoi hardfork scheduled for 5570000 block of mainnet and 4144000 block of T5
   testnet (#3476, #3473, #3486, #3487)

Behavior changes:
 * hide node logs timestamp if the node is running not in TTY (#3468)
 * distinguish log level for various node peer disconnection reasons (#3469)

Improvements:
 * ensure NeoFS nodes are configured when processing NeoFS oracle requests (#3455)
 * NeoFS SDK dependency upgrade (#3483)
 * ensure System.Runtime.GetNotifications handler can't break the MaxStackSize
   constraint before and after Domovoi hardfork (#3485)

Bugs fixed:
 * deployed contract script is included into wallet's account (#3470)
 * updated contract state is used to verify contract call permissions before the
   Domovoi hardfork (#3473)

## 0.106.1 "Implication" (3 Jun 2024)

An urgent release that fixes mainnet state difference at block 5462944 which halts
blocks processing starting from the height 5468658. This release requires full node
DB resynchronization for mainnet nodes. T5 testnet DB state is not affected by this
bug (at least up to the current 4087361 height). Thus, DB resynchronisation may be
skipped for testnet nodes. No configuration changes implied.

Bugs fixed:
 * mainnet state difference at block 5462944 caused by runtime notification
   permissions check against the updated contract state instead of executing state
   (#3472)
 * unused neofs-contract dependency in the node modules (#3458)

## 0.106.0 "Zephyranthes" (21 May 2024)

We're rolling out a large set of updates including all of Neo 3.7.4 protocol changes:
native contracts update functionality and extended native contract APIs united under
the upcoming Cockatrice hardfork. For smart contract developers an ability to
generate smart contract bindings with dynamic hash is supported as well as a number
of useful extensions for `unwrap` package, compatible NNS smart contract RPC binding
and a lot of other handy enhancements. This release also includes a couple of severe
regression bug fixes affecting the node state. As a bonus of fixing a wide range of
failing tests, we've refactored the node services start and shutdown procedures to
make it more stable. This version drops support for Go 1.19 and requires 1.20+ to
build (with Go 1.22 supported).

Notice that this release requires full node resynchronization for both mainnet
and testnet nodes. Please pay a special attention to the Cockatrice hardfork schedule
and check your configurations. It should be configured for 3967000 of T5 testnet and
5450000 of mainnet. To ensure compatibility your node must be configured
appropriately.

New features:
 * native contracts update functionality bound to hardforks (#3402, #3444)
 * Cockatrice hardfork planned for 5450000 block of mainnet and 3967000 block of T5
   testnet (#3402, #3448, #3402, #3446)
 * `CommitteeChanged` events are emitted by NeoToken native contract starting from
   Cockatrice hardfork (#3351, #3448)
 * `getCommitteeAddress` method of NeoToken native contract is available starting
   from Cockatrice hardfork (#3362, #3402)
 * `keccak256` method of CryptoLib native contract is available starting from
   Cockatrice hardfork (#3301, #3402)
 * dynamic contract hash support for smart contract bindings (#3405)
 * support `StringCompressed` API for `crypto.PublicKey` (#3408)
 * support `Copy` API for `transaction.Transaction` and `payload.P2PNotaryRequest`
   (#3407)
 * extend `verifyWithECDsa` method of native CryptoLib contract with Keccak256 hasher
   starting from Cockatrice and add an example of custom Koblitz-based and
   Keccak256-based transaction witness verification (#3425)
 * autogenerated `nativehashes` package (#3402, #3431)
 * introduce `unwrap.Exception` type for better RPC invocation result exceptions
   detection (#3438)

Behavior changes:
 * Neo Name Service smart contract RPC binding follows the official N3 implementation
   (#3291)
 * clear LastGasPerVote NeoToken account info on unvoting (#3349)
 * return fault exception (if any) in `unwrap` RPC client helpers (#3356)
 * move P2PNotary designation role out of P2PSigExtensions (#3452)

Improvements:
 * support `smartcontract.Convertible` interface as RPC Actor and Invoker call
   parameters (#3297)
 * allow to import multisignature account into wallet without WIF and password
   specified (#3293)
 * upgrade go-ordered-json dependency to avoid possible node state incompatibility
   issues caused by smart contract manifest marshalling (#3331, #3333)
 * Go 1.22 support, bump minimum required Go version up to Go 1.20 (#3336)
 * a number of dependent libraries are updated to the fresh versions (#3338, #3418)
 * improve error message of `System.Crypto.CheckMultisig` interop API handler (#3374)
 * upgrade dBFT library to v0.2.0 (#3371, #3413)
 * increase `ValidUntilBlock` value for transactions generated by CLI commands
   (#3376)
 * documentation updates (#3375, #3382)
 * let default Notary Actor options be customizable (#3394)
 * extend `getversion` RPC response with RPC server settings (#3386)
 * make errors related to wallet opening more detailed (#3389)
 * adjust error messages of `setExecFeeFactor` and `setStoragePrice` methods of
   native PolicyContract (#3406)
 * make neotest chain constructor options customizable (#3404)
 * format improvements for CLI commands description (#3410)
 * make state dumps comparator script more verbose (#3411)
 * refactor storage `Get` implementation for BoltDB (#3441)
 * add `updatecounter` field to the resulting items of `getnativecontracts` RPC API
   call (#3439)

Bugs fixed:
 * node panic on committee missing from node configuration (#3294)
 * autogenerated RPC bindings do not follow Go naming convention (#3299)
 * a batch of failing tests is fixed (#3306, #3313, #3321, #3332, #3337, #3335,
   #3344, #3340, #3350, #3355, #3364, #3368, #3377, #3393, #3397, #3392, #3398,
   #3400)
 * outdated minimum required Go version of boilerplate contract generated by
   `neo-go contract init` (#3318)
 * outdated address used as an example in RPC client documentation (#3327)
 * ungraceful node services shutdown procedure (#3307)
 * logging data race on node services shutdown (#3307)
 * Map parameter support is missing from RPC server handlers (#3329)
 * smart contract storage iterator prefix wasn't copied while iterating over values
   (#3336)
 * RPC error with -511 code (insufficient funds) returned on `sendrawtransaction` RPC
   request should cover multiple cases of sender's insufficient funds (#3360, #3361)
 * reentrancy possibility for Notary deposit withdrawal (#3357)
 * null `findstorage` RPC response in case of missing storage items (#3385)
 * uninitialized named return variables in compiler (#3401)
 * wrong debug sequence points after `JUMP*` instructions shortening by compiler
   (#3412)
 * corrupted genesis block record and application log caused by improper Conflicts
   attribute processing (#3437)
 * false positive DB-based blocked accounts detection in native PolicyContract leaded
   to invalid committee computations and reward distribution made by NeoToken (#3443)

## 0.105.1 "Enumeration" (12 Jan 2024)

This is another v3.6.2-compatible release that fixes mainnet state difference at
block 4688591. It is confirmed to have the same state up to 4723K height (which
is current), but to get proper mainnet state you need to resynchronize your node
from the genesis. T5 testnet state is not affected (at least up to the current
3323K height), thus DB resynchronisation may be skipped for testnet nodes.

Improvements:
 * better network services logging (#3287, #3290)

Bugs fixed:
 * state difference at block 4688591 of N3 mainnet caused by difference at
   characters escaping during manifest's `Extra` field JSON serialisation (#3286)

## 0.105.0 "Designation" (29 Dec 2023)

We're rolling out an update for NeoGo nodes that contains a number of user-facing
API improvements for RPC web-socket notification subsystem, CLI utility, wallet
related packages and not only. Try out our new `--await` CLI option for those
commands that relay transactions to the network to automatically wait for the
on-chain transaction execution result. Subscribe for new block headers with
extended RPC notification subsystem interface. Use contract-based accounts
provided by `wallet` package and `neotest` framework to sign transactions. These
and a set of other improvements are available to our users while this release is
staying compatible with 3.6.2 version of C# node.

This version also delivers a bug fix for consensus node panic caused by improper
native NeoToken cache initialisation. Moreover, there's a set of RPC server side
improvements, thus, we recommend to upgrade both consensus and RPC nodes to
provide more stable consensus node functioning and extended user APIs functionality.
No database resynchronisation is needed.

New features:
 * block headers RPC web-socket subscription (#3252)
 * --await option to synchronize on transaction execution for CLI commands (#3265)
 * partial session-based RPC iterator unwrapping (#3274)
 * contract-based transaction signers in neotest framework (#3233)
 * AMD64 release binaries for macOS (#3251)
 * complex contract signature schemes in wallet.Account (#3256)

Behavior changes:
 * basic RPC subscription filter validity checks are implemented on both RPC
   client and RPC server sides (#3258)
 * filter of notary request event RPC subscription is extended with `type` field
   (#3236)
 * if available, use block headers RPC web-socket subscription for transaction
   awaiting via `waiter` package API (#3283)

Improvements:
 * add smart contract storage limits to interop utilities (#3232)
 * extend ZKP examples documentation with additional links to PoT ceremony
   response files (#3234)
 * support Go types in VM emitter API (#3237)
 * documentation update (#3239, #3242, #3246)
 * BoltDB (go.etcd.io/bbolt) dependency upgrade (#3250)
 * CLI code refactoring (#2682)
 * extend wallet package to work with byte slice based wallets (#3255)
 * export RPC client side transaction awaiting functionality via `waiter` package
   (#3265, #3283)

Bugs fixed:
 * remove stale `updatehistory` section of `getnativecontracts` RPC response (#3240)
 * immediately check RPC client initialisation on access to blocks RPC subscription
   API (#3257, #3261)
 * fix CN panic caused by unexpected call to native NeoToken cache (#3253)
 * make "automatically generated" warning of all automatically generated files
   follow the standard (#3280)

## 0.104.0 "Globalization" (27 Nov 2023)

We're updating NeoGo to push out a number of useful updates and protocol
extensions as well as make it compatible with 3.6.2 version of C# node. The most
invasive behaviour changes are VM-level protocol constraints imposed on the size
of serialized stackitems and `NativeActivation` node setting removal. This version
also delivers a set of smaller useful changes in the interoperability layer and
native contract functionality including System.Runtime.CurrentSigners interop,
`strLen` StdLib method and PolicyContract-based transaction attributes pricing
as far as a user-facing `canceltx` CLI command and community-requested
`--relative-path` CLI option.

Node operators must resynchronize their nodes to get fully compatible state
(which is confirmed to be compatible with 3.6.2 for current mainnet up to
block 4483627 and T5 testnet up to block 3078762). Please, ensure your node
configuration doesn't contain `NativeActivations` protocol configuration section
as this logic is hidden under Hardforks starting from the current release.

New features:
* System.Runtime.CurrentSigners interop allowing to get signers of the currently
  loaded transaction (#3058)
* `strLen` method to native StdLib contract (#3208)
* transaction attributes pricing regulation via native PolicyContract (#3155)
* `canceltx` CLI command as an alternative to unsupported `canceltransaction` RPC
  request (#3223, #3214)

Behaviour changes:
* reduce maximum allowed stackitem.Item size (#3185)
* restrict maximum allowed NEF file size and prohibit large contracts deployment (#3186)
* bind `NativeActivations` node setting to the `Hardforks` setting (#3212)
* introduce serialization limit to stackitem.Item and fail large contracts deploy
  wrt this setting (#3218)
* add customizable `MaxRequestBodyBytes` and `MaxRequestHeaderBytes` RPC server
  configuration setting (#3221)

Improvements:
* provide more detailed error on attempt to read non-existent service wallet
  provided via node configuration file (#3210)
* optimize emit of imported code for autogenerated RPC bindings (#3215)
* documentation update (#3203, #3222)
* add `--relative-path` CLI flag allowing to override configuration-specific relative
  paths (#3206)
* update code owners (#3216, @fyrchik will live in our hearts forever)

Bugs fixed:
* unify messages of RPC errors according to the RPC errors NEP (#3199)
* limit maximum allowed number ad depth of transaction signers and witnesses per
  RPC request that accept transaction (#3207, #3221)
* forbid unknown fields usage in the node configuration file (#3209)
* enable hardfork-dependant code starting exactly from the block height specified
  in the node configuration (#3211)
* require Notary native deposit to be valid for at least one subsequent block after
  deposit transaction acceptance (#3211)
* deduplicate unnamed event types for autogenerated RPC bindings and make the
  binding generation order strictly defined and stable (#3215, #3220)
* do not panic on trying to compile an import cycle (#3215)
* state difference at block 3002333 of T5 testnet caused by difference at characters
  escaping during manifest's `Extra` field JSON serialisation (#3225)
* properly start node services that depend on native RoleManagement contract data
  with genesis `Roles` protocol configuration setting specified (#3229)

## 0.103.1 "Verification" (09 Nov 2023)

An urgent 3.6.0-compatible version that contains a hotfix for the bug that
prevents node from starting from the existing database every new dBFT epoch.
Also, the release includes a bugfix that fails any non-zero NEO and GAS
roundtrips from accounts with zero balance.

Although the DB format and storage states were not affected by this release,
the node resynchronization is still recommended for those who want to keep
correct application logs. Resynchronization can be skipped if you're ok with
wrong application logs for some transactions (e.g. CN doesn't care, and RPC node
cares a lot).

Bugs fixed:
* properly initialize cache of native NeoToken contract every new dBFT epoch (#3187)
* forbid non-zero NEO and GAS roundtrips from accounts with zero balance (#3191)

## 0.103.0 "Backwardation" (20 Oct 2023)

A minor 3.6.0-compatible version of NeoGo with new salty ZKP-related
functionality, a large batch of deprecated APIs removal and an experimental
genesis block extension. Build Groth-16 circuits, generate proofs and deploy
verification contracts easily with the new end-to-end ZKP example and `zkpbinding`
NeoGo API. Setup node roles via the node configuration starting from the genesis
block without any painful multisignature transaction forming and invoke any
custom script in the genesis-level transaction with the new `Genesis` protocol
configuration extension. And don't forget to move from deprecated RPC client
APIs, as a lot of them have been removed.

New node configuration format is finally adapted and the deprecated configuration
sections are permanently removed according to the schedule. Pay attention to the
`SecondsPerBlock` protocol configuration section that was replaced by
`TimePerBlock`, a set of node and services address- and port- related configuration
section that were replaced by the new `Addresses` format, direct `UnlockWallet`
consensus configuration that was replaced by a separate `Consensus` section and
a set of node-specific protocol configurations that were moved under a separate
`P2P` section.

This release fixes a set of bugs including a vulnerability introduced by changes
in the transaction conflicts storage scheme implemented in #3061 (a part of
0.102.0 release). With the patch presented, it's impossible to uncontrollably
pollute the storage with malicious conflicting records. 

This version is fully compatible with C# node 3.6.0. However, it requires
complete resynchronization on upgrade due to the database storage scheme changes.

New features:
* API for Groth-16 verification contracts autogeneration and end-to-end example for
  proving and verifying statements on NeoGo (#3043, #3146)
* introduce genesis protocol extensions: default node roles designation and genesis
  transactions (#3168)

Behaviour changes:
* a lot of deprecated functionality is dropped: RPC client old APIs, shared
  Notifications channel of WebSocket client, SecondsPerBlock protocol
  configuration, old way of services and node address and port configuration, old
  P2P related application settings configuration, direct UnlockWallet consensus
  configuration, direct node-specific protocol configuration (#3150)
* database scheme is changed by new way of storing the transaction conflicting
  records (#3138) 

Improvements:
* NeoFS SDK dependency upgrade (#3129)
* gnark and gnark-crypto dependencies upgrade (#3145, #3162, #3163)
* introduce timeout restriction for BoltDB opening (#3148)
* bump code coverage uploading action version (#3153)
* Go 1.21 support, bump minimum required Go version up to Go 1.19 (#3157)
* upgrade golang.org/x/net version (#3158)
* add protocol hardforks configuration to the `getversion` RPC response (#3160)
* format autogenerated smart contract bindings with accordance to standard `go fmt` rules (#3164)
* add "DO NOT EDIT" warning to the autogenerated smart contract bindings (#3167)

Bugs fixed:
* avoid race between `getnextblockvalidators` RPC call handler and blockchain's
  block handler routine, significantly refactor native validators caching scheme
  (#3110)
* do not panic on failed NeoFS oracle requests (#3129)
* enable Notary subsystem in NeoFS mainnet configuration (#3136)
* reorganize storage scheme for transaction conflicting records to avoid possible attack (#3138)
* properly deserialize wildcard trusts field of smart contract manifest (#3140)
* use more strict GAS limit for transaction network fee calculation via RPC call (#3141)
* avoid WebSocket client request blocking (#3142)
* properly emit big uint64 constants during smart contract compilation (#3147)
* avoid race access to the node version by tests (#3149)
* make FindStorage* RPC client APIs to be always compatible with NeoC# RPC server (#3166)

## 0.102.0 "Aberration" (06 Sep 2023)

Long-awaited feature-packed 3.6.0-compatible version of NeoGo with all the
appropriate protocol updates and a set of tasty improvements and bug
fixes. Groth16 ZKP proof checks are there for contract developers as well as
new opcodes. A huge number of improvements went into the RPC server and
client, new RPCs, improved contract binding generator and standardized
extended error codes make developing dApps much easier.

Users of smart contract utilities and RPC-related Prometheus metrics are
advised to take a look at the deprecated APIs removal schedule
(ROADMAP.md). This version was delayed for quite some time (waiting for 3.6),
so the next minor release (0.103.0) will remove a substantial amount of
compatibility code. It's expected to be released in November with 3.6 protocol
compatibility (3.7 cycle is likely to require more time).

This is also the first version that drops support for Go 1.17 and requires
1.18+ to build (with Go 1.20 supported). Using generics in smart contracts is
not supported yet, but some elements of this support can be implemented in
future versions.

Mainnet and testnet node operators, please pay attention to the Basilisk
hardfork schedule and check your configurations. It will happen at block
2680000 for T5 testnet and 4120000 for mainnet. To ensure compatibility your
node must be configured appropriately. This version requires DB
resynchronization, so schedule your updates accordingly.

New features:
 * ZKP proof checking support via CryptoLib native contract (operations with
   BLS12-381 curve points) (#2940, #3042)
 * System.Storage.Find interop now support "Backwards" flag to iterate in the
   opposite direction (#2952)
 * `util ops` CLI utility that pretty-prints VM script (#2965)
 * `notarypool_unsorted_tx` Prometheus metric for notary-enabled nodes (#2696)
 * `CloseNotificationChannelIfFull` WSClient option allowing the client to
   close notification channel on overflow (#2988)
 * autogenerated RPC bindings for contract events and conversion code from
   stackitem.Item to structure (#3008, #3035, #3036, #3087)
 * event type inference from contract code (#3008)
 * dynamic contract hashes support for RPC bindings (#3012)
 * "Basilisk" protocol hardfork support (#3056, #3080, #3086, #3104)
 * ABORTMSG and ASSERTMSG VM opcodes (#3066)
 * standardized RPC error codes (#3063)
 * `findstorage` and `findstoragehistoric` RPC support (#3099)
 * `getstoragehistoric` RPC support (#3099)
 * `getrawnotarypool` and `getrawnotarytransaction` RPC (#3098)

Behaviour changes:
 * deprecated `FromAddress` smart contract helper is dropped from the `util`
   interop package (#2941)
 * a number of deprecated RPC-related Prometheus counters are permanently
   removed (#2941)
 * NEP-2 account label will be asked on account import via CLI (#2889)
 * hashes and states of native contracts are now accessible via native
   ContractManagement API (#2991)
 * `serv_node_version` gauge Prometheus metric is marked as deprecated and
   replaced by `neogo_version` and `server_id` (#3009)
 * strict contract script check is back with Basilisk hardfork, it was
   previously removed in 0.101.3 for 3.5 protocol compatibility (#3056)
 * JSON number deserialization (via StdLib.jsonDeserialize) changes with
   Basilisk hardfork allowing for more precision and handling more corner
   cases (#3080)
 * notification type errors are treated as fatal after Basilisk hardfork,
   before it they're just logged (#3086)
 * NULL and non-UTF8 items are not allowed for String event types (#3086)
 * `Conflicts` and `NotValidBefore` transaction attributes are no longer NeoGo
   extensions, they can be used on regular networks (#2962)

Improvements:
 * documentation and example improvements (#2945, #2972, #2979, #3020, #3084,
   #3099, #3114, #3116, #3119, #3121)
 * `getproof` and `verifyproof` RPC API support in RPC client (#2942)
 * Go 1.20 support, bump minimum required Go version up to Go 1.18 (#2908)
 * faster state reset process (#2819)
 * optimized voting data storage scheme for NEO contract (#2892, #2893)
 * NeoFS SDK dependency upgrades (#2995, #3032)
 * special exported error returned in case of WSClient disconnection (#3000)
 * automatic guessing of contract and manifest filenames for `contract
   compile` CLI command and `loadnef` VM CLI command (#3013)
 * support for pushing stackitem.Convertible objects via VM script emitter (#3016)
 * economic adjustment for ranking of transactions with `Conflicts` attribute (#3031)
 * `google.golang.org/grpc` dependency upgrade fixing high severity security
   vulnerability (#3055, gRPC is only used to communicate with NeoFS nodes in
   the oracle service)
 * enforce default RPC server values when it's used as an independent package
   (not a part of node) (#3107)
 * unwrap.Nothing function for RPC clients (#3117)
 * address and reverse hash display in opcode dumps (#3115)

Bugs fixed:
 * invalid peer port type returned by `getpeers` RPC response (#2914)
 * invalid data source for `mempool_unsorted_tx` Prometheus metric (#2969)
 * dBFT library upgrade fixing the ability of a single node to speed up the
   process of new blocks creation for the whole network (#3018,
   nspcc-dev/dbft#75)
 * failing CALLT instructions in VM CLI for loaded NEF files (#3020)
 * missing signers check for on-chain conflicting transactions (#3061)
 * incorrect sequence point boundaries in debug data (#3074)
 * compiler panic on encountering generic code (#3041)
 * potential pooled fallback notary transaction changes (#3108)
 * lost LastGasPerVote value on NEO state deserialization for non-voting
   accounts (#3122)

## 0.101.4 "Yarborough" (01 Aug 2023)

Another one 3.5.0-compatible version that is aimed to fix T5 testnet state
difference that has happened at block 2336911 which leads to inability to process
new blocks since 2418703. The issue is fixed by allowing JSON numbers
unmarshalling from scientific notation to Integer stackitem. Maximum parsing
precision for such numbers is currently restricted by 53 bits. This is a
temporary C#-compatible solution that is likely to change in the future versions
when an appropriate C# node bug is fixed (neo-project/neo#2879).

A set of minor bug fixes is included as well to flush some of the long-awaited
changes that were blocked by the 0.102.0 release delay (caused by v3.6.0 C# node
release delay). In particular, invalid headers returned by an RPC server for
error responses, invalid format of incremental dumps created by CLI and deadlock
on unhealthy RPC server shutdown. Long-awaited `--config-file` CLI option to
start the node providing a single configuration file is added.

T5 testnet chain requires a complete resynchronization for this version. Mainnet
chain resynchronization is recommended, but not required.

New features:
 * `--config-file` CLI option allowing to start the node with a single configuration file (#3014)

Improvements:
 * blockchain Notary and Oracle services documentation improvement (#2972)
 * BoltDB (`go.etcd.io/bbolt`) dependency upgrade that fixes a number of Windows-related issues (#3034)

Bugs fixed:
 * panic on node start with invalid configuration (#2968)
 * deadlock on unhealthy RPC server shutdown (#2966)
 * improper WSClient notification channels managing after disconnection (#2980)
 * missing Prometheus metric initialisation on node start (#2992)
 * invalid initialisation of native contracts cache (#2994)
 * incorrect way of incremental DB dumps creation (#3047)
 * Notary contract is allowed to be a sender of main Notary request transaction (#3065)
 * discrepancy in signer's witness scope parsing on the RPC server side (#3060)
 * Invoker calling API isn't allowed to accept nil parameter (#3067)
 * contract RPC Client unwrapper helper can't handle missing contract case (#3072)
 * JSON numbers can't be unmarshalled to stackitem from scientific notation (#3073)
 * invalid content-type header returned by RPC server on error responses (#3075)

## 0.101.3 "Yuckiness" (08 Jul 2023)

Yet another 3.5.0-compatible emergency version that removes scrupulous
instructions check of smart contract's script on contract deployment or update.
Presence of this check leads to the known T5 testnet state incompatibility
(since 1670095) which causes inability to process new blocks (since 2272533).
It should be noted that the corresponding check was accidentally removed from
the reference C# node implementation way back in neo-project/neo#2266 and added
again in neo-project/neo#2849 which is planned to be a part of the upcoming
3.6.0 C# node release. Thus, changes made in the presented 0.101.3 release will
be reverted afterwards and strict contract script check will be present in the
next 3.6.0-compatible version of NeoGo node.

T5 testnet chain requires a complete resynchronization for this version. Mainnet
chain resynchronization is recommended.

Bugs fixed:

 * extra strict contract script check on contract deployment or update is
   removed (#3052)

## 0.101.2 "Excavation" (29 Jun 2023)

One more (and unexpected one!) 3.5.0-compatible version that fixes a VM bug
leading to mainnet state incompatibility (since 3672783) which in turn leads
to inability to process new blocks (since 3682290).

Mainnet chain needs to be resynchronized for this version.

Improvements:
 * documentation updates (#3029, #2990, #2981)

Bugs fixed:
 * incorrect handling of empty Any-type parameter for RPC invocations (#2959)
 * incorrect state rollbacks in case of exception during cross-contract call
   when the call is made from non-TRYing context (#3046)

## 0.101.1 "Shallowness" (17 Mar 2023)

Another 3.5.0-compatible version that delivers important bug fixes and
provides a new API to be used by NeoFS. An upgrade is recommended, the DB
doesn't need to be resynchronized.

New features:
 * internal RPC client for deeply integrated applications like NeoFS (#2916)

Improvements:
 * documentation updates (#2879, #2880, #2893, #2917, #2920, #2936)
 * code style, spelling and updated linter fixes (#2884, #2922, #2933)
 * NEP-2 import password can be provided via config file now (#2887)
 * custom stack item deserialization limit is available via public APIs now (#2904)
 * RPC client endpoint can be retrieved via public API (#2915)
 * dependency updates (#2919, #2929)
 * WSClient now copies filter parameters to Subscribe* and Receive* methods
   improving code safety (#2937)

Bugs fixed:
 * name parameter ignored for wallet import command (#2887)
 * incorrect RPC binding code generated for Any return type (#2918)
 * memory leak on active peer disconnection (#2924)
 * consensus process deadlock (#2930)
 * dBFT deadlock in "committed at previous view" scenario (#2935)
 * panic in RPC waiter code (#2938)

## 0.101.0 "Shortness" (13 Jan 2023)

This release delivers an important fix for block execution application logs
and requires a resynchronization, therefore it's 0.101.0 (even though it's
also 3.5.0-compatible). It fixes some other minor problems as well (the other
most notable change probably is in the compiler), so we recommend upgrading.

Improvements:
 * updated golang.org/x/* dependencies (#2854)
 * CLI help and required flags handling fixes (#2852)
 * transfer data storage optimization (#2865)
 * network's magic number is stored (and checked against the config on
   startup) in the DB now, reducing potential for node operator errors (#2867)

Bugs fixed:
 * in rare cases nodes could request an invalid number of blocks from peers
   leading to disconnect (#2846)
 * outdated documentation fixes (#2860, #2876)
 * application logs for blocks that contained GAS spends for transaction fees
   contained (and returned from getapplicationlog RPC) incorrect (off by one)
   values for amount in Transfer events; transaction application logs were not
   affected by this, but data returned to RPC event subscribers could
   potentially be (#2865)
 * findstates RPC returned an error instead of an empty data set for valid
   contracts that have no data (unlike C# node, #2866)
 * miscompiled shadowed range loop variable definition (#2871)
 * missing (compared to C# node) explicit (even though null) 'exception' field
   in the getapplicationlog RPC output from server (#2872)

## 0.100.1 "Chaptalization" (28 Dec 2022)

This is a tiny update that 99.99% of users can easily skip. The reason for
this release is the need to fix compatibility with the NeoFS mainnet
sidechain and have some stable version to be used there. In any other case it
can be ignored, but if you still decide to upgrade you don't need to
resynchronize.

Behaviour changes:
 * Aspidochelone fork is made to include ContractManagement native contract
   deploy/update methods call flags change, initially it was an unconditional
   part of 0.99.0 NeoGo release (or 3.2.0 C# version), but this behavior is
   incompatible with the NeoFS mainnet sidechain; the change to the fork logic
   does not affect any other public networks (mainnet/testnet) and any new
   networks that have Aspidochelone enabled since block 0 (#2848)

Improvements:
 * more robust NEP-11 divisibility check in the RPC server code (#2841)
 * microoptimization for some debug log messages (#2842)
 * additional fuzz tests for integer serialization and script parsing code (#2851)

## 0.100.0 "Centuplication" (08 Dec 2022)

A 3.5.0-compatible version of NeoGo with all the appropriate protocol updates
and a number of other changes. The most notable ones are configuration
updates. New features and some long-standing inconsistencies required for some
changes and this release brings them with it. Old configurations are still
supported and will work the same way (except for one minor exception in
VerifyBlocks that is only supposed to be used for tests/development), but we'd
like to highlight that all of the old settings will be removed in ~6 months,
so please review these updates and update your configurations. For public
networks the best way to go is to take the new versions from the "config"
directory and then adjust for particular scenario if needed.

This release requires a complete resynchronization due to native contract
changes, so please schedule your updates appropriately.

New features:
 * System.Runtime.LoadScript syscall (and appropriate smart contract interops)
   allowing to load/run dynamic code (#2719)
 * PUSHT/PUSHF VM instructions allowing for simpler/cheaper booleans (#2770)
 * ContractManagement native contract was extended with ID->hash mappings and
   `getContractById` and `getContractHashes` methods (#2702, #2837)
 * LogLevel application configuration option that can be changed on SIGHUP
   (#2831)
 * additional type data generated by the compiler that then can be used by the
   RPC bindings (SDK/contract wrapper) generator, this allows for complex
   types (structures/arrays/maps) to be easily represented/handled in
   contract-specific RPC code (#2828)
 * multiaddress listeners for all services and P2P, this changes the old
   Address/Port (and AnnouncedPort for P2P) configuration scheme to more
   generic Addresses list, the old configuration is still supported, but is
   deprecated and will be removed in future node versions (#2827, #2839)

Behaviour changes:
 * Aspidochelone fork block for NeoFS sidechain mainnet configuration is
   rescheduled again (#2823, #2830)
 * Blockchain's GetHeaderHash() method now accepts uint32 for parameter
   (#2814)
 * pre-0.97.3 and pre-0.99.0 deprecated compatibility fields and logic were
   dropped from the result.Version (`getversion` RPC result) structure (#2786)
 * SecondsPerBlock protocol configuration variable was replaced with
   TimePerBlock allowing for sub-second precision, SecondsPerBlock is still
   supported, but will eventually be removed (#2829)
 * AttemptConnPeers, BroadcastFactor, DialTimeout, ExtensiblePoolSize,
   MaxPeers, MinPeers, PingInterval, PingTimeout, ProtoTickInterval settings
   were moved into the new P2P section, timing parameters now use Duration
   type allowing for more human-friendly values in many cases, old parameters
   are still supported, but will eventually be removed (#2827)
 * consensus (dBFT) is configured as a separate service now that can be
   enabled/disabled (and can work in "watch only" mode without a wallet), the
   old direct "UnlockWallet" specification in "ApplicationConfiguration" is
   still supported, but is deprecated and will be removed in future versions
   (#2832)
 * GarbageCollectionPeriod, KeepOnlyLatestState, RemoveUntraceableBlocks,
   SaveStorageBatch and VerifyBlocks settings were moved from
   ProtocolConfiguration to ApplicationConfiguration; old configurations are
   still supported, except for VerifyBlocks which is replaced by
   SkipBlockVerification with inverted meaning (and hence an inverted default)
   for security reasons; this also affects NewBlockchain and neotest APIs
   (#2833)

Improvements:
 * more user-friendly error and help messages in some cases (#2824, #2834)
 * faster node startup time and smaller memory footprint for networks with
   lots (1-2M+) of blocks (#2814)
 * minor documentation fixes (#2834, #2838)

Bugs fixed:
 * transactions with system fee or more than MaxBlockSystemFee are no longer
   accepted into the mempool, preventing a form a network-wide DoS (#2826)
 * deprecated WSClient subscription methods not working correctly when filters
   are being used (#2836)

## 0.99.7 "Hyalinization" (23 Nov 2022)

Bugs, terrestrial arthropods with at least six legs. Sometimes we don't notice
them, sometimes we ignore them, but they do exist and there are times when
they need to be dealt with in one way or another. Most of the bugs fixed in
this release have a little less legs than proper insects, they're far from
being as critical as the ones fixed in the previous release and yet we
consider them annoying enough to warrant this new version of NeoGo to be
published. We don't want to see our users fighting these pesky creatures in
the wild, better update and have a troublefree experience.

If you're not affected by #2815 (requests for verbose transaction data and
application logs failed for some specific mainnet transaction(s)) you may
leave the DB as is with this upgrade, if you want it to be solved then please
resynchronize.

Behaviour changes:
 * "loadtx" VM CLI command now uses transaction system fee value as the
   default GAS limit for the context, unless --gas is used (previously it
   wasn't limited, #2816)
 * SendRawTransaction RPC client method will now always return transaction
   hash, even if error occured, this also affects Actor APIs and allows to
   handle some specific errors (like transaction already present in the
   mempool) in more appropriate way (#2817)
 * Actor and NotaryActor Wait() wrapper methods now handle "already exists"
   and "already on chain" errors as non-errors, waiting (if needed) and
   returning proper results (#2817)

Improvements:
 * reworked network discoverer/connection manager that has a better picture of
   the network (especially if it's a small one), doesn't try to make additional
   connections (even when having less than MinPeers of them) when there are no
   more real addresses to connect to, establishes stable connections faster in
   containerized environments and does more connection attempts when MinPeers
   setting is not satisfied, but there are potential candidates (#2811)
 * VM invocation stack handling microoptimization (#2812)
 * load* VM CLI commands now accept --gas parameter to set the amount of GAS
   to be used in this context (#2816)
 * better logging during state reset process (#2813)
 * subscription-based transaction waiter now handles already accepted
   transactions faster (~immediately vs waiting for block timeout previously,
   #2817)
 * WSClient can return more specific error on websocket handshake failure if
   it's provided by server (#2821)
 * new MaxWebSocketClients configuration option for the RPC server to control
   the number of allowed websocket clients, missing or 0 value make the server
   use the default of 64 which is compatible with the previous behavior (you
   can use old configurations if you don't care about this option, #2821)

Bugs fixed:
 * occasional "loadgo" VM CLI test failures because of timeout (#2810)
 * websocket waiter test instability (#2809)
 * websocket event unsubscriptions could lead to node deadlock in rare
   circumstances (#2809)
 * exception handler could use improper evaluation stack in some cases (#2812)
 * pointer stack items were not handled correctly in "protected" serialization
   mode leading to inability to deserialize these items back (#2816)
 * state reset functionality not working for big chains like mainnet (#2813)

## 0.99.6 "Funambulation" (16 Nov 2022)

An emergency release fixing mainnet incompatibility at block 2504320 leading
to inability to process block 2504813. A couple of other less severe bugs got
fixed as well. Unfortunately, this release requires complete resynchronization
for mainnet, other networks can use old DBs.

Improvements:
 * only transactions requested by the consensus process are forwarded to it
   now slightly improving performance in some networked scenarios (#2788)
 * a complete set of simple array unwrappers in the unwrap library (#2792)
 * RPC binding generator can handle simple arrays now with configuration file
   provided (#2792)
 * more user-friendly error messages in some cases (#2797, #2802, #2804)

Bugs fixed:
 * DB was not properly closed after state reset (#2791)
 * VM stack handling problem leading to lost items after return from call to
   another contract (#2800)
 * "istack" command panic in the VM CLI (#2800)
 * failure to decode incorrect contract from ContractManagement state leading
   to failure to start the node, it'll be ignored now similar to how C# node
   treats this (#2802)
 * subscription to execution events failed if no "state" filter was used along
   with "container" (#2804)
 * Actor using WSClient could deadlock in the transaction waiter during
   unsubscription (#2804)

## 0.99.5 "Underestimation" (11 Nov 2022)

It wasn't long since the previous release until a juicy set of features and
optimisations came on board. So we've decided to change our release plan and
supply one more 3.4.0 compatible version. It comes up with a massive suite
of network P2P and broadcast improvements. We've carefully investigated message
sending timeouts on benchmarks, reworked the broadcast logic and significantly
reduced delays in message sending and the amount of useless traffic between our
nodes. For smart-contract developers and RPC client users several new features
were added. These are automatic RPC bindings generator, poll-based and
websocket-based transactions awaiting and an extended websocket client
subscriptions interface. For better debugging experience we've improved state
related functionality of VM CLI and added an ability to reset chain state to a
particular height.

An important dBFT initialization bug that caused timestamp-related errors on
block addition was fixed. Also, those who use NeoGo node with
EnableCORSWorkaround setting may want to update the node to be able to handle
web-socket requests with Origin set in the header. The users of websocket RPC
client subscription mechanism are welcomed to move from deprecated subscriptions
API to the extended one. Finally, we are kindly asking our users to have a look
at our roadmap file where deprecated code removal schedule is attached.

This version is fully compatible with C# node 3.4.0 and does not require
resynchronization on upgrade. This time for sure, the next release is planned
to be compatible with 3.5.0.

New features:
 * logarithmic gossip fan-out, new `BroadcastFactor` application setting and
   adaptive peer requests are added to optimise broadcasting process (#608,
   #2678, #2743)
 * Prometheus histograms with handling time are added for P2P commands (#2750)
   and RPC calls (#2784)
 * transaction awaiting is added to RPC clients (#2740, #2749)
 * RPC bindings generator (#2705) is added for both safe (#2766) and
   state-changing (#2769, #2778) methods with initial iterator support (#2783)
 * the ability to reset chain state to a particular height is added (#2576,
   #2773)

Behaviour changes:
 * Aspidochelone fork block for NeoFS sidechain mainnet configuration is
   rescheduled (#2754, #2776)
 * RPC server will now handle any Origin in websocket connection when
   EnableCORSWorkaround setting is on (#2772, #2724)
 * WS RPC client subscriptions API is extended to accept custom subscriber
   channels, old subscriptions API is marked as deprecated (#2756, #2764)

Improvements:
 * network code refactoring (#2742)
 * network packets sending is now limited in time (#2738)
 * broadcast messages will be sent concurrently to each peer (#2741)
 * ping messages are broadcast using the common broadcast logic (#2742)
 * transactions requested by dBFT will be accepted on demand only (#2746, #2748,
   nspcc-dev/dbft#63)
 * VM CLI's state-based functionality improvements (extended signers and
   arguments parsing, enabled logging) and additional commands (`loadtx`,
   `loaddeployed`, `jump`) (#2729, #2606, #2740)
 * incoming P2P transactions are now processed concurrently and buffer for
   inventory hashes is reusable (#2755)
 * `GetData` and `GetBlocksByIndex` replies are now batched on a TCP-level
   (#2757)
 * optimisation of incoming P2P transactions and blocks filtering are introduced
   (#2758, #2759)
 * batch size of broadcast transactions hashes is adjusted (#2760)
 * peer reconnections logic is improved, `GetAddr` messages are sent more
   often (#2761, #2745)
 * documentation for wallet configuration file is clarified (#2763)
 * Koblitz curve implementation of btcd/btcec was replaced with decred/secp256k1
   due to GitHub security warning (#2781)
* `NOT` opcode for Bool emission is now used to reduce resulting script
  cost (#2762)

Bugs fixed:
 * failing tests cleanup on Windows (#2736)
 * race in CLI test (#2765)
 * invalid conversion of PublicKey smart contract parameter to emitable (#2739)
 * initialize dBFT's context with previous block's timestamp on view 0 (#2753,
   #2752, nspcc-dev/dbft#64)

## 0.99.4 "Transliteration" (07 Oct 2022)

A small update mostly interesting for people building/testing smart contracts
with NeoGo. It contains long-awaited VM CLI update that allows to use
blockchain state and complete set of interops, additional helper functions for
smart contracts and notification checking code (for upcoming 3.5.0 protocol
changes). Node operators using EnableCORSWorkaround (that is still not
recommended, but available) also may want to update to be able to handle
pre-flight CORS requests.

This version is compatible with C# node 3.4.0 and does not require
resynchronization on upgrade. The next release is planned to be compatible
with 3.5.0.

New features:
 * address conversion helpers for smart contracts (#2698)
 * interop helper to call specific version of a contract (#2695)
 * notifications are now checked for manifest compliance, a warning is logged
   in case on detected inconsistencies (#2710)
 * VM CLI can now use blockchain state DB (including historic states) to run
   code with a complete set of interops and contracts available; additional
   commands were added to it to inspect storage and generated events (#2723)

Behavior changes:
 * type assertion with two return values in Go contracts can't be compiled now
   (it never worked properly, #2718)
 * RPC server will now handle pre-flight CORS requests (via OPTIONS methods)
   when EnableCORSWorkaround setting is on (#2724)
 * all transaction-generating CLI commands were unified to accept
   gas/sysgas/out/force parameters, ask for confirmation and be able to save
   transaction into a file; this affects wallet claim and candidate-related
   commands mostly (#2731)

Improvements:
 * smartcontract.Builder API was extended with Len method (#2691)
 * NNS example contract adjustments for better DNS compatibility (#2679)
 * documentation updates (#2708, #2722, #2726, #2728)
 * compiler now emits code to explicitly cast runtime.Notify() parameters to
   appropriate types (#2720)
 * CLI tests repackaging (#2725)
 * NeoFS sidechain configurations for mainnet and testnet (#2730)

Bugs fixed:
 * panic on node shutdown (#2689)
 * panic on inlined append() compilation (#2716)

## 0.99.3 "Simplification" (09 Sep 2022)

Version 0.99.3 brings with it most of the planned refactoring changes to the
RPC client. dApp backend code can be greatly simplified in many cases by
using new APIs (old ones are still available for transition period of about
one-two releases more). We also have some updates for CLI and compiler and a
number of bug fixes.

This version is compatible with C# node 3.4.0 and does not require
resynchronization on upgrade. The next release is planned to be compatible
with 3.5.0 (if it's to be released around currently planned dates).

New features:
 * native contract RPC wrappers (#2643, #2646, #2650, #2666, #2665)
 * NEP-11 RPC wrappers (#2651)
 * invoker interface extension with session-based iterators support (#2650)
 * notary Actor greatly simplifying creation of notary-assisted transactions
   via RPC client (#2665)
 * historic smart contract calls can now be made via CLI (#2683)

Behavior changes:
 * calculatenetworkfee RPC can handle paid attributes (NeoGo extensions) and
   invalid contract signatures now, it won't return an error for them (#2658)
 * graceful node shutdown on SIGTERM (#2660)
 * wallet balance commands now require at least 0.99.1 NeoGo version (or
   compatible C# node) used by the RPC server (#2667)

Improvements:
 * build system corrections (#2641, #2684)
 * additional types in `unwrap` (#2650, #2651)
 * session iterator consistency check in `unwrap` (#2650)
 * multitransfers in NEP-17 RPC wrapper (#2653)
 * extended transaction validity time in CLI wallet commands (#2653)
 * reference counter optimization in VM (#2659)
 * RPC Actor API can have default (used for all transactions) attributes and
   callbacks now (#2665)
 * neptoken RPC package can now provide wallet.Token data (#2667)
 * NEP-11 balance commands can now work without direct token hash
   specification or previous token import (#2667)
 * support for offline signing in CLI (#2668)
 * compiler can optimize out unused global variables now (#2624)
 * private keys are now cleaned up from memory when they're no longer needed,
   additional Account APIs added to reduce direct interactions with private
   keys (#2672)
 * updated linter settings, some code cleanup (#2674)
 * more documentation and examples for new RPC APIs (#2681)
 * refactored state-changing methods of NEP-11/NEP-17 RPC wrappers into a
   separate structure to simplify reusing them in higher-level code (#2681)
 * simplified rpcclient historic API (#2686)

Bugs fixed:
 * compiler panic on empty package list (#2645)
 * compiler not allowed to use unnamed parameters in exported methods (#2648)
 * compiler allowed to export multireturn functions (#2648)
 * compiler panic on nil method receiver in the compiled code (#2649)
 * compiler panic on variable initialization from multireturn call (#2644)
 * potential lockups or panics on node shutdown (#2652)
 * contract manifest not checked for correctness in bindings generation CLI
   command (#2656)
 * SignTx wallet Account API could lead to inconsistent result in some cases
   (#2665)
 * wallet Account API allowed to sign with locked accounts (#2665)
 * potential panic in keys.WIFDecode on some inputs (#2672)

## 0.99.2 "Recalibration" (12 Aug 2022)

This is a 3.4.0-compatible (tested for mainnet and testnet) update to NeoGo
that implements all of the required protocol changes, adds a lot of new RPC
client interfaces and provides additional service management functions. This
is also the first version to support the recently released Go 1.19. Support
for Go 1.16 is removed at the same time, so you need Go 1.17+ to build NeoGo
now.

With this version you can turn on/off, restart or reconfigure internal node
services like RPC or Oracle without full node shutdown/startup. Node operators
can send an appropriate Unix signal and that'll do the job. At the same time
node operators must resynchronize their nodes when updating to 0.99.2 due to
native contract state change.

A number of RPC client's methods are now deprecated (but still available to
simplify transition), please explore new APIs, try them and leave feedback in
#2597 if any. We expect new APIs to better fit the needs of various backend
RPC client use cases while allowing to remove some repetitive code from
applications at the same time. This release only contains generic NEP-17 call
wrapper, so it doesn't cover all of the things currently provided by the
old client API, but this will be fixed in future releases.

New features:
 * signal-based configuration and service reloads (#2612)
 * new APIs in smartcontract package to convert Go types into NEP-14
   parameters (#2621, #2632)
 * invoker package to deal with invocations via RPC in various contexts
   (mostly useful for read-only and/or historic calls when used directly,
   #2621)
 * `hasMethod` method in the ContractManagement native contract (#2598, #2619,
   #2640)
 * actor package that helps in creating and sending transactions to the
   network via RPC using the same set of signers (can be used directly, but
   mostly intended to be used by higher-order contract wrappers, #2632, #2637)
 * unwrap package in the RPC client that checks for execution results and
   converts returned stack items to expected Go types (can be used directly,
   but more useful when being used by upper-layer contract wrappers, #2638)
 * nep17 RPC client package for simple remote access to NEP-17 contract
   methods via RPC, both safe and state-changing (#2642)

Behavior changes:
 * smartcontract.Params type and (*Parameter).TryParse APIs were removed
   completely as unused and not useful (#2621)
 * compiler now rejects unnamed parameter for exported methods (but they
   didn't work properly anyway, #2601)
 * restored deploy-time out of bounds script checks (#2538, #2619)
 * dynamic scripts (like entry scripts) no longer can emit notifications
   (#2613)
 * db dump and db restore CLI commands now reject `--debug` option (it was
   ignored previously, #2625)

Improvements:
 * manifest correctness check during compilation (#2601)
 * more robust CalledByEntry scope check (#2613)
 * a check for excessive arguments in CLI (#2625)
 * better help messages for CLI commands (#2625)
 * Go 1.19 support (#2634)
 * saved transactions created via CLI (`--out`) now use the maximum possible
   ValidUntilBlock values (#2632)
 * RPC server now returns more data about NeoGo-specific protocol extensions
   in `getversion` reply (#2632)

Bugs fixed:
 * NEP-6 wallet version was wrong wrt the standard and C# implementation
   (#2633)
 * smart contract invocations from the CLI didn't compensate for interactive
   prompt wait time which could lead to transaction expiration before it was
   sent (#2632)
 * Oracle native contract's `finish` method reentrancy problem (#2639)

## 0.99.1 "Parametrization" (28 Jul 2022)

We're updating NeoGo to push out a number of significant updates as well as
make it compatible with 3.3.1 version of C# node. The most prominent changes
are RPC sessions for iterators returned from invoke* calls, initial bits of
RPC client refactoring and support for darwin-arm64 and linux-arm64
platforms. A number of internal changes are less visible from outside, but
are also important for future evolution of the code base.

Node operators must resynchronize their nodes to get fully compatible state
(which is confirmed to be compatible with 3.3.1 for current mainnet up to
block 1932677 and T5 testnet up to block 475938). RPC client users, please
review the changes carefully and update your code wrt refactorings made as
well as iterator session support.

Subsequent releases will also change RPC client and associated code, we want
to make it easier to use as well as extend its functionality (see #2597 for
details, comments and suggestions are welcome). 0.99.2 is expected to be
released somewhere in August with 3.4.0 C# node compatibility.

New features:
 * `getcandidates` RPC method to get full list of registered candidates and
   their voting status (#2587)
 * wallet configuration file support to simplify using CLI non-interactively,
   use `--wallet-config` option instead of `--wallet` if needed (#2559)
 * session-based JSON-RPC iterator API for both server and client with
   `traverseiterator` and `terminatesession` calls; notice that the default
   server behavior (`SessionEnabled: false`) is compatible with NeoGo 0.99.0,
   iterators are expanded the way they were previously, only when sessions are
   enabled they're returned by the server to the client (#2555)
 * `interop` package now provides helper methods for proper `Hash160`,
   `Hash256` and other type comparisons in smart contracts (#2591)
 * `smartcontract.Builder` type to assist with entry script creation as well
   as a set of CreateXXXScript functions for simple cases (#2610)

Behavior changes:
 * 3.3.1-compatible order of ABI methods for the native NeoToken contract
   which affects LedgerContract state (#2539)
 * `getnextvalidators` RPC was reworked to be compatible with C# node, if you
   rely on Active flag from the old response or full candidate list please use
   `getcandidates` method (#2587)
 * `--account` parameter for `contract manifest add-group` CLI command was
   changed to `--address` (short `-a` is still the same) for better
   consistency with other commands (#2559)
 * (*WSClient).GetError won't return an error in case the connection was
   closed with (*WSClient).Close
 * `getnepXXbalances` RPCs now return decimals, symbol and token name data
   along with asset hash (#2581)
 * Ledger.GetBlock will now return state root hash for chains that use
   StateRootInHeader extension (#2583)
 * `request.RawParams` type is gone (but it likely wasn't used anway), if
   needed use `[]interface{}` directly (#2585)
 * `RawParams` field of `request.Raw` was renamed to `Params` (#2585)
 * `vm.State` type moved to a package of its own (`vm/vmstate`) to avoid
   importing whole VM where only State type is needed (#2586)
 * `MaxStorageKeyLen` and `MaxStorageKeyLen` definitions moved from
   `core/storage` to `config/limits` package simplifying their usage (#2586)
 * `vm.InvocationTree` type moved into `vm/invocations` (#2586)
 * `storage.Operation` type moved into `storage/dboper` package (#2586)
 * `rpc/server` package moved to `services/rpcsrv` (#2609)
 * `rpc/client` package moved to `rpcclient` (#2609)
 * `network/metrics` package moved to `services/metrics` (#2609)
 * `rpc/request` and `rpc/response` packages were merged under `neorpc`,
   `request.Raw` is `neorpc.Request` now, while `response.Raw` is
   `neorpc.Response` (#2609)
 * `rpc.Config` type was moved to `config.RPC` (#2609)
 * `subscriptions.NotificationEvent` type moved to
   `state.ContainedNotificationEvent` (#2609)
 * `subscriptions.NotaryRequestEvent` type moved to
   `result.NotaryRequestEvent` (#2609)

Improvements:
 * new `make` targets to build NeoGo binaries locally or using Docker
   environment (#2537, #2541)
 * more detailed client-side error on JSON-RPC WebSocket connection closure
   (#2540)
 * various node's micro-optimizations both for CPU usage and memory
   allocations (#2543)
 * new Wallet method that allows to save it in pretty format (#2549)
 * massive test code refactoring along with some core packages restructuring
   (#2548)
 * transaction package fuzz-test (#2553)
 * simplified client-side Error structure, more predefined errors for
   comparisons, refactored server side of RPC error handling (#2544)
 * more effective entry scripts for parameterless function invocations via RPC
   (#2558)
 * internal services now can start/stop properly independent of other node
   functionality (#2566, #2580)
 * all configurations now use 'localhost' instead of 127.0.0.1 (#2577)
 * documented JSON behavior for enumerations which deviates from C# node
   slightly (#2579)
 * better error messages in the CLI for invocations (#2574)
 * compiler can inline methods now (#2583)
 * `interface{}()` conversions are supported by the compiler now (although
   they don't change values, #2583)
 * server-side code moved out of common RPC packages completely (#2585, #2586)
 * metrics and DB configurations moved to config and dbconfig packages (#2586)
 * calling methods on returned values is now possible in Go smart contracts
   (#2593)
 * NNS contract now cleans up old entries in case a domain is registered again
   after expiration (#2599)
 * GetMPTData P2P messages (from P2PStateExchangeExtensions) can now be
   processed even if KeepOnlyLatestState is enabled (#2600)
 * builds and regular tests for MacOS (#2602, #2608)
 * internal `blockchainer.Blockchainer` is finally gone, simplifying some
   dependencies (#2609)
 * updated CLI `--version` output format to conform with NeoFS tooling style
   (#2614)

Bugs fixed:
 * dBFT update: increase the number of nodes that respond to RecoveryRequest
   (#2546, nspcc-dev/dbft#59)
 * invalid block height estimation for historic calls (#2556)
 * Signature parameter to RPC invocations was expected in hex format instead
   of base64 (#2558)
 * missing data in RPC server error logs in some cases (#2560)
 * potential deadlock in consensus node before the dBFT process is started
   (#2567)
 * dBFT update: ChangeView messages were not correctly transmitted in
   RecoveryResponse messages (#2569, nspcc-dev/dbft#60)
 * dBFT update: ChangeView messages from newer views were not correctly
   processed by outdated node (#2573, nspcc-dev/dbft#61)
 * RPC server restarts via SIGHUP could lead to server not starting at all in
   very rare case (#2582)
 * inlined code couldn't have multiple `return` statements (#2594)
 * empty list of transactions is now returned instead of `null` in `getblock`
   RPC responses when block doesn't contain transactions, the same way C# node
   does (#2607)
 * candidate registration didn't invalidate committee cache leading to state
   differences for T5 testnet (#2615)

## 0.99.0 "Overextrapolation" (03 Jun 2022)

A big NeoGo upgrade that is made to be compatible with C# node version
3.3.0. All of the protocol changes are implemented there with the main one
being the Aspidochelone hardfork that will happen on mainnet
soon. Compatibility is confirmed for current T5 testnet and mainnet, but this
version requires a resynchronization so schedule your updates accordingly.

But it's not just about the protocol changes, this release introduces an
ability to perform historic invocations via RPC for nodes that store old MPT
data. Using `invokefunctionhistoric` you can perform some invocation with the
chain state at the given height, retreiving old balances, ownership data or
anything else you might be interested in.

Please also pay attention to configuration files used by your node, mainnet
ones must have Aspidochelone hardfork enabled at height 1730000 to be
compatible and T5 testnet enables it at height 210000. T5 testnet is different
from T4, it uses different magic number, different seed nodes and different
protocol configuration settings. 0.99.0 won't work for T4 testnet, please
use 0.98.5 for it.

New features:
 * new methods in native contracts:
   - getTransactionVMState and getTransactionSigners in LedgerContract (#2417,
     #2447, #2511)
   - murmur32 in CryptoLib (#2417)
   - getAllCandidates and getCandidateVote in NeoToken (#2465)
 * System.Runtime.GetAddressVersion syscall (#2443)
 * StartWhenSynchronized option for RPC server (defaults to the old behaviour,
   #2445)
 * historic RPC invocations (invokefunctionhistoric, invokescripthistoric and
   invokecontractverifyhistoric APIs, #2431)
 * protocol hardforks, with Aspidochelone being the first supported (#2469,
   #2519, #2530, #2535)
 * MODMUL and MODPOW VM instructions (#2474)
 * ability to get connection closure error from WSClient (#2510)
 * isolated contract calls with state rollback on exception (#2508)
 * vote and candidate state change events in the NEO contract (#2523)
 * immutable compound types in VM (#2525)

Behavior changes:
 * ContractManagement deploy and update methods now require AllowCall flag
   (#2402)
 * GetStorageItems* APIs are no longer available in dao package, use Seek*
   (#2414)
 * committee candidates are now checked against the Policy contract block list
   (#2453)
 * getCandidates NEO method returns no more than 256 results now (#2465)
 * EQUAL checks are limited to 64K of data in VM now irrespective of the
   number of elements compared (#2467)
 * Create[Standard/Multisig]Account prices were raised to avoid DoS attacks
   (#2469)
 * System.Runtime.GetNotifications syscall costs 16 times more GAS now (#2513)
 * System.Runtime.GetRandom syscall now costs more and uses more secure seed
   (#2519)

Improvements:
 * better messages for some CLI commands (#2411, #2405, #2455)
 * fixes and extensions for example contracts (#2408)
 * better neotest documentation (#2408)
 * internal tests now use neotest framework more extensively (#2393)
 * neotest can now be used for benchmark code and has more multi-validator
   chain methods (#2393)
 * big (>64 bit) integers can now be used for RPC calls (#2413)
 * no error is logged now when notary-assisted transaction is already in the
   mempool (it's not an error, #2426)
 * T5 testnet with more aggressive protocol parameters (#2457)
 * destroyed contracts are blocked now (#2462)
 * JSONization errors for invoke* RPCs are now returned in exception field of
   the answer (#2461)
 * typos, grammar and spelling fixes in documentation, comments and messages
   (#2441, #2442)
 * faster RPC client initialization (#2468)
 * RPC processing errors use ERROR log severity now only if there is a
   server-side error occurred (2484)
 * increased server-side websocket message limit to fit any request (#2507)
 * invalid PrepareRequest now doesn't require other nodes to be alive to send
   ChangeView (#2512)
 * updated YAML library dependency (#2527)
 * notary subsystem compatibility fixes, using new IDs and options (#2380)
 * minor performance improvements (#2531)

Bugs fixed:
 * websocket-based RPCs were not counted in Prometheus metrics (#2404)
 * input data escaping missing for RPC log messages (#2404)
 * compiler panic in notification checking code (#2408)
 * missing 'hash' field in the debug data (#2427)
 * debug data used relative paths that are not compatible with neo-debugger
   (#2427)
 * getversion RPC method wasn't compatible with C# implementation (#2435,
   #2448)
 * old BaseExecFee and StoragePrice values could be used by transactions in
   the same block with transactions that change any of them (#2432)
 * context initialization race in dbft (#2439)
 * some stateroot data functions used incorrect keys in the DB (#2446)
 * voter reward data could not be deleted in NEO contract in some cases
   (#2454)
 * the maximum number of HTTPS oracle redirections is limited to 2 (and only
   using HTTPS) for C# compatibility (#2456, #2389)
 * maximum number of contract updates wasn't limited leading to overflow (#2462)
 * incorrect interop signature for getCandidates NEO contract method (#2465)
 * next instruction validitiy check is performed now before instruction
   pointer move to be compatible with C# implementation (#2475)
 * concurrent map access in Seek leading to panic (#2495, #2499)
 * insecure password reads (#2480)
 * minor VM reference counting fixes (#2498, #2502, #2525)
 * panic during serialization of transaction with empty script (#2485)
 * 'exception' field was missing in the invoke* RPC call output when there is
   no exception which differed from C# node behaviour (#2514)
 * interop interfaces used incompatible (wrt C# node) type string in JSON
   (#2515)
 * minor genesis block state differences wrt C# implementation (#2532)
 * incompatible (wrt C#) method offset check (#2532)

## 0.98.5 "Neutralization" (13 May 2022)

An urgent update to fix the same security issue that was fixed in 3.1.0.1 C#
node release. Please upgrade as soon as possible, resynchronization is not
needed for mainnet.

Bugs fixed:
 * GAS emission now happens after NEO transfer is finished (#2488)

## 0.98.4 "Mesmerization" (11 May 2022)

An urgent release to fix incompatibility with mainnet at block 1528989. The
actual pair of problems leading to inability to process this block occurred
earilier than that, so to fix this you need to resynchronize your node. Fixed
node is confirmed to have identical state as 3.1.0 C# node up to block
1529810.

Bugs fixed:
 * StdLib itoa() implementation emitted uppercase letters instead of lowercase
   (#2478)
 * StdLib jsonDeserialize() implementation couldn't handle properly integers
   larger than 64-bit signed (#2478)

## 0.98.3 "Liquidation" (07 May 2022)

This is a hotfix release to fix t4 testnet incompatibility at block
1589202. The actual problem was found and fixed during 0.99.0 development
cycle, but 0.99.0 is expected to be incompatible with t4 testnet. This release
allows to continue working with it as well as mainnet (and contains some other
fixes for known problems). It does not require resynchronizing a node.

Improvements:
 * double call to `WSClient.Close()` method won't cause a panic (#2420)

Bugs fixed:
 * Rules scope considered as invalid in binary representation (#2452)
 * incorrect compressed P2P message could lead to panic (#2409)
 * notary-assisted transaction could be in inconsistent state on the Notary
   node (#2424)
 * WSClient panics if request is made after connection breakage (#2450)
 * Rules scope JSON representation wasn't compatible with C# implementation
   (#2466)
 * JSONized Rules scope could only contain 15 conditions instead of 16 (#2466)

## 0.98.2 "Karstification" (21 Mar 2022)

We've decided to release one more 3.1.0-compatible version bringing all of the
new features and bug fixes (along with complete support for Windows platform)
made before going into full 3.2.0 compatibility. It's important for us to give
you more stable and flexible environment for the ongoing hackathon. Contract
bindings generator might be very useful for anyone operating with already
deployed contracts written in other languages, while the node itself can now
be configured for lightweight operation, keeping only the data relevant for
the past MaxTraceableBlocks, no more and no less. Current public networks
don't benefit from it yet with their large MaxTraceableBlocks setting, but
they're growing every day and it's just a matter of time when this feature
will start making a difference. And sorry, but this release is again faster
than the previous one.

We recommend updating, but if your node is not public and you don't
specifically want to play with new things brought by it you might as well wait
for the next one. It requires resynchronization and 0.99.0 will do too because
of planned protocol changes.

New features:
 * protocol extension allowing to change the number of validators in existing
   network (#2334)
 * MPT garbage collection mechanism allowing to store a set of latest MPTs,
   RemoveUntraceableBlocks setting now activates it by default unless
   KeepOnlyLatestState is used (#2354, #2360)
 * NEP-11/NEP-17 transfer data garbage collection (#2363)
 * Go bindings generator based on contract manifests (#2349, #2359)

Behavior changes:
 * some RPC client functions for divisible NEP-11 changed signatures using
   slice of bytes now instead of strings for IDs (#2351)
 * heavily refactored storage and dao package interfaces (#2364, #2366)
 * support for Go 1.18 added, 1.15 dropped (#2369)

Improvements:
 * additional CLI tests (#2341)
 * `defer` statement is now compiled more effectively (#2345)
 * `defer` statement can be used in conditional branches now (#2348)
 * new tests for divisible NEP-11 contract (#2351)
 * all tests failing on Windows platform are fixed (#2353)
 * RPC functions now accept integers larger than 2^64 from strings (#2356)
 * 10-35%% improvement in bulk block processing speed (node synchronization)
   depending on machine and configuration (#2364)
 * reworked VM CLI fixing Windows incompatibility and UTF-8 bugs (#2365)
 * incompletely signed transaction JSONs now also contain hash (#2368)
 * RPC clients can now safely be used from multiple threads (#2367)
 * additional tests for node startup sequences (#2370)
 * customizable notary service fee for Notary extension (#2378)
 * signers passed into RPC methods can now contain rules (#2384)
 * compiler tests now take less time (#2382)
 * some fuzzing tests added (#2399)

Bugs fixed:
 * test execution result wasn't checked for CLI invocations that saved
   transaction into file (#2341)
 * exception stack wasn't properly cleared during CALL processing in VM (#2348)
 * incorrect contract used in RPC client's GetOraclePrice (#2378)
 * oracle service could be tricked by redirects into going to local hosts when
   this was disabled (#2383)
 * `invoke*` RPC functions could be used to trigger OOM with specially crafted
   scripts (#2386)
 * some edge-cased integer conversions were not exactly matching the behavior
   of C# node in VM (#2391)
 * HASKEY instruction wasn't working for byte arrays (#2391)
 * specially-crafterd JSONPath filters for oracle requests could trigger OOM
   (#2372)
 * ENDFINALLY opcode before ENDTRY could lead to VM panic (#2396)
 * IsScriptCorrect function could panic on specially-crafted inputs (#2397)

## 0.98.1 "Immunization" (31 Jan 2022)

Bug fixes, interesting optimizations, divisible NEP-11 example and a big
compiler update --- everything you wanted to find in this NeoGo update. It
requires chain resynchronization, but this resynchronization will be faster
than ever.

One thing should also be noted, even though this release is 3.1.0-compatible,
it is known to have a different state for testnet after block 975644, but it's
not a NeoGo fault, it'll be fixed in the next C# node release. The root cause
is well-known and is not considered to be critical compatibility-wise.

New features:
 * support for reading the wallet from stdin in CLI (where it's possible, #2304)
 * new CLI command for wallet password change (#2327)
 * `getstateroot` support in RPC client (#2328)
 * divisible NEP-11 example (#2333)
 * helper script to compare node states via RPC (#2339)

Behavior changes:
 * zero balance is explicitly printed now for token-specific NEP-17 balance
   requests from CLI (#2315)
 * pkg/interop (used by smart contracts) is a separate Go module now (#2292,
   #2338)
 * smart contracts must be proper Go packages now (#2292, #2326)

Improvements:
 * optimized application log storage (#2305)
 * additional APIs in `neotest` framework (#2298, #2299)
 * CALLT instruction is now used by the compiler where appropriate leading to
   more optimized code (#2306)
 * DB seek improvements increasing chain processing speed by ~27% (#2316, #2330)
 * updated NeoFS dependencies (#2324)
 * optimized emitting zero-length arrays in `emit` package used to construct
   scripts (#2299)
 * native contract tests refactored using generic contract testing framework (#2299)
 * refactored internal Blockchainer interfaces, eliminating unnecessary
   dependencies and standardizing internal service behavior (#2323)
 * consensus process now always receives incoming transactions which might be
   helpful for accepting conflicting (wrt local pool) transactions or when the
   memory pool is full (#2323)
 * eliminated queued block networked re-requests (#2329)
 * better error reporting and parameter handling in FindStates RPC client
   method (#2328)
 * invoke* RPCs now also return notifications generated during execution (#2331)
 * it's possible to get storage changes from the result of invoke* RPCs (#2331)
 * better transfer data storage scheme resulting in faster getnep* RPC
   processing (#2330)
 * dropped deprecated `loader` package from compiler dependencies moving to
   updated x/tools interface (#2292)

Bugs fixed:
 * incorrect handling of missing user response in CLI (#2308)
 * improper primary node GAS distribution in notary-enabled networks (#2311)
 * excessive trailing spaces in the VM CLI output (#2314)
 * difference in JSON escaping wrt C# node leading to state difference for
   testnet at block 864762 (#2321)
 * potential panic during node shutdown in the middle of state jump (#2325)
 * wrong parameters for `findstates` call in RPC client (#2328)
 * improper call flags in Notary contract `withdraw` method (#2332)
 * incorrect ownerOf signature for divisible NEP-11 contracts (#2333)
 * missing safeness check for overloaded methods in the compiler (#2333)

## 0.98.0 "Zincification" (03 Dec 2021)

We've implemented all of Neo 3.1.0 protocol changes in this release (and
features like NEP-11 transfer tracker), but as usual it's not the only thing
it contains. If you're developing contracts in Go you might be interested in
contract testing framework this release brings with it, which allows you to
write automated tests in Go. You can also build a node for Windows now and
most of functionality works there (but some known problems still exist, so
this port is considered to be experimental for now). As usual there were some
optimizations made in various components as well as bug fixes.

There are also some things removed in this release, that is BadgerDB and
RedisDB support, this will allow us to optimize better for LevelDB and BoltDB
in future. Notice also that this release requires full node resynchronization.

New features:
 * optional signer scope parameter for deploy command (#2213)
 * interop packages extended with Abort() function and useful constants (#2228)
 * notary subsystem now allows to have multiple multisignature signers as well
   as combining simple notary-completed accounts with multisignature ones (#2225)
 * configurable method overloading for the compiler (#2217)
 * initial support for the Windows platform (#2239, #2265, #2267, #2264,
   #2283, #2288)
 * contract testing framework (#2229, #2262)
 * `Rules` witness scope (#2251)
 * PACKMAP/PACKSTRUCT opcodes and UNPACK extension (#2252)
 * NEP-11 transfer tracking with RPC API (#2266)
 * support for structure field slicing in the compiler (#2280)
 * invoked contract tracing for invoke* RPCs (#2270, #2291)

Behavior changes:
 * BadgerDB and RedisDB are no longer available for DB backend (#2196)
 * GetNEP17Transfers() RPC client method now works with Uint160 parameter
   instead of address (#2266)

Improvements:
 * compiler optimizations (#2205, #2230, #2232, #2252, #2256)
 * output fees as proper decimals in the CLI (#2214)
 * private network created from Makefile now uses dynamic IP addresses (#2224)
 * various minor optimizations across whole codebase (#2193, #2243, #2287, #2290)
 * `util convert` command now also handles base64 representations of script
   hashes (#2237)
 * unified "unknown transaction" RPC responses (#2247)
 * faster state switch when using P2P state synchronization extension (#2201)
 * better logging for oracle errors (#2275)
 * updated NeoFS client library dependency (#2276)

Bugs fixed:
 * getproof RPC API didn't handle destroyed contracts correctly (#2215)
 * invokefunction RPC returned error if no parameters were passed (#2220)
 * incorrect handling of empty MPT batch (#2235)
 * incoming block queue using more memory than it should, up to OOM condition
   on stuck node (#2238)
 * panic on peer disconnection in rare cases (#2240)
 * panic on password read failure (#2244)
 * miscompiled functions using `defer` and returning no values (#2255)
 * oracle responses processed not using request witnesses (#2261)
 * CreateTxFromScript() RPC client method incorrectly handled CustomContracts
   and CreateTxFromScript sender scopes (#2272)
 * potential OOM during synchronization on systems with slow disks (#2271)
 * incorrect oracle peer configuration in the default private network (#2278)
 * compiler error processing functions using defer and inlined calls (#2282)
 * node that is simultaneously a CN and Oracle node could create invalid
   oracle response transactions (#2279)
 * incorrect GetInvocationCounter() result in some cases (#2270)

## 0.97.3 "Exception" (13 Oct 2021)

This updated version of NeoGo is made to be compatible with Neo 3.0.3 bringing
with it all corresponding protocol changes and bug fixes so that you can use
it on public testnet and mainnet. It also brings with it a complete
experimental P2P state exchange extension for private networks as well as
additional optimizations. This is the first version requiring at least Go 1.15
to compile, so make sure your Go is up to date if you're not using pre-built
binaries. Protocol updates and bug fixes made in this release require a
resynchronization on upgrade.

We also make a final call on Badger and Redis DB issue #2130, if there are no
active users of them, support for both will be removed in the next release.

New features:
 * MaxConnsPerHost RPC client option (#2151)
 * P2P state exchange extension (#2019)
 * transaction-related CLI commands now show calculated fees before
   sending transaction to RPC node and ask for confirmation (#2180)
 * test invocations are now possible for partially-signed transactions stored
   in JSON format (used for manual signing by multiple parties, #2179)
 * `getstate` and `findstates` RPC to retrieve historic state data (#2207)

Behavior changes:
 * added support for Go 1.17, dropped Go 1.14 (#2135, #2147)
 * blocking native contracts is no longer possible in Policy contract (#2155)
 * `getversion` RPC call now also returns protocol configuration data (#2161,
   #2202)
 * NEF files now have Source field that can be specified in contract's
   metadata (#2191)
 * notification events from contracts in subscription service now also contain
   container (transaction usually) hash (#2192)
 * out of bounds exceptions can be catched in contracts now for PICKITEM and
   SETITEM VM instructions (#2209)

Improvements:
 * reduced number of memory allocations in some places (#2136, #2141)
 * VM optimizations (#2140, #2148)
 * notary subsystem documentation (#2139)
 * documentation fixes (#2162)
 * VM CLI now allows to dump slots (#2164)
 * better IPv6 checks in example NNS contract and `getAllRecords` method (#2166)
 * updated linters (#2177)
 * Migrate methods renamed into Update in examples (#2183)
 * old (no longer available) interop function names removed (#2185)
 * optimized processing of voting NEO transfers (#2186)
 * configuration parameters specified in seconds no longer use (improper)
   time.Duration type (#2194)
 * better error message for conflicting transactions (#2199)
 * open wallet in read-only mode if not changing it (#2184)
 * optimized header requests in P2P communication (#2200)
 * compiler now checks for method existence if it's specified as safe in
   metadata (#2206)

Bugs fixed:
 * incorrect CustomGroups witness scope check (#2142)
 * empty leaf values were ignored for MPT calculcations (#2143)
 * lower-case hexadecimal in block header JSON output (differing from C# node,
   #2165)
 * `getblockheader` RPC result missing `Nonce` and `Primary` fields (#2165)
 * return empty list of unverified transactions for `getrawmempool` RPC
   instead of null (C# node never returns null, #2165)
 * return empty list of NEF tokens in JSON format instead of null which C#
   node never returns (#2165)
 * races in some tests (#2173)
 * parameter context JSON serialization/deserialization incompatiblity with C#
   node leading to interoperability problems (#2171)
 * transfers of 0 GAS or NEO were not possible (#2169)
 * incorrect ContentTypeNotSupported oracle response transaction handling (#2178)
 * JSON escaping differences with C# implementation (#2174)
 * NEO balance update didn't save LastUpdatedBlock before GAS distribution
   leading to problems in transactions with recursive NEO transfers (#2181)
 * panic in CLI transfer command when missing destination address (#2211)
 * multiple blank identifiers in function/method were misinterpreted by the
   compiler (#2204)
 * `getnep17transfers` used uint32 for internal time processing which is not
   enough for ms-precision N3 timestamps (#2212)
 * PICKITEM instruction could fail on attempt to get an item with long key
   from map (#2209)

## 0.97.2 "Dissipation" (18 Aug 2021)

We're rolling out an update for NeoGo nodes that mostly concentrates on
performance. We've tweaked and tuned a lot of code while staying compatible
with N3 mainnet and testnet. At the same time we're gradually introducing
changes required for our P2P state exchange extension and this affected DB
format, so you'll need to resynchronize on update. This update is not
mandatory though, 0.97.1 is still perfectly valid for N3 networks.

Note also that we're discussing removal of Badger and Redis databases support
in future releases, so if you're interested in them take a look at #2130.

Behavior changes:
 * address blocking in Policy contract now also blocks calls to contracts with
   blocked addresses (#2132)

Improvements:
 * numerous memory and CPU optimizations across whole codebase (#2108, #2112,
   #2117, #2118, #2122, #2123, #2128, #2114, #2133)
 * preliminary work for P2P state exchange extension (#2119)
 * `util convert` command now also detects public keys and converts them to
   script hash/address (#2125)

Bugs fixed:
 * key decoding functions could accept some additional data even though only
   key is expected to be present (#2125)
 * conflicting dummy transactions could stay in the DB even with block removal
   enabled (#2134)

## 0.97.1 "Gasification" (06 Aug 2021)

We're updating NeoGo to make it compatible with the latest protocol changes
made in 3.0.2 version of C# node. But that's not the only thing we do, this
release also fixes one important bug, improves node's performance and adds CLI
support to add group signatures to manifests.

It requires resynchronization on upgrade.

New features:
 * `contract manifest add-group` command to add group signatures to contract
   manifest (#2100)

Behavior changes:
 * GAS contract no longer has Refuel method, it could be used as DOS
   amplification tool for attacks on network and there is no way to securely
   fix it (#2111)

Improvements:
 * memory store optimizations leading to substantial single-node TPS
   improvements (#2102)
 * various micro-optimizations across the board both for CPU usage and memory
   allocations (#2106, #2113)
 * optimized transaction decoding (#2110)

Bugs fixed:
 * ping messages created with wrong value used for node's height (#2115)

## 0.97.0 "Ventilation" (02 Aug 2021)

This is an official 3.0.0-compatible release that is ready to be used both for
mainnet and testnet. Technically, 0.96.0 and 0.96.1 are compatible too, but
they need an updated configuration to work on mainnet while this version has
it covered.

We keep improving our node and this release is not just a repackage of
something older, so DB format changes require a resynchronization if you're
upgrading from 0.96.X.

Behavior changes:
 * updated configuration for mainnet (#2103)

Improvements:
 * documentation for contract configuration file (#2097)
 * significant change to NEP-17 tracking code, it shouldn't affect valid
   NEP-17 tokens, but now we store a little less data in the DB and get more
   from token contracts when needed (for `getnep17balances` RPC for
   example); this change is required for our future state exchange protocol
   extension (#2093)
 * improved block processing speed on multicore systems (#2101)
 * JSON deserialization now has the same limits as binary, but this doesn't
   affect any valid code (#2105)

Bugs fixed:
 * potential deadlocks in notary-enabled nodes (#2064)
 * wallet files not truncated properly on key removal (#2099)

## 0.96.1 "Brecciation" (23 Jul 2021)

New CLI commands, updated dependencies and some bugs fixed --- you can find
all of this in the new NeoGo release. It's compatible with 0.96.0 (except for
multisignature contexts, but you're not likely to be using them) and
confirmed to have proper RC4 testnet state up to 15K blocks (but 0.96.0 is
fine wrt this too). At the same time we recommend to resynchronize the chain
if you're using LevelDB or BoltDB, both databases were updated.

New features:
 * `query candidates`, `query committee` and `query height` CLI commands
   (#2090)
 * `GetStateHeight` RPC client call support (#2090)

Behavior changes:
 * `wallet candidate getstate` command was renamed to `query voter`, now it
   also prints the key voted for and handles addresses with 0 balance without
   spitting errors (#2090)

Improvements:
 * `query tx` now outputs signer address along with script hash (#2082)
 * updated many of node's dependencies including BoltDB and LevelDB, there are
   no radical changes there, mostly just some fixes improving stability (#2087)
 * better error messages in some places (#2089, #2091)

Bugs fixed:
 * `query tx` command wasn't providing correct results if used with C# RPC
   node (#2082)
 * watch-only node (with a key, but not elected to participating in consensus)
   could panic on receiving a number of Commit messages (#2083)
 * `getstateheight` RPC call produced result in different format from C# node
   (#2090)
 * JSON representation of multisignature signing context wasn't compatible
   with C# implementation (#2092)

## 0.96.0 "Aspiration" (21 Jul 2021)

We're updating NeoGo to support RC4 changes, there are some incompatible ones
so this version can't be used with RC3 networks/chains (RC3 testnet is
still available, please use 0.95.4 for it). This release was checked for RC4
testnet compatibility and has the same state as C# for the first 5K blocks.

No significant protocol changes are expected now as we're moving to final N3
release, but we'll keep updating the node fixing bugs (if any), improving
performance and introducing NeoGo-specific features.

New features:
 * "System.Runtime.GetNetwork" system call (#2043)
 * ContentTypeNotSupported oracle return code and content type configuration
   for Oracle service (#2042)
 * block header have "Nonce" field again which is used for
   "System.Runtime.GetRandom" system call (#2066)
 * import from incremental chain dumps (#2061)
 * configurable initial (from the genesis block) GAS supply (#2078)
 * "query tx" CLI command to check for transaction status (#2070)

Behavior changes:
 * verification GAS limits were increased (#2055)
 * SQRT price reduced (#2071)
 * binary deserialization is now limited to 2048 (MaxStackSize) items (#2071)
 * notary deposit is stored now as serialized stack item (#2071)
 * testnet and mainnet configuration updates (#2077, #2079, #2081)

Improvements:
 * faster stack item binary and JSON serialization (#2053)
 * faster Storage.Find operation (#2057)
 * additional documentation for public network CNs (#2068)
 * better code reuse for native contract's data serialization (#2071, see new
   stackitem.Convertible interface)
 * some types from state package renamed to remove "State" word (#2075)
 * util.ArrayReverse helper moved to package of its own with additional
   helpers (#2075)

Bugs fixed:
 * nested structure cloning could lead to OOM (#2054, #2071)
 * addresses were not accepted for witness accounts in RPC calls (#2072)
 * POW instruction could take unknown amount of time to complete for big
   parameters (#2060)
 * oracle contract could accept invalid user data (#2071)
 * EQUAL for deeply nested structures could never complete (#2071)
 * arrays and structures were limited to 1024 items (#2071)
 * oracle fallback transactions could try using outdated ValidUntilBlock value
   (#2074)
 * oracle node starting from genesis block tried to process all requests from
   the chain instead of only working with current ones (#2074)
 * some tests used non-unique/wrong temporary files and directories (#2076)

## 0.95.4 "Yatter" (09 Jul 2021)

Making a fully compliant Neo node is not easy, there are lots of minuscule
details that could be done in a little different way where both ways are
technically correct but at the same time just different which is enough to
create some interoperability problems. We've seen that with Legacy node
implementation where we were functionally complete with 0.74.0 release but
some fixes kept coming up to the most recent 0.78.3 version. It was a bit
easier with Legacy because we already had some years-long chains to test the
node against while N3 is a completely new territory.

So we'd like to thank all RC3 hackathon participants as well as all other
people playing with N3 RC3 network, your joint efforts gave enough material
to keep us busy fixing things for some time. The chain moving forward hit more
and more edge cases with every block and this is actually very helpful, we saw
a lot of things that could be improved in our node and we improved them.

Now we're releasing 0.95.4 with all of these fixes which is still
RC3-compatible and it's stateroot-compatible with C# implementation up to 281K
blocks. Please resynchronize to get identic testnet state. And most likely
that's the last RC3-compatible release as we're moving forward to the official
3.0.0 release of C# version NeoGo will be updated with appropriate changes
(some of which are not compatible with RC3).

New features:
 * 'sysgas' parameter for invocation and transfer CLI commands to manually add
   some system fee GAS (#2033)

Behavior changes:
 * contract calls are now checked against specified permissions by the
   compiler (#2025)
 * stack items with nesting levels of 10 and more won't be serialized to JSON
   for purposes like including them into RPC call reply (#2045)

Improvements:
 * AddHeaders() with multiple headers added at the same time now works with
   StateRootInHeader option (#2028)
 * cached GAS per vote value leads to substantially improved block processing
   speed (#2032)
 * failed native NEP-17 transfers now still re-save the old value to the
   storage making state dumps compatible with C# (#2034)
 * refactored and renamed some stackitem package functions, added proper error
   values in all cases (#2045)
 * RPC calls (and application logs) can now return real serialization errors
   if there are any (previously all errors were reported as "recursive
   reference", #2045)
 * better tests (#2039, #2050)

Bugs fixed:
 * wrong onNEP11Payment name in contract configuration examples (#2023)
 * missing permission sections in contract configuration examples (#2023)
 * buffer stack item deserialization created byte string (#2026)
 * Extra contract manifest field serialization was reworked to match C#
   implementation more closely (#2021)
 * wildcard permission in manifest allowed any methods, while it shouldn't
   (#2030)
 * group permission in manifest worked incorrectly for a set of groups (#2030)
 * call flags were JSONized in incompatible way (#2041)
 * MPT update left branch nodes with 1 child in some cases (#2034, #2047)
 * JSON marshalling code for stack items detected item recursion in some cases
   where no actual recursion was present (#2045)
 * binary serialization code for stack items could overwrite old errors in
   some cases (#2045)
 * binary serialization code for stack items wasn't ensuring maximum size
   limit during serialization which could lead to OOM (#2045)
 * batched MPT update create excessive extension nodes for the last child of
   branch node in some cases (#2047)
 * in some cases "TCP accept error" was logged during node shutdown (#2050)
 * contract updating code was updating caches before they should've been
   updated (#2050)
 * INVERT/ABS/NEGATE unary operations were not copying their arguments (#2052)

## 0.95.3 "Yuppification" (17 Jun 2021)

One more N3 RC3-compatible release that fixes testnet state difference at
block 151376. Please resynchronize to get proper testnet state.

Behavior changes:
 * NEP2-related functions in `crypto/keys` package changed a bit to allow
   Scrypt parameters overriding, standard parameters are available via
   `NEP2ScryptParams` function (#2001)

Improvements:
 * better unit test stability (#2011, #2001)
 * updated neofs-api-go dependency (with support for TLS-enabled NeoFS node
   connections, #2003)
 * removed annoying token matching warning (#2018)

Bugs fixed:
 * state mismatch resulting from different committee candidate sorting (#2017)

## 0.95.2 "Echolocation" (10 Jun 2021)

This is another N3 RC3-compatible release and it's better in its RC3
compatiblity than the previous one because we've fixed some state mismatches
wrt C# implementation that were found on testnet. It is confirmed to have the
same state up to 126K height (which is current), but to get proper state you
need to resynchronize your node from the genesis.

New features:
 * RPC notification subsystem was extended to support notary pool events, so
   clients can now react on request additions (#1984)
 * compiler now checks notification event name length to fit into the limit
   (#1989)
 * compiler now also checks for runtime.Notify() calls from Verify method
   (which won't work anyway, #1995)

Improvements:
 * codegeneration improvements removing some unnecessary store/loads and type
   conversions (#1881, #1879)
 * additional MPT tests added ensuring compatibility with C# implementation
   (#1993)
 * additional consistency check added to prevent node running with native
   contracts differing from the ones saved in DB (#2010)

Bugs fixed:
 * `calculatenetworkfee` RPC result used format different from C# node (#1998)
 * `CALLT` opcode was missing any price leading to wrong GAS calculations and
   different state wrt C# node (#2004)
 * '+' character was emitted directly by `jsonSerialize` method which is fine
   wrt JSON itself, but differs from C# node behavior leading to node state
   difference (#2006)
 * NEO self-transfers were not checking the amount transferred (they didn't
   change balance, but they succeeded) leading to state difference wrt C#
   implementation (#2007)

## 0.95.1 "Shiftiness" (31 May 2021)

Bringing NeoGo up to date with N3 RC3 changes this release also improves
compiler and CLI a bit.

This release is mostly compatible with 0.95.0, but you need to resynchronize
your chains to have proper stateroot data and to make new methods available to
contracts. At the same time there won't be long-term support provided for it,
just as with all previous previews and RC.

New features:
 * base58CheckEncode/base58CheckDecode methods in StdLib native contract
   (#1977, #1979)
 * getAccountState method in NeoToken native contract (#1978, #1986)
 * custom contracts and groups can now be specified for witness scopes in
   invocations from CLI (#1973)

Improvements:
 * local variable handling was refactored in the compiler, removing some
   duplicate code and improving robustness (#1921)
 * CLI help now describes how "unvote" can be done (#1985)

Bugs fixed:
 * boolean parameters to function invocations via RPC were not processed
   correctly (#1976)
 * VM CLI used too restrictive default call flags for loaded scripts (#1981)
 * IPv6 check in NNS contract was out of date wrt C# implementation (#1969)

## 0.95.0 "Sharpness" (17 May 2021)

This version mostly implements N3 RC2 protocol changes (and is fully
RC2-compatible), but also brings NEP-11 CLI support and small improvements for
node operators.

Please note that this release is incompatible with 0.94.1 and there won't be
long-term support provided for it.

New features:
 * CLI command for NEP-17 transfers now accepts `data` parameter for the
   transfer (#1906)
 * contract deployment CLI comand now also accepts `data` parameter for
   `_deploy` method (#1907)
 * NEP-11 and NEP-17 transfers from CLI can now have multiple signers (#1914)
 * `System.Runtime.BurnGas` interop was added to burn some GAS as well as
   `refuel` GAS contract method to add GAS to current execution environment
   (#1937)
 * port number announced via P2P can now differ from actual port node is bound
   to if new option is used (#1942)
 * CLI now supports full set of NEP-11 commands, including balance and
   transfers (#1918)
 * string split, memory search and compare functions added to stdlib (#1943)
 * MaxValidUntilBlockIncrement can now be configured (#1963)

Behavior changes:
 * `data` parameter is now passed in a different way to NEP-17 RPC client
   methods (#1906)
 * default (used if nothing else specified) signer scope is now
   `CalledByEntry` in CLI and RPC (#1909)
 * `SignAndPushInvocationTx` RPC client method now adds as many signatures as
   it can with the wallet given which in some cases allows CLI to create
   complete transaction without going through multisignature procedure (#1912)
 * `getversion` RPC call now returns network magic number in `network` field
   (#1927)
 * RoleManagement native contract now emits `Designation` event in
   `designateAsRole` method (#1938)
 * `System.Storage.Find` syscall now strips full prefix given when
   `FindRemovePrefix` option is used (#1941)
 * LT, LE, GT, GE VM opcodes now accept Null parameters (#1939)
 * `features` field was re-added to contract manifests, though it's not
   currently used (#1944)
 * node will reread TLS certificates (if any configured) on SIGHUP (#1945)
 * contract trusts are now expressed with permission descriptors in manifest
   (#1946)
 * NEP-11 transfers now also support `data` parameter (#1950)
 * N3 RC2 testnet magic differs from N2 RC1 testnet (#1951, #1954)
 * stdlib encoding/decoding methods now only accept inputs no longer than 1024
   bytes (#1943)
 * `System.Iterator.Create` interop was removed with all associated logic (#1947)
 * `Neo.Crypto.CheckSig` and `Neo.Crypto.CheckMultisig` interops were renamed
   to `System.Crypto.CheckSig` and `System.Crypto.CheckMultisig` (#1956)
 * oracle requests now use Neo-specific JSONPath implementation (#1916)
 * native NNS contract was removed and replaced by non-native version (#1965)

Improvements:
 * RPC errors reported by server are now more verbose for `submitblock`,
   `sendrawtransaction` calls (#1899)
 * all CLI commands that accept addresses now also accept hashes and vice
   versa (#1905)
 * code cleanup based on a number of static analysis tools (#1908, #1958)
 * CLI implementation refactoring (#1905, #1911, #1914, #1915, #1918)
 * only one state validator now sends complete stateroot message normally
   (#1953)
 * oracle HTTPS requests are now sent with User-Agent header (#1955)
 * stdlib `itoa` and `atoi` methods can now be called with one parameter
   (#1943)
 * oracle nodes are no longer on extensible payload whitelist (#1948)
 * extensible message pool is now per-sender with configurable size (#1948)
 * `static-variables` field support for debugger as well as debug data for
   `init` and `_deploy` functions (#1957)
 * user documentation for configuration options (#1968)

Bugs fixed:
 * `getproof` RPC request returned successful results in some cases where it
   should fail
 * `Transfer` events with invalid numbers were not rejected by NEP-17 tracking
   code (#1902)
 * boolean function parameters were not accepted by `invokefunction` RPC call
   implementation (#1920)
 * potential races in state validation service (#1953)
 * single state validator couldn't ever complete stateroot signature (#1953)
 * SV vote resending was missing (#1953)
 * SV vote messages used invalid (too big) ValidBlockEnd increment (#1953)
 * memory leak in state validation service (#1953)
 * NEP-6 wallets have `isDefault` field, not `isdefault` (#1961)

## 0.94.1 "Channelization" (08 Apr 2021)

This is the second and much improved N3 RC1-compatible release. We've mostly
focused on documentation and examples with this release, so there is a number
of updates in this area including oracle contract example and NEP-11 NFT
contract example. At the same time proper testnet network revealed some
implementation inconsistencies between NeoGo and C# especially in oracle and
state validation services, so there are fixes for them also.

Protocol-wise this release is compatible with 0.94.0, but MPT structures have
changed and there are known state change differences for N3 RC1 testnet, so
you need to do full node resynchronization on update from 0.94.0. Some SDK
APIs have changed also improving developer experience, but they may affect
your code. We don't plan to make more of RC1-compatible releases (the protocol
has slightly changed already since the release).

New features:
 * RPC (*Client).GetDesignatedByRole method to easily get node lists from
   RoleManagement contract (#1855)
 * `calculatenetworkfee` RPC method support (#1858)
 * RPC client now has additional methods for NEP-11 contracts and specifically
   for NNS native contract (#1857)
 * contract deployment from multisig addresses (#1886)

Behavior changes:
 * node roles for RoleManagement contract were moved into separate package
   (#1855)
 * NotaryVerificationPrice constant was moved into package of its own (#1855)
 * testnet configuration now has proper N3 RC1 testnet magic (#1856)
 * crypto.Verifiable interface was removed, now any hash.Hashable thing can be
   verified (and signed, #1859)
 * Network field was dropped from transaction.Transaction, block.Header,
   payload.Extensible, state.MPTRoot, payload.P2PNotaryRequest and
   network.Message, it was needed for proper hash calculations before recent
   protocol changes (even though this field is not a part of serialized
   representation of any of these elements), but now it's only used to
   sign/verify data and doesn't affect hashing which allowed to simplify
   interface here (#1859)
 * RPC server now returns `StateRootInHeader` option in its `getversion`
   request answers if it's used by the network (#1862)
 * NNS record types were moved to separate package (#1857)
 * MaxStorageKeyLen and MaxStorageValueLen constants were moved to storage
   package from core (#1871)
 * oracle service now accepts complete URL for other nodes RPC services (#1877)

Improvements:
 * RPC (*Client).AddNetworkFee method is now more informative in some error
   case (#1852)
 * proper NEP-17 support in unit test contract (#1852)
 * documentation updates for examples and services (#1854, #1890)
 * syscall number is now printed for failed syscalls (#1874)
 * better logging (#1883, #1889)
 * Oracle native contract interop documentation extended (#1884)
 * Oracle native contract interop extended with return codes and constants (#1884)
 * oracle smart contract example (#1884)
 * NEP-11 NFT smart contract example (#1891)

Bugs fixed:
 * node could simultaneously try to connect to the same peer multiple times in
   some cases (#1861)
 * uniqueness is enforced now for other node addresses provided by peers (#1861)
 * stateroot message hash wasn't calculated the same way as in C# (#1859)
 * state validation service P2P messages were not signed properly (#1859)
 * state.MPTRoot structure JSON representation was different from C# one (#1859)
 * stateroot messages generated by state validation service were not setting
   proper message category (#1866)
 * block persistence caches were flushed too early by MPT-managing code (#1866)
 * validated stateroot data overwrote local one which lead to node not
   functioning after restart if states were different (#1866)
 * peers delivering P2P message with validated stateroot differing from local
   one were disconnected (#1866)
 * state dump comparing script was using old Ledger native contract ID (#1869)
 * candidate registration check was made in different way from C# in native
   NEO contract leading to different execution results for erroneous voting
   transactions (#1869)
 * state change dumps were different from C# node for erroneous voting
   transactions even though contract's state was the same (#1869)
 * arguments were not completely removed from stack in erroneous
   Runtime.Notify calls leading to different stack state with C#
   implementation (#1874)
 * in case contract failed execution with THROW the node didn't print the
   message from the contract properly (#1874)
 * `getproof` RPC call output was not strictly compliant with C#
   implementation (#1871)
 * MPT node serialization and some constants were out of date wrt C#
   implementation (#1871)
 * native NEP-17 contracts could use stale balance data in some cases (#1876)
 * state validation service could lock the system during shutdown, preventing
   proper node exit (#1877)
 * state validation and oracle services were started before node reaching full
   synchronization which could lead to excessive useless traffic (#1877)
 * some native contract calls in function arguments could be miscompiled
   (#1880)
 * oracle service was accepting http URLs instead of https (#1883)
 * neofs URIs were subject to host validation in oracle service even though
   there is no host there (#1883)
 * neofs URI scheme was differing from C# implementation (#1883)
 * no default value was used for NeoFS request timeout if it's not specified
   in the configuration (#1883)
 * oracle response code was marshalled as integer into JSON (#1884)
 * MPT could grow in memory unbounded (#1885)
 * Storage.Find results could differ from C# in some cases (#1888)

## 0.94.0 "Tsessebe" (19 Mar 2021)

N3 RC1-compatible release is here. We've implemented all Neo protocol changes
(including state validation service and NeoFS support for oracles) and are
ready for testnet launch now, so that you could play with new native
contracts, VM instructions and other goodies RC1 brings with it. Some
usability improvements and documentation updates also went into this release
as well as a number of fixes stabilizing Notary subsystem (which is
NeoGo-specific protocol extension).

Please note that this release is incompatible with 0.93.0. We do plan to make
an update soon (with more examples and documentation), but there won't be
long-term support provided, Neo N3 is still on its way to mainnet (although
RC1 and testnet are major milestones on this route).

New features:
 * Compiler:
   - ellipsis support for append() to non-byte slices (#1750)
   - NEP-11 standard conformance check (#1722)
   - onNEP*Payable compliance checks (#1722)
 * you can pass files as contract arguments now with `filebytes` CLI parameter
   (#1762)
 * CLI now supports specifying addresses everywhere script hashes are accepted
   (#1758)
 * System.Contract.CreateMultisigAccount interop function (#1763)
 * SQRT and POW VM instructions (#1789, #1796)
 * new NeoFSAlphabet node role added to RoleManagement contract (#1812)
 * node can serve as state validator node (#1701)
 * oracles now support NeoFS (#1484, #1830)
 * CLI can be used to dump wallet's public keys (#1811)
 * storage fee, candidate register price and oracle request price can now be
   adjusted by committee (#1818, #1850)
 * native contracts can now be versioned (#1827)
 * RPC client was extended with price getters for native contracts (#1838)

Behavior changes:
 * NFTs no longer have "description" field (#1751)
 * P2P Notary service configuration moved to ApplicationConfiguration section
   (#1747)
 * native contract methods requiring write permission in call flags now also
   require read permission (#1777, #1837)
 * System.Contract.Call interop function now requires state read permission
   (#1777)
 * NeoGo no longer supports Go 1.13 (#1791)
 * native contract calls are now contract version aware (#1744)
 * interop wrappers for smart contracts now use `int` type for all integers
   (#1796)
 * MaxTransactionsPerBlock, MaxBlockSize, MaxBlockSystemFee settings are now a
   part of node configuration and no longer are available in Policy contract
   (#1759, #1832)
 * storage items can no longer be constant (#1819)
 * state root handling is now conformant with C# implementation (with state
   validators and vote/stateroot messages, #1701)
 * blocks no longer have ConsensusData section, primary index is now a part of
   the header (#1792, #1840, #1841)
 * `wallet multisign sign` command was renamed to `wallet sign`, it now works
   not just for multisignature contract signing, but also for multiple regular
   signers as well as contract verification signing
 * conversion interops were moved to StdLib native contract (#1824)
 * crypto interops (except basic `CheckSig` and `CheckMultiSig`) were moved to
   CryptoLib native contract (#1824)
 * PACK, UNPACK and CONVERT instructions now cost more (#1826)
 * some native contract types were adjusted (#1820)
 * native Neo's `getCommittee` and `getNextBlockValidators` methods now cost
   less (#1828)
 * block/transaction/payload hashing and signing scheme has changed (#1829,
   #1839)
 * signing context is now serialized to JSON using base64 for data (#1829)
 * System.Contract.IsStandard interop was removed (#1834)
 * notifications are no longer allowed for safe contract methods (#1837)

Improvements:
 * verification script are now allowed to make contract calls (#1751)
 * extensible payloads now have the same size limit as other P2P messages
   (#1751)
 * error logging is more helpful now in some cases (#1747, #1757)
 * function inlining support in compiler for internal intrinsics, interop
   refactoring (#1711, #1720, #1774, #1785, #1804, #1805, #1806, #1796, #1809)
 * documentation updates (#1778, #1843)
 * `sendrawtransaction` and `submitblock` RPC calls now return more detailed
   information about errors for failed submissions (#1773)
 * NeoGo now supports Go 1.16 (#1771, #1795)
 * NEP-17 transfer tracking was optimized to avoid some DB accesses (#1782)
 * interop wrappers added for SIGN/ABS/MAX/MIN/WITHIN instructions (#1796)

Bugs fixed:
 * `designateAsRole` method returned value while it shouldn't (#1746)
 * deleting non-existent key could lead to node panic during MPT calculation
   (#1754)
 * some invalid IPv4 were accepted by name service contract (#1741)
 * some legitimate IPv6 addresses were rejected by name service (#1741)
 * compiler: append() to byte array produced wrong results for bytes with
   >0x80 values (#1750)
 * wrong notary-signed transaction size in notifications leading to client
   disconnects (#1766)
 * CALLT wasn't checking permission to read states and make calls in call
   flags (#1777)
 * proper escaping wasn't done in some cases on VM stack item conversion to
   JSON (#1794)
 * improper network fee calculation for complex multi-signed transactions in
   some cases (#1797)
 * importing package with the same name as compiled one could lead to
   incorrect compiler behavior (#1808)
 * boolean values were emitted as integers by compiler leading to some
   comparison failures (#1822)
 * `invokecontractverify` wasn't calculating proper GAS price of verification
   using contract (#1825)
 * ContractManagement contract wasn't returning NULL value from `getContract`
   method if contract is missing (#1851)

## 0.93.0 "Hyperproduction" (12 Feb 2021)

This is a 3.0.0-preview5 compatible release with important protocol changes,
improved smart contract interop interface for native contracts (it's much
easier now to use them) and complete Notary subsystem which is a NeoGo
experimental protocol extension for P2P signature collection.

Please note that this release is incompatible with 0.92.0 and there are no
plans for long-term support of it, Neo 3 is still changing and improving.

The release is tested with preview5 testnet data (and one of testnet CNs is
already a neo-go node) up to 48K blocks and it has exactly the same storage
data except for the Ledger contract that technically can differ between nodes
(it's not a part of state proper), and in case of neo-go it's intentionally
different (but this doesn't affect contract's functionality and state roots
compatibility).

New features:
 * "ProtocolNotSupported" oracle response code (#1630)
 * POPITEM VM opcode (#1670)
 * CALLT VM opcode and contract tokens support (#1673)
 * extensible P2P payloads (#1667, #1674, #1706, #1709)
 * contract method overloading (#1689)
 * oracle module (#1427, #1703)
 * NNS native contract (#1678, #1712, #1721)
 * complete P2P Notary subsytem (experimental protocol extension, use only on
   private networks, #1658, #1726)
 * Ledger native contract (#1696)
 * `getblockheadercount` RPC call (#1718)
 * native contract wrappers for Go smart contracts (#1702)
 * `getnativecontracts` RPC call (#1724)

Behavior changes:
 * VM CLI now supports and requires manifests to properly work with NEF files
   (#1642)
 * NEP-17-related CLI commands now output GAS balance as floating point numbers
   (#1654)
 * `--from` parameter can be omitted now for NEP-17 CLI transfers, the default
   wallet address will be used in this case (#1655)
 * native contracts now use more specific types for methods arguments (#1657)
 * some native contract names and IDs have changed (#1622, #1660)
 * NEF file is stored now in contract's state instead of script only which
   also affects RPC calls and ContractManagement's interface
 * call flag definitions were moved to a package of their own (#1647)
 * callback syscalls were removed (#1647)
 * calling a contract now requires specifying allowed flags (and `CallEx`
   syscall was removed, #1647)
 * iterator/enumerator `Concat` calls were removed (#1656)
 * `System.Enumerator.*` syscalls were removed (#1656)
 * `System.Storage.Find` interface was reworked (#1656)
 * NEF file format was changed merging compiler and version fields, adding
   reserved fields and tokens (#1672)
 * registering as committee/validator candidate now costs 1000 GAS (#1684)
 * dbft now uses extensible P2P payloads (#1667)
 * contract hashing scheme was changed now including names in the mix (#1686)
 * GAS fees JSON marshalling was changed to plain integers (#1687, #1723)
 * native methods requiring committee signature now fail the script instead of
   returning `false` (#1695)
 * native ContractManagement contract now has two `deploy` methods, with
   additional data and without it (#1693)
 * updated contract manifest now can't change contract's name (#1693)
 * default values of native contracts were moved to storage from code (#1703)
 * blocked accounts now store empty string instead of `01` byte (#1703)
 * testnet magic number was changed for preview5 (#1709, #1715)
 * `onPayment` method was renamed to `onNEP17Payment` (#1712)
 * manifests are stored and accessed as stack items instead of JSON (#1704)
 * zero-length "Headers" packet is a protocol error now (#1716)
 * `getstorage` RPC call now uses base64 instead of hex for input/output
   (#1717)
 * state dumper and comparer now use base64 instead of hex (#1717)
 * deployed contracts and manifests are now checked for correctness (#1729)
 * transaction scripts are now checked for correctness before execution
   (#1729)

Improvements:
 * default VM interop prices were adjusted (#1643)
 * batched MPT updates (#1641)
 * you can use `-m` now for manifest parameter of contract deploying command
   (#1690)
 * transaction cosigners can be specified with addresses now (#1690)
 * compiler documentation was updated (#1690)

Bugs fixed:
 * oracle response transaction can't be correctly evicted from the mempool
   (#1668)
 * the node left with zero peers didn't initiate reconnections to seeds in rare
   cases (#1671)
 * native contracts supposed to check for committee witness in fact checked
   for validators witness (#1679)
 * it was allowed for contracts to have non-bool `verify` methods (#1689)
 * previous proposal reuse could lead to empty blocks accepted even if there
   are transactions in the mempool (#1707)
 * NEO contract's `getCommittee` method name was misspelled (#1708)
 * CLI wasn't correctly handling escape sequences (#1727)
 * init and deploy methods could be misoptimized leading to execution failures
   (#1730)
 * NEP-17 contract example was missing `onNEP17Payment` invocation (#1732)
 * missing state read privilege could lead to transaction failures for
   transfers from CLI or when using native contract wrappers (#1734, #1735)
 * some native contract methods had different parameter names and return
   output types (#1736)

## 0.92.0 "Fermentation" (28 Dec 2020)

NeoGo project is closing year 2020 with 3.0.0-preview4 compatible release that
also has much improved performance, a lot of updates to compiler and SDK and
some experimental protocol extensions. This release also is the most tested
release of NeoGo ever, we've reached 83% code coverage (Neo 2.x only has
66%).

Please note that this release is incompatible with 0.91.0 and there are no
plans for long-term support of it, Neo 3 is still changing and improving.

Protocol-wise this release is tested with preview4 testnet (including working
in consensus with C# nodes) and it is compatible with it even though there are
some known storage change mismatches between NeoGo and C# (functionally the
contents is the same, this mismatch is caused by JSON handling differences and
needs to be addressed at the protocol level).

New features:
 * "high priority" transaction attribute (#1341)
 * "oracle response" transaction attribute (#1407, #1520, #1573)
 * new GAS distribution and committee update scheme (#1364, #1529, #1518,
   #1542, #1549, #1636, #1639)
 * oracle native contract (#1409, #1432, #1474, #1476, #1521, #1528, #1548,
   #1556, #1625)
 * designation native contract (#1451, #1504, #1519)
 * support for `_deploy` method (#1452, #1466)
 * `MaxTraceableBlocks` parameter can now be configured (#1520)
 * `KeepOnlyLatestState` configuration option to drop old MPT data (#1553)
 * `StateRootInHeader` configuration option to include state root data in the
   header (it's a protocol extension, so use with care, #1500)
 * NEP-5 was replaced by NEP-17 (#1558, #1574, #1617, #1636)
 * `RemoveUntraceableBlocks` configuration option to perform old (unreachable)
   block and transaction purging (#1561)
 * stable deployed contract hashes (#1555, #1616, #1628)
 * `Safe` method flag for contract manifests (#1597, #1607)
 * native Management contract to deploy/update/destroy contracts (#1610,
   #1624, #1636)
 * Compiler/SDK:
   - base58 encoding/decoding interops for smart contracts (#1350, #1375)
   - you can use aliases now when importing interop packages (#397)
   - `+=` operator is now supported for strings (#1351)
   - `switch` without statement support (#1355)
   - `make()` support for maps and slices using basic types (#1352)
   - `copy()` support for byte slices (#1352, #1383)
   - `iota` support (#1361)
   - support for basic function literals (#1343)
   - new variable initialization is now supported in `if` statements (#1343)
   - `defer` is now supported (#1343)
   - `recover()` support for exception handling (#1343, #1383)
   - `delete()` support (#1391)
   - `util.Remove()` function is added for smart contracts to remove elements
     from slice (#1401)
   - `interop` package now provides specific type aliases like Hash160/Hash256
     for smart contracts to use (it affects generated manifests, #1375, #1603)
   - variables and function calls support in struct and slice literals (#1425)
   - atoi/itoa encoding/decoding interops for smart contracts (#1530, #1533)
   - contracts specifying NEP-17 as supported standard are now checked for
     interface compliance (#1373)
   - `contract.CallEx` function is now available to contracts that allows
     specifying flags for contract called (#1598)
 * CLI:
   - support for array parameters in invocations (#1363, #1367)
   - contract addresses can now be imported into wallet (#1362)
   - deploying/invoking from multisignature accounts (#1461)
 * RPC:
   - `getcommittee` RPC call (#1416)
   - limits and paging are now supported for `get*transfers` calls (#1419)
   - `getunclaimedgas` RPC call now also supports passing address as a
     parameter (#1418)
   - `getcontractstate` now also accepts addresses, IDs and names (for native
     contracts, #1426)
   - batched JSON-RPC requests support (#1514)
   - `invokecontractverify` RPC call to run verification scripts (#1524)
 * P2P notaries subsystem (configurable `P2PSigExtensions` protocol extension,
   use with care):
   - optional `NotValidBefore` transaction attribute (#1490)
   - optional `Conflicts` transaction attribute (#1507)
   - native Notary contract (#1557, #1593)
   - notary request P2P payload (#1582)

Behavior changes:
 * converting items to boolean now fail for strings of bytes longer than 32
   (#1346)
 * consensus messages now use uint8 field for validator indexes (#1350)
 * maximum possible try-catch nesting is now limited to 16 levels (#1347)
 * maximum manifest size increased to 64K (#1365, #1555)
 * required flags changed for many interop functions (#1362)
 * compiler no longer generates code to log `panic()` message (#1343)
 * `GetBlock`, `GetTransaction` and similar interop functions now return
   pointers to structures (#1375)
 * calling script hash now also is accounted for by CheckWitness interop
   (#1435, #1439)
 * CLI is using `--address` parameter everywhere it's needed (some commands
   used `--addr` previously (#1434)
 * VM now restricts comparable byte array size to 64K (#1433)
 * `FeeOnly` witness scope was renamed to `None` (#1450)
 * `getvalidators` RPC call was renamed to `getnextblockvalidators` (#1449)
 * `emit.Opcode` is now `emit.Opcodes`, allowing for variadic specification of
   opcodes (#1452)
 * `CalculateNetworkFee` was moved to `fee.Calculate` (#1467)
 * fault exception string is now returned for failed invocations (#1462)
 * `runtime.GetInvocationCounter` no longer errors (#1455)
 * `invoke*` RPC calls now also return `transaction` field (#1418)
 * `getversion` RPC call now also returns network magic number (#1489)
 * RPC calls now return data in base64 instead of hex (#1489, #1525, #1537)
 * blocked accounts interface and storage was changed in Policy contract (#1504)
 * voting fee is lower now (#1510)
 * blocks are now processed with two execution triggers, `OnPersist` and
   `PostPersist`, `getapplicationlog` RPC call was updated to support passing
   trigger type (#1515, #1531, #1619)
 * storage fee formula has changed (#1517)
 * `MaxValidUntilBlockIncrement` is now 5760 instead of 2102400 (#1520)
 * Policy contract no longer saves default values during initialization
   (#1535)
 * opcode pricing was changed and now it's adjustable (#1538, #1615)
 * contracts no longer have `IsPayable` (see NEP-17) and `HasStorage` (they
   all have it by default now) features (#1544)
 * notification size is restricted now (#1551)
 * unsolicited `addr` messages are now treated as errors (#1562)
 * native contracts no longer have `name()` methods, it is now a part of
   manifest (#1557)
 * transaction fees, invocation GAS counters and unclaimed GAS values are now
   returned as strings with floating point values from RPC calls (#1566,
   #1572, #1604)
 * NEF format was changed (#1555)
 * `engine.AppCall()` interop was renamed to `contract.Call()` (#1598)
 * call flags were renamed (#1608)
 * deploying contract now costs at minimum 10 GAS and MaxGasInvoke
   configuration was adjusted to account for that (the fee is configurable by
   the committee, #1624, #1635)

Improvements:
 * a lot of new tests added (#1335, #1339, #1341, #1336, #1378, #1452, #1508,
   #1564, #1567, #1583, #1584, #1587, #1590, #1591, #1595, #1602, #1633,
   #1638)
 * a number of optimizations across all node's components applied (#1342,
   #1347, #1337, #1396, #1398, #1403, #1405, #1408, #1428, #1421, #1463,
   #1471, #1492, #1493, #1526, #1561, #1618)
 * smartcontract package now provides function to create simple majority
   multisignature contract (in addition to BFT majority, #1341)
 * `AddNetworkFee()` now supports contract addresses (#1362)
 * error handling and help texts for CLI wallet commands (#1434)
 * compiler emitting short jump instructions if possible (#805, #1488)
 * compiler emitting jump instructions with embedded conditions where possible
   (#1351)
 * transaction retransmission mechanism added to mempool (#1536)
 * parallel block fetching (#1568)
 * Binary and Runtime interops refactored to separate packages (#1587)
 * message broadcasts now finish successfully if the message is sent to 2/3 of
   peers (#1637)

Bugs fixed:
 * TRY opcode implementation was not allowing for 0 offsets (#1347)
 * compiler wasn't dropping unused elements returned from calls (#683)
 * MEMCPY with non-zero destination index was not working correctly (#1352)
 * asset transfer from CLI didn't work (#1354)
 * specifying unknown DB type in configuration file induced node crash (#1356)
 * address specifications in configuration file didn't work correctly (#1356)
 * RPC server wasn't processing hashes with "0x" prefix in parameters (#1368)
 * incorrect context unloding on exception handling (#1343)
 * attempt to get committee only returned validators if there was no voting on
   chain (#1370)
 * block queue could be attacked with wrong blocks to cause OOM (#1374)
 * token sale example wasn't checking witness correctly (#1379)
 * structure methods were added to generated manifests (#1381)
 * type conversions in `switch` and `range` statements were compiled as
   function calls (#1383)
 * RPC server result format wasn't conforming to C# implementation for
   `getapplicationlog` and `invoke*` (#1371, #1387)
 * subslicing non-byte slices was miscompiled (it's forbidden now, #1401)
 * potential deadlock in consensus subsystem (#1410)
 * race in peer connection closing method (#1378)
 * race in MPT calculation functions (#1378)
 * possible panic on shutdown (#1411)
 * block-level `getapplicationlog` result had block hash in it (#1413)
 * fail CN execution if wrong password is provided in the configuration
   (#1419)
 * tx witness check didn't account for GAS properly when several witnesses are
   used (#1439)
 * map keys longer than 64 bytes were allowed (#1433)
 * unregistered candidate with zero votes wasn't removed (#1445)
 * standard contract's network fee wasn't calculated correctly (#1446)
 * witness checking wasn't taking into account transaction size fee (#1446)
 * sending invalid transaction from the CLI wasn't prevented in some cases
   (#1448, #1479, #1604)
 * `System.Storage.Put` wasn't accounting for new key length in its GAS price
   (#1458)
 * blocks can't have more than 64K transactions (#1454)
 * Policy native contract wasn't limiting some values (#1442)
 * MerkleBlock payload wasn't serialized/deserialized properly (#1454, #1591)
 * partial contract updates were not always possible (#1460)
 * potential panic on verification with incorrect signature size (#1467)
 * CLI attempted to save transaction when it wasn't requested (#1467)
 * VM allowed to create bigger integers than it should support (#1464)
 * some protocol limits were not enforced (#1469)
 * iterating over storage items could produce incorrect KV pairs when using
   LevelDB or BadgerDB (#1478)
 * stale transaction were not deleted from the mempool (#1480)
 * node panic during block processing with BoltDB (#1482)
 * node that failed to connect to seeds on startup never attempted to
   reconnect to them again (#1486, #1592)
 * compiler produced incorrect code for if statements using function calls
   (#1479)
 * invocation stack size check wasn't performed in some cases (#1492)
 * incorrect code produced by the compiler when assigning slices returned from
   functions to new variables (#1495)
 * websocket client closing connection on new events (#1494)
 * minor `getrawtransaction`/`gettransactionheight` and NEP5-related RPC
   implementation incompatibilities (#1543, #1550)
 * VM CLI breakpoint wasn't stopping before the instruction (#1584)
 * VM CLI was incorrectly processing missing parameter error (#1584)
 * wallet conversion wasn't performed correctly (#1590)
 * node didn't return all requested blocks in response to `getblocks` P2P
   requests (#1595)
 * CN didn't request transactions properly from its peers in some cases
   (#1595)
 * incorrect manifests generated for some parameter types (#1599)
 * incorrect code generated when no global variables are present, but there
   are some global constants (#1598)
 * native contract invocations now set proper calling script hash (#1600)
 * byte string and buffer VM stack items conversion to JSON differed from C#
   (#1609)
 * when mempool is full new transaction's hash could still be added into it
   even if it is to be rejected afterwards (#1621)
 * CN wasn't always performing timestamp validation correctly (#1620)
 * incorrect stack contents after execution could stop block processing
   (#1631)
 * `getapplicationlog` RPC call handler wasn't validating its parameters
   properly, potentially leading to node crash (#1636)
 * a peer could be connected twice in rare circumstances (#1637)
 * missing write timeout could lead to broadcasting stalls (#1637)

## 0.91.0 "Ululation" (18 August 2020)

We've updated NeoGo for 3.0.0-preview3 compatibility implementing all the
appropriate protocol changes as well as improving NeoGo-specific
components. This release brings with it significant changes for smart
contracts, both in terms of Neo protocol changes (no more there is a single
entry point! execution environment has also changed in lots of ways) and Go
smart contract compiler updates and fixes.

Please note that this release is incompatible with 0.90.0 and there will be no
long-term support provided for it, Neo 3 is still changing and
improving. If you have any wallets used with 0.90.0 release you'll need to
regenerate them from private keys because of changes to verification scripts
that also changed hashes and addresses.

But nonetheless it is tested to be compatible with preview3 testnet for up to
68K of blocks in terms of storage changes, with neo-debugger for debug data
produced by the compiler and with consensus process for heterogeneous setups
(like 2 neo-go CNs with 2 C# CNs).

New features:
 * secp256k1 signature checks added to interop functions
   (Neo.Crypto.VerifyWithECDsaSecp256k1 and
   Neo.Crypto.CheckMultisigWithECDsaSecp256k1 syscalls,
   crypto.ECDsaSecp256k1Verify and crypto.ECDSASecp256k1CheckMultisig interop
   functions, #918)
 * RIPEMD160 hash added to interop functions (Neo.Crypto.RIPEMD160 syscall,
   crypto.RIPEMD160 interop function, #918, #1193)
 * "NotFound" P2P message (#1135, #1333)
 * base64 encoding/decoding interop functions (binary.Base64Encode and
   binary.Base64Decode, #1187)
 * new contract.GetCallFlags interop (System.Contract.GetCallFlags syscall)
   implemented (#1187)
 * it is possible now to create iterators and enumerators over primitive VM
   types in smart contracts (#1218)
 * runtime.Platform interop (System.Runtime.Platform syscall) is now available
   to smart contracts in Go (#1218)
 * storage.PutEx interop (System.Storage.PutEx syscall) is now available to
   smart contracts in Go (#1221)
 * exceptions support in VM (#885)
 * CLI conversion utility functions for addresses/hashes/etc (#1207, #1258)
 * multitransfer transactions now can be generated with RPC client or CLI
   (#940, #1260)
 * System.Callback.* syscalls for callback creation and execution (callback.*
   interop functions, #1197)
 * MPT implementation was added (#1235)
 * Policy native contract now also contains MaxBlockSystemFee setting (#1195)
 * getting blocks by indexes via P2P is now supported (#1192)
 * limited pointer support was added to the compiler (#1247)
 * voting support in CLI (#1206, #1286)

Behavior changes:
 * crypto.ECDsaVerify interop function was renamed to
   crypto.ECDsaSecp256r1Verify now that we have support for secp256k1 curve
   (#918)
 * many RPC requests/responses changed names used for data fields (#1169)
 * runtime.Notify interop function now requires a mandatory UTF8 name
   parameter (#1052) and this name can be used to filter notifications (#1266)
 * sendrawtransaction and submitblock RPC calls now return a hash instead of
   boolean value in case of success (#1216)
 * System.Runtime.Log syscall now only accepts valid UTF8 strings no longer
   than 1024 bytes (#1218)
 * System.Storage.Put syscall now limits keys to 64 bytes and values to 1024
   bytes (#1221)
 * PUSHA instruction now works with relative code offset (#1226)
 * EQUAL instruction no longer does type conversions, so that values of
   different types are always unequal (#1225)
 * verification scripts now can't use more than 0.5 GAS (#1202)
 * contracts no longer have single entry point, rather they export a set of
   methods with specific offsets. Go smart contract compiler has been changed
   accordingly to add all exported (as in Go) methods to the manifest
   (but with the first letter being lowercased to match NEP-5 expectations,
   #1228). Please also refer to examples changes to better see how it affects
   contracts, manifests and configuration files (#1296)
 * native contracts are now called via Neo.Native.Call syscall (#1191)
 * compressed P2P payloads now also contain their uncompressed size (#1212,
   #1255)
 * NEF files now use double SHA256 for checksums (#1203)
 * VM's map keys and contract methods now can only contain valid UTF-8 strings
   (#1198)
 * stack items now can be converted to/from JSON natively (without
   smartcontract.ContractParameters intermediate) which is now used for
   invoke* RPC calls and application execution logs (#1242, #1317)
 * invoking Policy native contracts now requires AllowsStates (to get
   settings) or AllowModifyStates (to change setting) flags (#1254)
 * Transaction now has Signers field unifying Sender (the first Signer) and
   Cosigners, a Signer can have FeeOnly or any other regular witness scope
   (#1184)
 * verification scripts no longer have access to blockchain's state (#1277)
 * governance scheme was changed to delegated committee-based one. The number
   of validators is now specified with ValidatorsCount configuration option,
   standby validators are no longer being registered by default (#867, #1300)
 * Go 1.13+ is now required to build neo-go (#1281)
 * public contract methods now always return some value and this is being
   checked by the VM (#1196, #1331, #1332)
 * runtime interop package now exports triggers as proper constants rather
   than functions (#1299)
 * RPC client no longer has SetWIF/WIF methods that didn't do anything useful
   anyway (#1328)

Improvements:
 * Neo.Crypto.CheckMultisigWithECDsaSecp256r1 syscall is now available via
   crypto.ECDSASecp256r1CheckMultisig interop function (#1175)
 * System.Contract.IsStandard syscall now also checks script's container (#1187)
 * syscalls no longer have allowed triggers limitations (#1205)
 * better testing coverage (#1232, #1318, #1328)
 * getrawmempool RPC call now also supports verbose parameter (#1182)
 * VMState is no longer being stored as a string for application execution
   results (#1236)
 * manifest now contains a list of supported standards (#1204)
 * notifications can't be changed now by a contract after emitting them
   (#1199)
 * it is possible to call other contracts from native contracts now (#1271)
 * getnep5transfers now supports timing parameters (#1289)
 * smartcontract package now has CreateDefaultMultiSigRedeemScript that should
   be used for BFT-compliant "m out of n" multisignature script generation
   (#1300)
 * validators are always sorted now (standby validators were not previously,
   #1300)
 * debug information now contains all file names (#1295)
 * compiler now accepts directory to compile a package, only one file could be
   passed previously (#1295)
 * some old no longer used functions and structures were removed (#1303)
 * contract inspection output was improved for new Neo 3 VM instructions (#1231)
 * ping P2P message handling was changed to trigger block requests (#1326)

Bugs fixed:
 * inability to transfer NEO/GAS from deployed contract's address (#1180)
 * System.Blockchain.GetTransactionFromBlock syscall didn't pick all of its
   arguments from the stack in some error cases (#1187)
 * System.Contract.CallEx syscall didn't properly check call flags (#1187)
 * System.Blockchain.GetContract and System.Contract.Create syscalls returned
   an interop interface instead of plain well-defined structure (#1187)
 * System.Contract.Update syscall's manifest checks were improved, return
   value was fixed (#1187)
 * getnep5balances and getnep5transfers RPC calls now support addresses in
   their parameters (#1188)
 * rare panic during node's shutdown (#1188)
 * System.Runtime.CheckWitness, System.Runtime.GetTime syscalls are only allowed to be called with
   AllowStates flag (#1218)
 * System.Runtime.GasLeft syscall result for test VM mode was wrong (#1218)
 * getrawtransaction RPC call now also returns its VM state after execution
   (#1183)
 * getnep5balances and getnep5transfers RPC calls now correctly work for
   migrated contracts (#1188, #1239)
 * compiler now generates correct code for global variables from multiple
   files (#1240)
 * compiler now correctly supports exported contracts and variables in
   packages (#1154)
 * compiler no longer confuses functions with the same name from different
   packages (#1150)
 * MaxBlockSize policy setting was not enforced (#1254)
 * missing scope check for signers (#1244)
 * compiler now properly supports init() functions (#1253, #1295)
 * getvalidators RPC call now returns zero-length array of validators when
   there are no registered candidates instead of null value (#1300)
 * events were not added to the debug data (#1311, #1315)
 * RPC client's BalanceOf method was lacking account parameter (#1323)
 * VM CLI debugging commands didn't really allow to step through the contract
   (#1328)
 * recovery message decoding created incorrect PrepareRequest payload that
   lead to consensus failures (#1334)

## 0.90.0 "Tantalization" (14 July 2020)

The first Neo 3 compatible release of neo-go! We've targeted to make it
compatible with preview2 release of Neo 3, so it only contains features
available there, but at the same time this makes the node more useful until we
have some more up to date reference version. It's a completely different
network, so almost everything has changed and it's hard to describe it with
the same level of details we usually do (but we'll provide them for subsequent
releases where the changeset is going to be lower in size). Please note that
this is a preview-level release and there won't be long-term support provided
for it, Neo 3 is evolving and the next release won't be compatible with this
one.

Main Neo 3 features in this release:
 * no UTXO
 * native contracts
 * new VM
 * scoped witnesses for transaction
 * updated interop/syscalls set
 * contract manifests
 * more efficient P2P protocol

Things that have also changed:
 * transaction format
 * block format
 * address format
 * wallets
 * RPC protocol
 * notification subsystem
 * executable format output for compiler

Compatibility level of this neo-go release:
 * identical storage changes compared to C# node for 378K blocks of preview2
   testnet
 * debugging info produced is compatible with preview2-compatible neo-debugger
 * running consensus nodes in heterogeneous setup is possible (2 neo-go CNs
   with 2 C# CNs, for example)

Changes specific to neo-go:
 * some CLI parameters like wallet path or RPC endpoint URL have been unified
   across all commands and thus have changed in some of them (refer to CLI
   help for details)
 * as an extension we support post-preview2 cosigners parameter for
   invokefunction RPC calls (see neo-project/neo-modules#260)
 * Go compiler now supports comparisons with nil properly
 * we no longer provide bootstrapping 6k block dump for private networks, you
   have 30000000 GAS right in the genesis block and it's not hard to make use
   of it (see
   [neo-go-sc-wrkshp](https://github.com/nspcc-dev/neo-go-sc-wrkshp) for an
   example of how to use it)
 * we have a conversion tool for your old Neo 2 wallets (`wallet convert`
   command), so you can reuse keys on Neo 3 networks
 * util.Equals interop function may not function the way you expect it to due
   to Neo VM changes, it still is an EQUAL opcode though. This interop may be
   removed in the future.

## Older versions

Please refer to the [master-2.x branch
CHANGELOG](https://github.com/nspcc-dev/neo-go/tree/master-2.x/CHANGELOG.md)
for versions prior to 0.90.0 (that are Neo 2 compatible, unlike 0.90.0+ that
are Neo 3 compatible).
