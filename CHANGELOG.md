# Changelog

This document outlines major changes between releases.

## 0.78.3 "Calcification" (22 February 2021)

We're updating NeoGo node for Neo 2 network with this release bringing a
number of fixes for various problems. Some were found during our internal RPC
compatibility audit and some are just backports from Neo 3 development
branch. There are no new features introduced, but it's an important
upgrade. Unfortunately we had to change the DB format to fix some problems, so
add some time for resynchronization to your node maintenance plan.

Improvements:
 * RPC documentation was updated mentioning all known differences with C#
   implementation that are considered to be not worth fixing (#1760)

Bufs fixed:
 * when mempool is full new transaction's hash could still be added into it
   even if transaction is to be rejected afterwards (#1623)
 * `gettxout` RPC API was returning data for already spent outputs, although it
   shouldn't (C# node doesn't do that, #1649)
 * `getrawtransaction` RPC call returned metadata for pooled (not yet accepted)
   transactions (#1648)
 * `gettransactionheight` RPC call returned zero height for pooled (not yet
   accepted) transactions (#1648)
 * VM CLI set breakpoints after target instructions, although it should set
   them before execution (#1648)
 * a peer could be disconnected and not reconnected again properly in some
   cases (#1648)
 * missing write timeout could lead to broadcasting stalls (#1648)
 * CN transaction requests could try to ask for more transactions from peers
   than it is allowed to by the protocol (#1648)
 * CN transaction requests could be trimmed unnecessarily (#1648)
 * FP Fixed8 values (GAS usually) could be unmarshalled from JSON/YAML
   incorrectly in some cases (#1761)
 * previous proposal reuse could lead to empty blocks accepted even if there
   are transactions in the mempool (#1761)
 * UTXO asset issuer wasn't properly stored by the node (#1760)
 * `getassetstate` RPC call answer used wrong field name for "frozen" data
   (#1760)
 * UTXO asset type was serialized as numeric code instead of a string in
   `getrawtransaction` and getassetstate RPC call results (#1760)
 * `getblockheader` RPC call didn't support block numbers in parameters (#1760)
 * `getblocksystemfee` RPC call returned block's system fee while it should
   return cumulative system fee from genesis block to the one requested
   (#1760)
 * system fee calculation was wrong for some types of transactions (#1760)
 * `gettxout` RPC call implementation was returning asset hash in wrong byte
   order (#1760)
 * `getpeers` RPC call implementation returned port numbers wrapped into
   strings (#1760)
 * `getenrollments` RPC call implementation returned duplicate wrong records
   along with proper ones (#1760)
 * NEP5 tracking code wasn't deleting entries with zero balance leading to
   excessive getnep5balances request outputs for some addresses (#1760)
 * NEP5 tracking code wasn't able to handle tokens overflowing int64 in
   transfer and balance amounts (like pnWETH, #1760)
 * `confirmations` value in `getblock` RPC call response was shifted by two
   (#1760)
 * some fields with default values were omitted for `getrawtransaction` and
   `getblock` RPC call answers with Register and Publish transactions (#1760)

## 0.78.2 "Colorization" (03 December 2020)

Release 0.78.2 adds some minor features to the node while still being
compatible both for protocol and for DB format.

New features:
 * `getblocktransfertx` RPC API to get list of asset-transferring transactions
   (both UTXO and NEP5) for a block, see docs/rpc.md for more details (#1516)
 * support for batched JSON-RPC requests (#1509)
 * `KeepOnlyLatestState` option to only track the latest MPT state and not store
   old trees, it's disabled by default (keeping old behavior) and can only be
   turned on for a new DB (resynchronizing state from the genesis block), but
   it allows to drastically reduce disk space requirements for mainnet node
   (and even make it work a bit faster while still having stateroot
   tracking). Nodes with stateroot disabled (`EnableStateRoot: false`) use ~53GB
   for DB at current 6.5M height, stateroot-enabled nodes (`EnableStateRoot:
   true`) require ~122GB, but with `KeepOnlyLatestState: true` setting (and
   `EnableStateRoot: true`) the DB size is ~56GB, so the difference is quite
   noticeable for disk-constrained setups.

Improvements:
 * mempooled transaction retransmission mechanism (#1532)

## 0.78.1 "Circumnavigation" (22 October 2020)

This is a bug fix release that solves DB access and seed reconnection
problems. If you're using LevelDB or BadgerDB for backend storage you need to
resynchronize your node for testnet and you're recommended to do that for
mainnet. BoltDB users can proceed with the old DB.

Behavior changes:
 * default mainnet configuration now has updated values for FreeGas changes
   height (6195000 instead of 6200000, #1412)

Improvements:
 * dBFT timer was tuned to eliminate block processing delay from the
   inter-block interval, this only affects consensus nodes (#1429)

Bugs fixed:
 * node using LevelDB or BadgerDB for backend storage had incorrect state for
   testnet (#1468)
 * node that failed to connect to seeds on startup never attempted to
   reconnect to them again (#1483)
 * node using BoltDB could panic in various situations like processing a new
   block, this was noticed on the master branch with 3.0 support and never
   seen on 2.x, although theoretically it could still happen (#1482)

## 0.78.0 "Cardioacceleration" (15 September 2020)

This version is a Neo 2.12.0-compatible release that follows DeFi-related
network changes while also introducing some new RPC functionality and fixing
bugs.

There is an important configuration update there, please check FreeGasLimit,
MaxFreeTransactionsPerBlock and MaxTransactionsPerBlock settings as they're
now height-dependent (see the default mainnet and testnet configurations for
example of how to configure them).

If you're using getutxotransfer calls, please resynchronize your node to fix
NEO values returned.

New features:
 * minimum network fee policy value can now be configured, but it defaults to
   zero for now (#1397)
 * stateroot import/export (#1307)
 * new `getalltransfertx` RPC call returning all UTXO and NEP5 transactions
   for specified account (parameters are the same as for `getnep5transfers`,
   see RPC documentation for details, #1399, #1404)
 * GetUTXOTransfers support was added to RPC client (#1399)

Behavior changes:
 * FreeGasLimit is now height-dependent (configured as a map, instead of
   simple value), default value for mainnet since 6200000 and testnet since
   4840000 is 50.0 (#1394, #1397, #1400, #1402)
 * getnep5transfers and getutxotransfers RPC calls now support limiting and
   paging in addition to time span selection, by default both only return a
   1000 of elements (and it's the maximum possible) for a single call (#1395,
   #1399)
 * MaxTransactionsPerBlock and MaxFreeTransactionsPerBlock are
   height-dependent now, the default values for mainnet and testnet change
   since 6200000 and 4840000 respectively to 200 transactions per block with
   199 free ones
 * RPC client's GetNEP5Transfers call was updated to support new parameters
   (#1399)

Improvements:
 * the project is now using proper YAML library import path (#1306)
 * the node will exit with an error if no valid password is provided for CN
   wallet (#1316)

Bugs fixed:
 * state root addition errors in CN logs (harmless, but annoying, #1313)
 * state root was generated by CNs before StateRootEnableIndex (#1320)
 * panic on consensus watcher node (#1314)
 * RPC calls working with 256-bit hash parameters were not accepting hashes
   with "0x" prefix (#1369)
 * getutxotransfer RPC call returned NEO values multiplied by 10⁸ (#1386)
 * incorrect vout indexes in getblock or getrawtransaction RPC outputs (#1392)
 * network fee check was not accounting for LowPriorityThreshold correctly
   (#1397)

## 0.77.0 "Cornification" (11 August 2020)

This release is aligned with Neo 2.11.0 release, bringing important changes to
the NeoX consensus protocol, some bug fixes and new RPC functionality. It is
tested to work in consensus with 2.11.0. The DB format was changed for NEP5
tracking data and we also track UTXO transfers now, so you need to
resynchronize the DB from the genesis for this version to operate correctly.

New features:
 * `getutxotransfers` RPC call to track NEO and GAS movements. It accepts
   address as mandatory parameter, asset name ("gas" or "neo") and start/end
   time as optional parameters (similar to `getnep5transfers`). See #1268 and
   #1288.
 * `invoke*` RPC calls now support passing hashes to be used as verified ones
   during execution (for contract's `CheckWitness` calls), this functionality
   is also available in the CLI now (#1282)

Behavior changes:
 * `getnep5transfers` and `getnep5balance` now return raw balance values, the
   same way C# node does (previously the result was adjusted by contract's
   decimals value, #1250)
 * `getstateheight` RPC call response now uses lowercase field names (#1274)
 * `getnep5transfers` RPC call now accepts start/end optional time parameters
   (the same way C# plugin does), lacking start option the result might be
   different from the one returned by previous node versions (#1284)
 * we no longer support Go 1.12 to build the node (#1285)
 * NeoX consensus was reworked to exchange signatures during Prepare phase and
   emit stateroot message along with Commit (#1291). Note that this new
   consensus process is not compatible with 0.76.X versions of neo-go or
   2.10.3-neox-preview1 version of C# node, but it is compatible with 2.11.0,
   so you need to upgrade all of your consensus nodes for the network to run
   correctly. Also note that the default configuration for mainnet was changed
   to enable StateRoot functionality.

Bugs fixed:
 * `getnep5transfers` RPC call not returning results for migrated contracts
   (#1215)
 * incorrect consensus recovery message data for NeoX-enabled networks (#1270)

## 0.76.2 "Calibration" (19 July 2020)

Minor update for Neo 2. If you're running testnet node, we recommend to
resynchronize it to fix state mismatch at block 4516236, mainnet is not known
to have any similar problem so you can keep the old DB.

Bugs fixed:
 * rare panic on node shutdown (#1185)
 * VM not clearing alt stack when doing CALLs (#1158)
 * incorrect resource accounting for isolated calls (#1186)
 * state height wasn't updated properly for networks with StateRootEnableIndex
   higher than zero (like testnet, #1213)

## 0.76.1 "Conduplication" (15 July 2020)

Minor bug fixing for Neo 2 implementation. If you're affected by NEP5 balance
tracking bug, please resynchronize your node to get correct results, other
than that it's not required, the DB is fully compatible.

Bugs fixed:
 * block synchronization stalls on consensus node when it's not yet up to date
   and there are a lot of blocks to fetch (#1116)
 * getnep5balances and getnep5transfers RPC calls didn't support passing
   addresses as parameters (#1146)
 * NEP5 balance tracking was not aware of contract migrations leading to wrong
   data being returned for getnep5balances RPC call (#1144)

## 0.76.0 "Cross-pollination" (25 June 2020)

We wanted to make a 0.75.1-neox-preview1 release of neo-go compatible with
version 2.10.3-neox-preview1 of C# node, but then suddenly decided to make
something better than that and now release 0.76.0 instead which brings with
it configurable cross-chain functionality support. So there is no need to use
different node builds for different networks with NeoGo, one binary fits
all. As usual some bugs were also fixed, this time mostly concentrating around
consensus functionality.

New features:
 * cross-chain (aka neox) support, refer to docs/neox.md for details on what's
   included and how to use it
 * compiler built-in Remove function was added for slices/maps element
   deletion (#1021)

Behavior changes:
 * the default testnet config now enables state root with StateRootEnableIndex
   of 4380100, if you have a testnet node you need to either stay with the old
   configuration or remove the DB and resynchronize blocks from genesis
 * contracts using comparison with nil will fail to compile, this comparison
   actually never functioned correctly (#952)

Improvements:
 * storage cache flushing was optimized, improving block import speed by ~10%
   (#1014)
 * consensus process now logs a bit more relevant messages (#1041)
 * limits to invocation scripts were added to consensus messages (#1087)
 * transaction request retries were added in case consensus process is missing
   some transactions (#1095)

Bugs fixed:
 * dbft's failed nodes detection was tuned for the case of node lagging behind
   the network (#1009)
 * dbft was fixed to always process recovery messages for the current block
   (#1009)
 * compiler now initializes complex struct fields correctly (#952)
 * NOTEQUAL opcode is now only being emitted for integers by the compiler (#952)
 * typo in docker entrypoint script preventing proper bootstrap file passing
   (#1089)
 * MaxFreeTransactionsPerBlock limit wasn't really enforced (#1019)
 * changeview payloads were not processed correctly by the consensus subsystem
   (#1041)
 * dbft library wasn't including messages from higher views for last seen
   message checks (#1041)
 * recovery request sent instead of recovery message in one case (#1095)
 * some correctness checks were not done for proposed block in not all
   transactions were present when the node received PrepareRequest (#1095)
 * timeout wasn't properly calculated in some cases for Primary node leading
   to early PrepareRequest send (#1095)

## 0.75.0 "Caramelization" (28 May 2020)

A long-awaited Neo 2.0 update for neo-go that fixes a lot of subtle little
differences in VM and syscalls behavior compared to C# node that resulted in
storage state mismatches between two nodes. This release makes neo-go fully
compatible with public testnet and mainnet chains, for every transaction in
every block you get the same result.

But it's not just about bugs, as it's been quite a long development cycle,
we've also included some interesting new features like notification subsystem,
neo-debugger compatibility and BadgerDB support. Smart contract compiler and
interop packages were also updated making neo-go even better for developing
real-world complex smart contracts.

New features:
 * support for `for` loops with no condition was added to the compiler (#799)
 * compiler can now emit debug information compatible with neo-debugger (#804,
   #829)
 * experimental BadgerDB support was added (#839)
 * support for abi.json files generation was added for contract deployment
   with NEO-Express (#916)
 * RPC over websocket connections is now supported both by the server (with
   ws://$SERVER:$PORT/ws URL) and client (WSClient structure, #921)
 * notification subsystem was added for RPC server and client (using websocket
   connections, #895)
 * interop package now has a complete set of syscalls available (#795, #956)
 * push command was added to VM CLI (#967)
 * diff dumps are now supported in `db restore` CLI command (#991)

Behavior changes:
 * due to DB format changes you'll need to resynchronize your node from
   scratch (and it'll also update its state to a more proper one)
 * runtime.Notify interop now accepts varargs (#825)
 * compiler's `--debug` parameter is now used for debug information generation,
   use `--verbose` to get more internal compiler's messages (#829)
 * compiler now outputs hex-encoded contract's bytecode only with `--verbose`
   option (#829, previously it was always printed)
 * RPC client's representation of GetBlock results changed significantly (#951)
 * some interop functions changed their in/out types to a more proper ones
   (#956, though previous ones didn't really work, so no current code should
   notice that)
 * `skip` parameter to `db restore` CLI command was replaced by `start`
   (#991), when using full (non-diff) NGD dumps with short (`-s`) form they're
   compatible, but `start` (as a block index) makes more sense for diff dumps

Improvements:
 * (*Stack).PushVal in vm now supports all types of integers (#776)
 * state.AppExecResult now stores stack in binary representation (#782)
 * vm.NewBigIntegerItem now accepts int64 parameter (#788)
 * RPC client tests were added (#716)
 * buffer reuse added for persisting temporary changes which improved block
   handling speed (#772)
 * significant RPC server package refactoring was done (#753)
 * BoltDB was updated to v1.3.4 (#793)
 * Go 1.14 is now supported (#775)
 * serialization/deserialization tests were unified (#802)
 * emit package now has Array and AppCallWithOperationAndArgs methods for
   easier script creation (#803)
 * persisting from MemCachedStore to MemCachedStore was optimized which
   allowed to gain 10% improvement in block import time for 1.5M mainnet
   blocks (#807)
 * storage.Find usage example was added (#795)
 * VM stack item tests were improved (#812)
 * `config` directory now only contains configuration files, no Go code (#423,
   #816)
 * local variables are counted more accurately now in the compiler leading to
   less waste in script runtime (#815)
 * NEP5 example was extended with minting function making it a bit more usable
   (#823)
 * DAO was refactored away into its own package from core (#832)
 * additional tests were added for bitwise and numeric VM operations (#833)
 * tests for VM CALL* instructions were added (#833)
 * consensus message parsing was split into two parts so that ordinary nodes
   won't touch CN's data (#862)
 * contract's metadata was moved into smartcontract package from rpc (#916)
 * interop packages documentation was extended (#956)
 * Docker build was fixed to use Makefile (#981)

Bugs fixed:
 * integer stack values were not following C# node format when being converted
   to JSON (#770)
 * vm now uses truncated division to match C# node behavior for negative
   integers (#773)
 * getapplicationlog and other RPC calls now convert Uint160 to JSON the same
   way C# node does (#769)
 * asset state JSON representation now follows the same format as C# node uses
   (with proper field names for 'id' and 'type' and without 'fee' add
   'address' fields, #785, #819)
 * `nextconsensus` field in `getblockheader` RPC call answer was using hex
   representation instead of address (#763)
 * `getcontractstate` RPC call implementation was using wrong format for
   `script` field in answer (#763)
 * `getnep5balances` RPC call implementation was using wrong name for
   `balance` field in answer (#763)
 * `getvalidators` RPC call was using non-compliant validators keys format in
   its answer (#763)
 * potential problems with GAS parameter handling in CLI (#790)
 * `contract init` CLI command was not working correctly (#792)
 * RPC calls now accept integer parameters in JSON strings which fix some
   incompatibilities (#794)
 * CALLI VM instruction was using wrong offset (#791, #798)
 * vm Map stack item for using different serialization format from C# node
   (#806, #808)
 * invoke* RPC calls were returning stack in `Stack` element rather than more
   compliant `stack` (#805)
 * slices of compound types were treated wrong by the compiler (#800)
 * wrong code was generated for struct variables declarations by the compiler
   (#800)
 * RPC client was incorrectly processing GetRawTransactionVerbose,
   GetBlockVerbose, GetBlockHeader, GetTxOut and ValidateAddress call results
   (#789)
 * type-specific transaction data was completely missing in getrawtransaction
   RPC call verbose output (#585)
 * documentation for wallet CLI commands was fixed to follow renames made by
   previous version (#814)
 * panic in map-containing notifications processing (#809)
 * VM Map implementation now has deterministic iteration order (#818)
 * MOD instruction behavior was fixed to follow neo-vm (#826)
 * negative arguments are now accepted for SHL/SHR VM instructions (#827)
 * improper in-block cache propagation leading to storage state differences
   with C# node (#821, #817)
 * CLI didn't check for mandatory `method` parameter presence for
   invokefunction command (#828)
 * wrong code generated by the compiler for return statements with no
   parameters (#929)
 * wrong code generated by the compiler for multiple function arguments with
   one type specification (#935)
 * NEP5 example contained wrong address check (#945)
 * improper code generated by the compiler for `op=` assignments to struct
   fields and slice elements (#954)
 * Storage.Find elements order was adjusted to match C# node implementation
   and eliminate state differences resulting from that (#822, #977, #988,
   #994)
 * step command wasn't really working in VM CLI (#967)
 * PICKITEM instruction implementation was fixed to reject improper input
   argument types (#965, #967)
 * Runtime.CheckWitness was fixed to only accept compressed keys (#968, #971)
 * input data length check was added to (*PublicKey).DecodeBytes (#971)
 * VM reference counting for SETITEM instruction was fixed (#973)
 * Map VM stack item can now be converted to Boolean (#974)
 * Structs were not cloned by SETITEM instruction implementation (#972, #975)
 * GetUnspentCoins syscall implementation was fixed to return array (#978,
   #979, #984)

## 0.74.0 "Comprehension" (17 Mar 2020)

Functionally complete NEO 2.0 node implementation, this release can be used as
a drop-in replacement for C# node in any setting. It features full RPC
functionality support and full set of wallet operations. As usual, there
also was a number of bugs fixed, the node stability improved and some great
optimizations were also done (especially concentrated around DB interactions),
so even though this release has more functionality than ever (and it stores a
lot more chain data than ever) it at the same time imports blocks faster than
the previous one.

Of course we will make additional maintenance releases for NEO 2.0 when
needed, but following this release we'll concentrate more on NEO 3.0 features
and catching up with recent community developments around that.

New features:
 * WIF and NEP2 keys import/export into/from the wallet (#685)
 * multisig accounts import into the wallet (#685)
 * additional key generation for existing wallets (#685)
 * support for `break` and `continue` statements in the compiler (#678)
 * `getblocksysfee` RPC method support (#341)
 * `getapplicationlog` RPC method support (#500, #754)
 * `getclaimable` RPC method support (#694)
 * gas claiming support in wallet CLI (#694)
 * asset transfer commands in wallet CLI (#694, #706)
 * `getrawmempool` RPC method support (#175)
 * RPC client support for all methods implemented in neo-go's server (#586,
   #715, #694, #723, #750)
 * `getblockheader` RPC method support (#722)
 * `getnep5balances` and `getnep5transfers` RPC methods support (#498, #751)
 * `gettransactionheight` RPC method support (#713)
 * `submitblock` RPC method support (#344)
 * `getvalidators` RPC method support (#714)
 * support for `switch` `fallthrough` statements in the compiler (#628)
 * a set of `NEP5*` methods added to RPC client for convenient NEP5 contract
   calls (#728, #764)
 * NEP tokens balance querying and transfers support for CLI (#728, #757)
 * `getunclaimed` RPC method support (#712)
 * contract import was added to the wallet CLI command set (#757)
 * key removal added to the wallet CLI command set (#757)
 * https support for RPC server (#702)

Behaviour changes:
 * gas parameter for deployment no longer specifies full gas to be added into
   the transaction, it now only specifies the network fee part while system
   fee part is computed automatically based on contract's metadata (#747)
 * contract deployment and invocation from CLI is now integrated with the
   wallet subsystem, WIF parameter support was dropped (#747)
 * wallet subcommands were renamed, `create` became `init` and
   `create-account` became simple `create` (#757)
 * DB format was changed several times during this release cycle, so please
   resynchronize your chains

Improvements:
 * improved and extended `wallet` package (#685, #694, #706, #728, #757)
 * refactored RPC package (dividing it into smaller subpackages, #510, #689)
 * `GroupInputsByPrevHash` is no longer tied to `Transaction`, allowing its
   wider use (#696)
 * more efficient `transaction.InOut` type is used for `References` (#696,
   #733)
 * RPC client's `SendToAddress` was renamed to `TransferAsset` to better
   reflect its purpose (#686)
 * P2P server's graceful shutdown with connection closing (#691)
 * broadcasted transaction batching was added improving network efficiency
   (#681)
 * dropped duplicating `rpc.StackParamType` in favor of improved
   `smartcontract.ParamType` (#690, #706)
 * trigger types moved to their own `trigger` package (#690)
 * all Go packages were moved from github.com/CityOfZion to
   github.com/nspcc-dev where they technically already reside since August
   2019 (#710)
 * NEP5 balances and transfers tracking was added to Blockchain (#723, #748)
 * optimized transaction inputs/outputs/results verification (#743)
 * `SpentCoin` and `UnspentCoin` structures were merged and moved into `state`
   package (#743)
 * `AddVerificationHash` method was added to `Transaction` to simplify
   attributes management (#752)

Bugs fixed:
 * `getpeers` RPC request was not returning unconnected and bad peers (#676)
 * RPC client was not reporting real error from the answer in case of HTTP
   error (#676)
 * potential race in `Seek` implementation for `MemoryStore` (#693)
 * improper handling of more than 64K branching points in the compiler (#687)
 * Enrollment transactions verification didn't check for validator key (#696)
 * Claim transaction witness check might miss some hashes from Inputs (#696)
 * double Claim verification was missing (#696)
 * network fee calculation was completely broken (#696)
 * mempool's `Remove` might drop wrong transaction (#697)
 * missing double claim verification for mempool transactions (#697)
 * server deadlock upon reaching connection limit (#691)
 * wrong logic short-circuiting by compiler in complex conditions (#699, #701)
 * `AddHeaders` method was not verifying headers in any way (#703)
 * missing Claim amount verification (#694)
 * bogus error returned from `GetValidators` when processing transfers to
   (yet) unexisting accounts (#694)
 * Claim and Miner transactions	network fee was wrong (#694)
 * negative outputs were allowed in transactions, but shouldn't (#694)
 * smart contract invocation CLI command failed to process some parameters
   correctly (#719)
 * fatal error on concurrent access to `Blockchain` internal variable (#720)
 * Invocation transactions missed some decoding checks (#718)
 * it was allowed to have fractional GAS in Invocation transactions (which is
   interpreted as system fee), but it shouldn't (#718)
 * system fee calculation for Invocation transactions was incorrect (#718)
 * `Transaction` JSON unmarshalling was processing Outputs and Witnesses
   incorrectly (#706)
 * panic on server shutdown in case it's not fully started yet (#721)
 * `sendrawtransaction` RPC method implementation was not following error
   codes convention (#724)
 * interop implementations were using wrong byte order for returned hashes
   leading to storage state differences at mainnet's block 2025204 (#727)
 * `getblock` verbose response format was not following official documentation
   (#734)
 * contract deployment could fail with no error returned (#736)
 * max contract description limit was wrong, leading to deployment failures
   (#735)
 * compiler didn't properly clean up stack on `return` or `break` in some
   situations (#731)
 * deadlock in discovery service (#741)
 * missing dynamic `APPCALL` support (#740)
 * `EQUAL` opcode implementation was not comparing different stack item types
   correctly (#745, #749)
 * RPC error on contract invocation when Hash160 is being passed into it
   (#758)
 * absent any ping timeouts configuration the node was constantly pinging its
   neighbours (#680)
 * contract's state migration was not done properly (#760)
 * db import with state dump was not saving dumps correctly on interruption
   and was now resuming writes to dump files correctly after restart (#761)
 * incomplete State transaction verification (#767)
 * missing Owner signature check for Register transaction (#766)
 * incomplete Issue transaction verification and mempool conflicts check
   (#765)

## 0.73.0 "Cotransduction" (19 Feb 2020)

This is the first release than can successfully operate as a Testnet CN
bringing with it fixed voting system implementation, policying support and
other related fixes. It also contains RPC improvements and updates to the
compiler.

New features:
 * Go smart contracts can now use variables for struct fields initialization
   (#656)
 * for range loops support in the compiler (#658)
 * subslicing support for `[]byte` type (#654)
 * block's storage changes dump support similar to NeoResearch's storage audit
   (#649)
 * `gettxout` RPC method support (#345)
 * `getcontractstate` RPC method support (#342)
 * `getstorage` RPC method support (#343)
 * GetChangeAddress function in the wallet (#682)
 * basic policying was implemented allowing to configure
   MaxTransactionsPerBlock, MaxFreeTransactionsPerBlock,
   MaxFreeTransactionSize and FeePerExtraByte (#682)

Behaviour changes:
 * consensus process now only start when server sees itself synchronized with
   the rest of the network (see (*Server).IsInSync method, #682)
 * default testnet and mainnet configurations now contain policer settings
   identical to default SimplePolicy C# plugin (#682), privnet is not changed

Improvements:
 * keys.PublicKey now has Cmp method available (#661)
 * core now exports UtilityTokenID and GoverningTokenID functions (#682)
 * miner transactions generated by consensus process now properly set outputs
   based on block's transactions fees (#682)
 * (*Blockchain).`IsLowPriority` now takes fee (Fixed8) as an input (#682)
 * mempool's GetVerifiedTransactions and TryGetValue now also return
   transaction's fee
 * DBFT logging was improved
 * keys package no longer has Signature methods, they were replaced with more
   useful GetScriptHash (#682)

Bugs fixed:
 * compiler produced wrong code if there was some data containing byte 0x62 in
   the program (#630)
 * answer to the `getblock` RPC method was not following the specification in
   multiple places (#659)
 * State transaction's descriptors were not encoded/decoded correctly (#661)
 * keys.PublicKeys slice was not decoded properly (#661)
 * 'Registered' descriptor of State transaction was not decoded correctly
   (#661)
 * voting processing and validators election was fixed to follow C#
   implementation (#661, #512, #664, #682)
 * deadlock in network subsystem on disconnect (#663)
 * RPC answers with transactions were not following the specification in fields
   names (#666)
 * segmentation fault when checking references of bad transactions (#671)
 * `getassetstate` and `getrawtransaction` RPC methods were not returning
   error properly when missing asset or transaction (#675)
 * consensus RecoveryMessage encoding/decoding wasn't correct (#679)
 * DBFT was not reinitialized after successful chain update with the new block
   received from other peers (#673)
 * DBFT timer was extended too much, not following the C# implementation (#682)

## 0.72.2 "Confabulation" (11 Feb 2020)

Bugfix and small refactoring release, though it's the first one to include
improved CHECKMULTISIG implementation that is ~20% faster.

Improvements:
 * parallel CHECKMULTISIG implementation improving this instruction speed by
   around 20% (#548)
 * bytecode emitting functions moved to separate package, avoiding old
   duplicate code and improving testing (#449, #534, #642)
 * AppCall interop now accepts variable number of arguments (#651)
 * more logging at info level from consensus subsystem, following neo-cli
   behaviour

Bugs fixed:
 * compiler-emitted integers were using wrong format (#642)
 * FreeGasLimit configuration set to zero was not really disabling gas limits
   (#652)
 * stale disconnected peers could cause networking subsystem deadlock (#653)
 * transaction reverification (on new block processing) could lead to
   blockchain deadlock for non-standard verification contracts (#655)

## 0.72.1 "Contextualization" (07 Feb 2020)

Exactly one bug fixed relative to the 0.72.0:
 * testnet synchronization failed at block 713985 because of wrong
   notification generated by the invocation transaction (#650)

## 0.72.0 "Crystallization" (06 Feb 2020)

Starting at the end of the year 2019 we've spent a lot of effort to
stress-test our node with transaction load and this allowed us to uncover
some problems in various parts of the system. This release addresses all of
them and brings with it updated networking, consensus and mempool components
that can handle quite substantial transaction traffic and successfully produce
new blocks while being stressed. There are also important updates in VM that
fix incompatibilities with C# node and interesting improvements in compiler
based on our experience porting neofs smart contract to Go.

New features:
 * `dump` command for wallet to dump it (replaced non-functional `open`
   command, #589)
 * support for single-node consensus setup (#595)
 * ping/pong P2P messages are now supported with PingInterval and PingTimeout
   configuration options (#430, #610, #627, #639)
 * VM now fully supports calculating GAS spent during execution (#424, #648)
 * Fixed8 type now supports YAML marshalling (#609)
 * RPC invoke* calls can now be GAS-limited with MaxGasInvoke configuration
   option (#609)
 * gas limit for free invocation transactions can now be configured with
   FreeGasLimit option which default to 10.0 for mainnet and testnet (#609)
 * new compiler interop VerifySignature to verify ECDSA signatures (#617)
 * compiler now supports loops with single condition (no init/post, #618)
 * memory pool size is now configurable with MemPoolSize option which defaults
   to 50000 for all configurations (#614)
 * support for variables in slice literals in the compiler (#619)
 * support for map literals in the compiler (#620)
 * AppCall interop function (#621)
 * `string` to `[]byte` conversion was implemented in the compiler (#622)
 * Docker images now contain a dump of 1600 blocks for single-node setups and
   one can choose which dump to use for restore with ACC environment variable
   (#625)
 * switch statement support in the compiler (#626)
 * panic support in the compiler (#629)

Behaviour changes:
 * NEP2Decrypt function in keys package now returns a pointer to PrivateKey
   rather than a WIF-encoded string (#589)
 * DecryptAccount function in wallet package was moved to Decrypt method of
   Account (#589)
 * logging was moved from logrus to zap which changed messages format a little
   (#284, #598)
 * Block, BlockBase and Header structures were moved into their own `block`
   package (#597)
 * MemPool was moved into its own `mempool` package (#597)
 * Sign method of PrivateKey no longer returns an error (#604)
 * wallet is now compatible with C# node wallets (#603)
 * consensus service now gets its keys from the wallet file instead of
   plain-text WIF in the configuration (#588)
 * `image` Makefile target was updated to nspccdev repo (#625)
 * serializing invalid Transaction structures (that have no Data) is now
   forbidden (#640)
 * Item is no longer exposed by the mempool package (#647)

Improvements:
 * better testing of keys, wallet, compiler and mempool packages (#589, #568,
   #647)
 * smart use of previous proposal for block generation after dBFT's ChangeView
   (#591)
 * peer-server interaction refactored (#606)
 * peer message queues were introduced with different priorities reducing
   delays in consensus messages delivery and improving unicast communication
   (#590, #615, #639)
 * more efficient code generated by the compiler in some cases (#626, #629,
   #637)
 * mempool redesign leading to better performance (#647), it also doesn't
   store unverified transactions now

Bugs fixed:
 * consensus service was not really requesting transactions from peers when it
   was missing some locally (#602)
 * build failing with newer rfc6979 package (#592)
 * wrong integer serialization/deserialization in VM leading to
   inconsistencies in state with C# node (#501/#605)
 * consensus service was not processing node election results to
   enable/disable dBFT (#603)
 * in some cases peers were not unregistered after disconnect (#606)
 * different execution results with C# node because of missing invocation fee
   calculations (#607)
 * RPC server now properly returns the amount of gas spent during execution
   for invoke* calls (#609)
 * FromAddress interop was translated improperly by the compiler (#621)
 * compiler emitting wrong integers (#624)
 * EOF errors on recovery consensus messages decoding (#632)
 * wrong handling of assignments to underscore variable in the compiler (#631)
 * wrong answers to getunspents/getaccountstate RPC requests for unknown
   addresses (#633)
 * multiple dBFT issues leading to consensus stalls, attempts to add wrong
   block to the chain, message processing using wrong height and other
   problems (#635, #641)
 * duplicate connections were not detected before the handshake completion
   which can happen too late (#639)
 * attempts to connect to peer that has already connected to node (#639)
 * unregistration of connected addresses (#639)
 * ArrayReverse function was not copying arrays of size 1 (#644)
 * race in transaction verification/addition to the mempool leading to
   potential double spends (#647)
 * wrong concurrent attempts to add new block to the chain (#647)
 * blocks were not relayed properly by non-consensus nodes (#647)
 * mempool memory leak (#647)
 * missing reverification after block acceptance for transactions in mempool
   (#647)

## 0.71.0 "Celebration" (30 Dec 2019)

Version 0.71.0 ends the year 2019 of neo-go development with a solid release
that adds support for pprof, implements all missing interoperability
functions, improves performance and as usual fixes some bugs. This is also the
first release with a test coverage of more than 60% which is a significant
milestone in our trend to improve code quality and testing. Back in August
2019 when NSPCC had started neo-go developement the project only had a
coverage of 45% and even though a lot of code has been added since we're
improving this metrics with every release.

New features:
 * pprof support (disabled by default, #536)
 * compiler now supports assignments to an element of slice using variable
   index (#564)
 * hashed 4-byte IDs support for SYSCALL instruction  (#565)
 * remainder ('%') operator support in compiler (#563)
 * Neo.Witness.GetVerificationScript and Neo.InvocationTransaction.GetScript
   interop functions (#421)
 * Neo.Iterator.*, Neo.Enumerator.* and Neo.Storage.Find interops (#422)
 * tuple return support in compiler (#562)
 * `getblocks` P2P command implementation (#577)

Behaviour changes:
 * `Monitoring` config section changed its name to `Prometheus` because of
   pprof addition (#536)
 * `db dump` command now also writes the block number zero by default to
   comply with ImportBlocks format (#582)
 * `skip` parameter was renamed to `start` in the `db dump` command (#582)
 * `db restore` now skips the genesis block in the dump if starting to import
   into the new DB and if this block matches the one generated on blockchain
   init (#582)

Improvements:
 * VM optimizations (#558, #559, #565, #572)
 * top block caching in core to improve dApps performance (#572)
 * transaction's TXType can now be JSON-unmarshalled (#575)
 * `crypto` package no longer exports AES functions (#579)
 * merkle tree functions and structures were moved into the `hash` package
   (#579)
 * base58 functionality was moved into its own package from `crypto` (#579)
 * `Uint160DecodeAddress` and `AddressFromUint160` functions were moved out of
   `crypto` into their own package (#579)
 * `address` package now can be configured to use different prefixes (#579)

Bugs fixed:
 * wrong INVERT instruction behaviour for converted values (#561)
 * wrong DUP behaviour when the next instruction doesn't convert the value
   (#561)
 * OVER and PICK implementations didn't properly duplicate stack items (#561)
 * PICKITEM instruction failed to duplicate picked items (#561)
 * `getblock` RPC call was not answering with full hex-encoded block (#575)
 * base58.CheckDecode was decoding values with leading zeroes in a wrong way
   (#579)
 * decoding wrong public key could succeed (#579)
 * consensus payload verification was inconsistent with other verification
   code (#555)
 * DB dump was using wrong format in release 0.70.1 (#582)
 * DB restorer was not properly counting skipped blocks which could lead to
   bogus error being reported (#582)
 * the DB was not closed properly on error during DB restore (#582)
 * NPE in consensus (#583)

## 0.70.1 "Centrifugation" (16 Dec 2019)

Release 0.70.1 brings no new functionality focusing on internal node
improvements instead. Some bugs were fixed, refactoring was done and
performance was improved substantially.

Behaviour changes:
 * as with the previous release, this one changes the database format, so you
   need to resynchronize your nodes
 * code testing will fail if you're not to checkout submodules (we have one
   with neo-vm tests, #543)

Improvements:
 * "inv" command handling was changed to not ask peers of the items we already
   have (#533)
 * numerous optimizations across the whole system (#535, #546, #547, #545,
   #549, #550, #553, #554, #556, #557)
 * compiler no longer outputs errors by itself, passing them programmatically
   instead via its API (#457)
 * VM opcodes were moved into their own package allowing wider reuse (#538)
 * `compiler` package was moved one level up from `pkg/vm/compiler` to
   `pkg/compiler` (#538)
 * proper script trigger constants were added (#509)
 * consensus payloads are now properly signed and this signature is being
   checked (#544)
 * Uint160/Uint256 types were made more consistent and having an explicit
   LE/BE suffix for conversion functions (#314)
 * a separate layer based on storage.Store with state item-specific functions
   was created (#517, #334)
 * ReadBytes/WriteBytes in io package were renamed into
   ReadVarBytes/WriteVarBytes to more accurately describe them and be more
   consistent with C# implementation (#545)
 * io package was refactored to add type-specific read/write methods to its
   structures and remove generic (slow) ReadLE/WriteLE (#553, #554)
 * a simple tx performance benchmark was added (#551)

Bugs fixed:
 * consensus could start and stall with not-yet-connected peers (#532)
 * nil pointer dereference in PrepareResponse and PrepareRequest messages
   handling (#532)
 * fatal error because of locking problems in mempool (#535)
 * missing error handling in compiler emitter functions (#297)
 * wallet opening command now hides the password printed into it (#539)
 * storage was updated improperly when several transactions in one block were
   touching the same keys, leading to subsequent transactions failures (#542)
 * application execution results were not saved in full (#517)
 * mempool transaction verification was wrong (#557)

## 0.70.0 "Constellation" (29 Nov 2019)

This is a long-awaited and exciting release implementing a full Neo
consensus node that you can run your own neo-go private net with! It also
brings with it serious improvements in contract handling, you can now not
only compile, but also deploy and invoke contracts with neo-go.

New features:
 * systemd unit file for deployment (#326)
 * claim transactions processing was added (#489)
 * Consensus payloads decoding/encoding (#431)
 * `getunspents` method is RPC server (#473)
 * updated docker-compose environment for privnet setup (#497, #529)
 * client-side `getunspents` RPC method support (#511)
 * contract deployment from the CLI (#474)
 * enrollment and state transactions processing (#508)
 * `Neo.Blockchain.GetValidators` interop support (#420)
 * `invokefunction` RPC method support in the server (#347)
 * `testinvokefunction` command in the CLI to do test invocations via
   `invokefunction` RPC method (#521)
 * consensus node support (#507, #525)
 * server-side `invoke` RPC method support (#346)
 * `testinvoke` CLI command to invoke contracts via `invoke` RPC method (#527)
 * `getheaders` P2P message processing (#529)
 * relaying was added for transactions coming from P2P network (#529)
 * `invoke` and `invokefunction` commands to invoke deployed script and send an
   invocation transaction to the network (#531)

Behavior changes:
 * db dump/restore format is now compatible with NGD chain dumps (#466)
 * smart contracts now have a new configuration format that is used to deploy
   them (#511)
 * `testinvoke` CLI command was renamed to `testinvokescript` (#521)

Improvements:
 * `core.Blockchainer` interface now has a `Close` method (#485)
 * `util.Uint256Size` is now public (#490)
 * `io` package now has generic functions for array
   serialization/deserialization (#490)
 * `util.Uint256` now supports `io.Serializable` interface (#495)
 * `smartcontract.ParamType` type now supports `io.Serializable` interface
   (#495)
 * `vm.ByteArrayItem` now uses hex representation when being marshalled into
   JSON (#499)
 * `io` serialization/deserialization for arrays is now restricted in elements
   count (#503, #505)
 * `core.AccountState` now stores all UTXOs for the account (#504)
 * interop functions got some testing coverage (#492)
 * rpc client now implements `CalculateInputs` method via `getunspents` call
   for transaction building (#511)
 * `transaction.NewInvocationTX` now accepts a gas parameter for the
   corresponding transaction field (#511)
 * `rpc.StackParamType` now supports YAML marshaling/unmarshaling
 * `rpc` package now has more fine-grained methods for transaction building
   (#511, #531)
 * `Blockchain` now stores and updates validators list (#508)
 * blockchain state management refactored (#508)
 * `rpc` invocation parameter management reworked (#513)
 * `util` test coverage improved (#515)
 * `invokescript` tests were added to the `rpc` package (#521)
 * `crypto/keys` and `crypto/hash` packages test coverage improved (#516)

Bugs fixed:
 * blockchain not persisting the latest changes on exit (#485)
 * db dump/restore commands incorrectly handled `skip` parameter (#486)
 * vm failed to serialize duplicating non-reference elements (#496)
 * improper smartcontract notifications handling (#453)
 * nondeterministic `GetReferences` interop behaviour leading to contract
   failures (#454)
 * writing message to the peer could be interleaved with other messages
   leading to garbage being sent (#503, #506)
 * inability to process block with previously relayed transaction (#511)
 * decoding transaction with invalid type didn't return an error (#522)
 * attempts to reconnect to the node with the same ID (#507)
 * peer disconnects during handshake because of code race (#529)
 * useless header requests from peers with low height (#529)
 * wrong header hashes initialization from the DB in case there are 2000*N + 1
   blocks in the chain (#529)

## 0.62.0 "Commotion" (07 Nov 2019)

Release 0.62.0 finishes one very important work some pieces of which were
gradually rolled out in previous releases --- it integrates all neo-vm project
JSON-based tests for NEO 2.0 C# VM and runs them successfully against neo-go
VM. There are also important bug fixes based on mainnet nodes deployment
experience and additional configuration options.

New Features:
 * implemented `Runtime.Serialize` and `Runtime.Deserialize` syscalls (#419)
 * new configuration option -- `AttemptConnPeers` to set the number of
   connections that the node will try to establish when it goes below the
   MinPeers setting (#478)
 * `LogPath` configuration parameter to write logs into some file and not to
   stdout (#460), not enabled by default
 * `Address` configuration parameter to specify the address to bind to (#460),
   not enabled by default

Behavior changes:
 * mainnet configuration now has correct ports specified (#478)
 * multiple connections to the same peer are disallowed now (as they are in C#
   node (#478))
 * the default MaxPeers setting was increased to 100 for mainnet and testnet
   configurations and limited to 10 for privnet (#478)

Improvements:
 * implemented missing VM constraints: stack item number limitation (#462) and
   integer size checks (#484, #373)
 * added a framework to run JSON-based neo-vm tests for C# VM and fixed all
   remaining incompabitibilities (#196)
 * added wallet unit tests (#475)
 * network.Peer's NetAddr method was split into RemoteAddr and PeerAddr (#478)
 * `MakeDirForFile` function was added to the `io` package (#470)

Bugs fixed:
 * RPC service responded with block height to `getblockcount` request which
   differs from C# interpretation of `getblockcount` (#471)
 * `getbestblockhash` RPC method response was not adding leading `0x` prefix
   to the hash, while C# node does it
 * inability to correctly handshake clients on the server side (#458, #480)
 * data race in `Server` structure fields access (#478)
 * MaxPeers configuration setting was not working properly (#478)
 * useless DB reads (that failed in some cases) on persist attempt that didn't
   persist anything (#481)
 * current header height was not stored in the DB when starting a new
   blockchain which lead to node failures on restart (#481)
 * crash on node restart if no header hashes were written into the DB (#481)

## 0.61.0 "Cuspidation" (01 Nov 2019)

New features:
 * Prometheus support for monitoring (#441)
 * `neo-go contract invoke` now accepts endpoint parameter (`--endpoint` or
   `-e`) to specify RPC node to be used for invocation (#363)
 * RPC server now supports `invokescript` method (#348)
 * minimum peers number can now be configured (#468)
 * configured CORS workaround implemented in the RPC package (#469)

Behavior changes:
 * `neo-go contract inspect` now expects avm files in input, but can also
   compile Go code with `-c` parameter (previously is was done by default),
   `inspect` subcommand was removed from `neo-go vm` (it dumped avm files in
   previous release) (#463)
 * the default minimum peers was reduced to 3 for privnet setups to avoid
   useless reconnections to only 4 available nodes
 * RPC service now has its own section in configuration, update your
   configurations (#469)

Improvements:
 * VM.Load() now clears the state properly, making VM reusable after the Run()
   (#463)
 * Compile() in compiler package no longer accepts Options, they were not used
   previously anyway (#463)
 * invocation stack depth is now limited in the VM (#461)
 * VM got new State() method to get textual state description (#463)
 * vm's Stack structure can now be marshalled into JSON (#463)

Bugs fixed:
 * race in discoverer part of the server (#445)
 * RPC server giving improper (not JSON) respons to unimplemented API requests
   (#463)

## 0.60.0 "Cribration" (25 Oct 2019)

Release 0.60.0 brings with it an implementation of all NEO 2.0 VM opcodes,
full support for transaction relaying, improved logging, a bunch of fixes and
an updated project logo.

New features:
 * blocks dumping from DB to file and restoring from file to DB (#436)
 * new logo (#444)
 * implemented `getdata` message handling (#448)
 * issue tx processing (#450)
 * CALL_I, CALL_E, CALL_ET, CALL_ED, CALL_EDT implementation in the VM (#192)

Internal improvements:
 * codestyle fixes (#439, #443)
 * removed spurious prints from all the code, now everything is passed/logged
   correctly (#247)

Bugs fixed:
 * missing max size limitation in CAT and PUSHDATA4 opcodes implementation
   (#435)
 * wrong interpretation of missing unspent coin state when checking for double
   spend (#439)
 * panic on successive node starts when no headers were saved in the DB (#440)
 * NEWARRAY/NEWSTRUCT opcodes didn't copy operands for array<->struct
   conversions
 * deadlock in MemPool on addition (#448)
 * transactions were not removed from the MemPool when processing new signed
   block (#446)
 * wrong contract property constants leading to storage usage failures (#450)

## 0.51.0 "Confirmation" (17 Oct 2019)

With over a 100 commits made since 0.50.0 release 0.51.0 brings with it full
block verification, improved and fixed transaction verifications,
implementation of most of interop functions and VM improvements. Block
verification is an important milestone on the road to full consensus node
support and it required implementing a lot of other associated functionality.

New features:
 * CHECKSIG, VERIFY and CHECKMULTISIG instructions in VM (#269)
 * witness verification logic for transactions (#368)
 * NEWMAP, HASKEY, KEYS and VALUES instructions, support for Map type in
   PICKITEM, SETITEM, REMOVE, EQUAL, ARRAYSIZE (#359)
 * configurable transaction verification on block addition (#415, #418)
 * contract storage and support for VM to call contracts via APPCALL/TAILCALL
   (#417)
 * support for Interop type in VM (#417)
 * VM now has stepInto/stepOver/stepOut method implementations for debugging
   matching neo-vm behavior (#187)
 * storage support for contracts (#418)
 * added around 90% of interop functions (#418)
 * Invocation TX processing now really does invoke contracts using internal VM
   (#418)
 * blocks are now completely verified when added to the chain (if not
   configured otherwise; #12, #418)

Behavior changes:
 * full block verification is now enabled for all network types
 * block's transaction verification enabled for privnet setups, mainnet and
   testnet don't have it enabled

Technical improvements:
 * GetVarIntSize and GetVarStringSize were removed from the io package (use
   GetVarSize instead; #408)
 * OVER implementation was optimized to not pop the top element from the stack
   (#406, part of #196 work)
 * vm.VM was extended with HasFailed() method to check its state (previously
   external VM users couldn't do it; #411)
 * redesigned input block queue mechanism, now it's completely moved out of
   the Blockchain, which only accepts the next block via AddBlock() (#414)
 * unpersisted blocks are now fully available with the Blockchain (thus we
   have symmetry now in AddBlock/GetBlock APIs; #414, #366)
 * removed duplicating batch structures from BoltDB and Redis code, now all of
   them use the same batch as MemoryStore does (#414)
 * MemoryStore was exporting its mutex for no good reason, now it's hidden
   (#414)
 * storage layer now returns ErrKeyNotFound for all DBs in appropriate
   situations (#414)
 * VM's PopResult() now doesn't panic if there is no result (#417)
 * VM's Element now has a Value() method to quickly get the item value (#417)
 * VM's stack PushVal() method now accepts uint8/16/32/64 (#417, #418)
 * VM's Element now has TryBool() method similar to Bool(), but without a
   panic (for external VM users; #417)
 * VM has now completely separated instruction read and execution phases
   (#417)
 * Store interface now has Delete method (#418)
 * Store tests were reimplemented to use one test set for all Store
   implementations, including LevelDB that was not tested at all previously
   (#418)
 * Batch interface doesn't have Len method now as it's not used at all (#418)
 * New*FromRawBytes functions were renamed to New*FromASN1 in the keys
   package, previous naming made it easy to confuse them with functions
   operating with NEO serialization format (#418)
 * PublicKey's IsInfinity method is exported now (#418)
 * smartcontract package now has CreateSignatureRedeemScript() matching C#
   code (#418)
 * vm package now has helper functions
   IsSignatureContract/IsMultiSigContract/IsStandardContract matching C# code
   (#418)
 * Blockchain's GetBlock() now returns full block with transactions (#418)
 * Block's Verify() was changed to return specific error (#418)
 * Blockchain's GetTransationResults was renamed into GetTransactionResults
   (#418)
 * Blockchainer interface was extended with GetUnspentCoinState,
   GetContractState and GetScriptHashesForVerifying methods (#418)
 * introduced generic MemCacheStore that is used now for write caching
   (including temporary stores for transaction processing) and batched
   persistence (#425)

Bugs fixed:
 * useless persistence failure message printed with no error (#409)
 * persistence error message being printed twice (#409)
 * segmentation fault upon receival of message that is not currently handled
   properly (like "consensus" message; #409)
 * BoltDB's Put for a batch wasn't copying data which could lead to data
   corruption (#409)
 * APPEND instruction applied to struct element was not copying it like neo-vm
   does (#405, part of #196 work)
 * EQUAL instruction was comparing array contents, while it should've compared
   references (#405, part of #196 work)
 * SUBSTR instruction was failing for out of bounds length parameters while it
   should've truncated them to string length (#406, part of #196 work)
 * SHL and SHR implementations had no limits, neo-vm restricts them to
   -256/+256 (#406, part of #196 work)
 * minor VM state mismatches with neo-vm on failures (#405, #406)
 * deadlock on Blockchain init when headers pointer is not in sync with the
   hashes list (#414)
 * node failed to request blocks when headers list was exactly one position
   ahead of block count (#414)
 * TestRPC/getassetstate_positive failed occasionally (#410)
 * panic on block verification with no transactions inside (#415)
 * DutyFlag check in GetScriptHashesForVerifying was not done correctly (#415)
 * default asset expiration for assets created with Register TX was wrong, now
   it matches C# code (#415)
 * Claim transactions needed specific GetScriptHashesForVerifying logic to
   be verified correctly (#415)
 * VerifyWitnesses wasn't properly sorting hashes and witnesses (#415)
 * transactions referring to two outputs of some other transaction were
   failing to verify (#415)
 * wrong program dumps (#295)
 * potential data race in logging code (#418)
 * bogus port check during handshake (#432)
 * missing max size checks in NEWARRAY, NEWSTRUCT, APPEND, PACK, SETITEM
   (#427, part of #373)

## 0.50.0 "Consolidation" (19 Sep 2019)

The first release from the new team focuses on bringing all related
development effort into one codebase, refactoring things, fixing some
long-standing bugs and adding new functionality. This release merges two
radically different development branches --- `dev` and `master` that were
present in the project (along with all associated pull requests) and also
brings in changes made to the compiler in the neo-storm project.

New features:
 * configurable storage backends supporting LevelDB, in-memory DB (for
   testing) and Redis
 * BoltDB support for storage backend
 * updated and extended interop APIs (thanks to neo-storm)

Notable behavior changes:
 * the default configuration for privnet was changed to use ports 20331 and
   20332 so that it doesn't clash with the default dockerized neo-privnet
   setups
 * the default configuration path was changed from `../config` to `./config`,
   at this stage it makes life a bit easier for development, later this will
   be changed to some sane default for production version
 * VM CLI now supports type inference for `run` parameters (you don't need to
   specify types explicitly in most of the cases) and treats `operation`
   parameter as mandatory if anything is passed to `run`
 
VM improvements:
 * added implementation for `EQUAL`, `NZ`, `PICK`, `TUCK`, `XDROP`, `INVERT`,
   `CAT`, `SUBSTR`, `LEFT`, `RIGHT`, `UNPACK`, `REVERSE`, `REMOVE`
 * expanded tests
 * better error messages for different erroneous code
 * implemented item conversions following neo-vm behavior: array to/from
   struct, bigint to/from boolean, anything to bytearray and anything to
   boolean
 * improved compatibility with neo-vm (#394)

Technical improvements:
 * switched to Go 1.12+
 * gofmt, golint (#213)
 * fixed and improved CircleCI builds
 * removed internal rfc6969 package (#285)
 * refactored util/crypto/io packages, removed a lot of duplicating code
 * updated READMEs and user-level documents
 * update Makefile with useful targets
 * dropped internal base58 implementation (#355)
 * updated default seed lists for mainnet and testnet from neo-cli

Bugs fixed:
 * a lot of compiler fixes from neo-storm
 * data access race in memory-backed storage backend (#313)
 * wrong comparison opcode emitted by compiler (#294)
 * decoding error in `publish` transactions (#179)
 * decoding error in `invocation` transactions (#173)
 * panic in `state` transaction decoding
 * double VM run from CLI (#96)
 * non-constant time crypto (#245)
 * APPEND pushed value on the stack and worked for bytearrays (#391)
 * reading overlapping hash blocks from the DB leading to blockchain state
   neo-go couldn't recover from (#393)
 * codegen for `append()` wasn't type-aware and emitted wrong code (#395)
 * node wasn't trying to reconnect to other node if connection failed (#390)
 * stricly follow handshare procedure (#396)
 * leaked connections if disconnect happened before handshake completed (#396)

### Inherited unreleased changes

Some changes were also done before transition to the new team, highlights are:
 * improved RPC testing
 * implemented `getaccountstate`, `validateaddress`, `getrawtransaction` and
   `sendrawtransaction` RPC methods in server
 * fixed `getaccountstate` RPC implementation
 * implemented graceful blockchain shutdown with proper DB closing

## 0.45.14 (not really released, 05 Dec 2018)

This one can technically be found in the git history and attributed to commit
fa1da2cb917cf4dfccbe49d44c5741eec0e0bb65, but it has no tag in the repository
and so can't count as a properly released thing. Still it can be marked as a
point in history with the following changes relative to 0.44.10:
 * switched to Go modules for dependency management
 * various optimizations for basic structures like Uin160/Uint256/Fixed8
 * improved testing
 * added support for `invoke` method in RPC client
 * implemented `getassetstate` in RPC server
 * fixed NEP2Encrypt and added tests
 * added `NewPrivateKeyFromRawBytes` function to the `wallet` package

## 0.44.10 (27 Sep 2018)

This is the last one tagged in the repository, so it's considered as the last
one properly released before 0.50+. Releases up to 0.44.10 seem to be made in
automated fashion and you can find their [changes on
GitHub](https://github.com/nspcc-dev/neo-go/releases).
